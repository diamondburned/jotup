package filetree

import (
	"context"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

type treeColumn = int

const (
	columnIcon treeColumn = iota
	columnName
	columnPath
	columnUnsaved
	columnSensitive
	// TODO: add column for system attribute marks (e.g. errors, loading, etc.)
)

var allTreeColumns = []treeColumn{
	columnIcon,
	columnName,
	columnPath,
	columnUnsaved,
	columnSensitive,
}

var columnTypes = []glib.Type{
	glib.TypeString,
	glib.TypeString,
	glib.TypeString,
	glib.TypeString,
	glib.TypeBoolean,
}

func filePseudoError(err error) []glib.Value {
	return []glib.Value{
		*glib.NewValue("dialog-error-symbolic"),
		*glib.NewValue(fmt.Sprintf(
			`<span color="red"><b>Error:</b></span> %s`,
			html.EscapeString(err.Error()),
		)),
		*glib.NewValue(""),
		*glib.NewValue(""),
		*glib.NewValue(false),
	}
}

func fileColumnValues(path string, file fs.DirEntry) []glib.Value {
	var icon string

	switch {
	case file == nil:
		icon = "dialog-error-symbolic"
	case file.IsDir():
		icon = "folder-symbolic"
	case file.Type().Perm()&0111 != 0:
		icon = "application-x-appliance-symbolic"
	default:
		icon = iconExt(filepath.Ext(file.Name()))
	}

	return []glib.Value{
		*glib.NewValue(icon),
		*glib.NewValue(html.EscapeString(filepath.Base(path))),
		*glib.NewValue(path),
		*glib.NewValue(""),
		*glib.NewValue(true),
	}
}

type treePath struct {
	*gtk.TreePath
	dir    *TreeDir
	expand bool
}

// TreeEntry describes one entry (row) matching up to a file.
type TreeEntry interface {
	IsDir() bool
	Remove() bool
	FilePath() string
	TreePath() *gtk.TreePath
	TreeIter() (*gtk.TreeIter, bool)
}

type TreeFile struct {
	store    *gtk.TreeStore
	treePath *gtk.TreePath
	filePath string
}

func newTreeFile(store *gtk.TreeStore, root *gtk.TreeIter, path string) *TreeFile {
	return &TreeFile{
		store:    store,
		treePath: store.Path(root),
		filePath: path,
	}
}

// IsDir returns false.
func (f *TreeFile) IsDir() bool { return false }

// SetSensitive sets the sensitive property of all cells.
func (f *TreeFile) SetSensitive(sensitive bool) {
	i, ok := f.TreeIter()
	if ok {
		f.store.SetValue(i, columnSensitive, glib.NewValue(sensitive))
	}
}

// FilePath returns the file's OS path.
func (f *TreeFile) FilePath() string {
	return f.filePath
}

// TreePath returns the file's TreePath.
func (f *TreeFile) TreePath() *gtk.TreePath {
	return f.treePath
}

// TreeIter returns f's TreeIter.
func (f *TreeFile) TreeIter() (*gtk.TreeIter, bool) {
	return f.store.Iter(f.treePath)
}

// Remove removes f from the store.
func (f *TreeFile) Remove() bool {
	if iter, ok := f.TreeIter(); ok {
		f.store.Remove(iter)
		return true
	}
	return false
}

// UnsavedDot is the dot used to mark a file as unsaved.
const UnsavedDot = "●"

// SetUnsaved sets whether the file shows the unsaved dot.
func (f *TreeFile) SetUnsaved(unsaved bool) {
	if iter, ok := f.TreeIter(); ok {
		if unsaved {
			f.store.SetValue(iter, columnUnsaved, glib.NewValue(UnsavedDot))
		} else {
			f.store.SetValue(iter, columnUnsaved, glib.NewValue(""))
		}
	}
}

type TreeDir struct {
	TreeFile
	child map[string]TreeEntry // name -> TreeEntry

	// temp is nilable.
	temp *gtk.TreeIter
	// load is nilable.
	load chan struct{}

	unsaved int
}

func newTreeDir(store *gtk.TreeStore, root *gtk.TreeIter, path string) *TreeDir {
	dir := TreeDir{
		TreeFile: *newTreeFile(store, root, path),
		temp:     store.Append(root),
	}
	return &dir
}

func (d *TreeDir) IsDir() bool { return true }

func (d *TreeDir) Clear() {
	for filePath, TreeEntry := range d.child {
		TreeEntry.Remove()
		delete(d.child, filePath)
	}
}

// IsInitialized returns true if TreeDir is being initialized or
func (d *TreeDir) IsInitialized() bool {
	return d.load == nil && d.child != nil
}

// Init initializes the directory asynchronously if it hasn't been already.
func (d *TreeDir) Init(ctx context.Context, done func()) {
	if !d.IsInitialized() {
		d.Refresh(ctx, done)
		return
	}
	done()
}

// Refresh refresshes the directory. If the directory contains expanded children
// directories, then it's refreshed as well. If the TreeDir is already
// reloading, then done will be queued up and called once that loading is done.
func (d *TreeDir) Refresh(ctx context.Context, done func()) {
	d.refresh(ctx, done)
}

func (d *TreeDir) refresh(ctx context.Context, done func()) {
	if d.load != nil {
		// Make a copy of the load channel for us. We don't want to read the
		// field in a goroutine.
		go func(load chan struct{}) {
			select {
			case <-ctx.Done():
				return
			case <-load:
				glib.IdleAdd(done)
			}
		}(d.load)
		return
	}

	// We're not currently loading. Create a load channel so goroutines can wait
	// for the load.
	load := make(chan struct{})
	d.load = load

	d.refreshOnce(ctx, func() {
		// Close our load channel to unblock all waiting goroutines.
		close(load)

		// If TreeDir still has our load channel, then we nil it. Otherwise,
		// leave it be because it's not ours.
		if d.load == load {
			d.load = nil
		}

		done()
	})
}

func (d *TreeDir) refreshOnce(ctx context.Context, done func()) {
	// Quick assert before going asynchronous.
	if _, ok := d.TreeIter(); !ok {
		done()
		return
	}

	d.SetSensitive(false)

	if d.temp != nil {
		d.store.Remove(d.temp)
		d.temp = nil
	}

	finalize := func() {
		d.SetSensitive(true)
		done()
	}

	gtkutil.Async(ctx, func() func() {
		files, err := os.ReadDir(d.filePath)

		return func() {
			defer finalize()

			if err != nil {
				root, ok := d.TreeIter()
				if !ok {
					return
				}

				d.Clear()

				d.temp = d.store.Append(root)
				d.store.Set(d.temp, allTreeColumns, filePseudoError(err))
				return
			}

			root, ok := d.TreeIter()
			if !ok {
				return
			}

			sort.SliceStable(files, func(i, j int) bool {
				idir := files[i].IsDir()
				jdir := files[j].IsDir()
				if idir == jdir {
					return false
				}
				return idir || !jdir
			})

			child := make(map[string]TreeEntry, len(files))

			for _, file := range files {
				entry, ok := d.child[file.Name()]
				if ok && entry.IsDir() == file.IsDir() {
					child[file.Name()] = entry
					continue
				}

				var it *gtk.TreeIter
				// See if we can grab the entry's iterator directly.
				if entry != nil {
					if iter, ok := entry.TreeIter(); ok {
						it = iter
					}
				}
				if it == nil {
					// No, so just append normally.
					it = d.store.Append(root)
				}

				path := filepath.Join(d.filePath, file.Name())

				if file.IsDir() {
					entry = newTreeDir(d.store, it, path)
				} else {
					entry = newTreeFile(d.store, it, path)
				}

				d.store.Set(it, allTreeColumns, fileColumnValues(path, file))
				child[file.Name()] = entry
			}

			d.child = child
		}
	})
}

type treeRoot struct {
	TreeDir
	entries map[string]TreeEntry // TreePath -> TreeEntry
}

func newTreeRoot(path string) treeRoot {
	store := gtk.NewTreeStore(columnTypes)

	root := store.Append(nil)
	store.Set(root, allTreeColumns, []glib.Value{
		*glib.NewValue("folder-symbolic"),
		*glib.NewValue(html.EscapeString(filepath.Base(path))),
		*glib.NewValue(path),
		*glib.NewValue(""),
		*glib.NewValue(true),
	})

	return treeRoot{
		TreeDir: *newTreeDir(store, root, path),
	}
}

// Model returns the root's model.
func (r *treeRoot) Model() gtk.TreeModeller {
	return r.store
}

// RootPath returns the root TreePath.
func (r *treeRoot) RootPath() *gtk.TreePath {
	return r.treePath
}

// RootIter returns the root node's TreeIter.
func (r *treeRoot) RootIter() *gtk.TreeIter {
	iter, ok := r.store.Iter(r.treePath)
	if !ok {
		panic("BUG: RootIter cannot find root node")
	}
	return iter
}

// IterPath returns the file path from the given TreeIter.
func (r *treeRoot) IterPath(iter *gtk.TreeIter) string {
	// TODO: figure out a more optimized way. We can keep track of gtk.TreePath
	// strings to all entries.
	pathValue := r.store.Value(iter, columnPath)
	path := pathValue.String()
	return path
}

// EntryFromTreePath returns the TreeEntry from the TreePath.
func (r *treeRoot) EntryFromTreePath(path *gtk.TreePath) TreeEntry {
	it, ok := r.store.Iter(path)
	if !ok {
		return nil
	}

	return r.Entry(r.IterPath(it))
}

// Entry gets the TreeEntry value from the given path. If the path is not
// known, then nil is returned.
//
// If the given path is absolute, then the root path is used to resolve the
// relative path. If the root path and the absolute base does not match, then an
// error is assumed and nil is returned.
func (r *treeRoot) Entry(path string) TreeEntry {
	var entry TreeEntry
	r.WalkEntry(path, func(e TreeEntry) bool {
		entry = e
		return true
	})
	return entry
}

// WalkEntry is like Entry, except the function f is called on every directory
// descended until the requested file is found. If any of the paths cannot be
// found, then f(nil) is called and the function returns. If f returns false
// during the walk, then the function also returns.
func (r *treeRoot) WalkEntry(path string, f func(TreeEntry) bool) {
	r.ResolveEntry(nil, path, f)
}

// ResolveEntry is like WalkEntry, except if any of the tree directories are
// uninitialized, it'll do that asynchronously, and f will be called at a later
// point once that happens.
func (r *treeRoot) ResolveEntry(ctx context.Context, path string, f func(TreeEntry) bool) {
	// This whole process appears to take from 3us to 160us with maximum 3
	// levels deep. That's pretty good.

	// now := time.Now()
	// defer func() { log.Println("entry() took", time.Since(now)) }()

	if filepath.IsAbs(path) {
		p, err := filepath.Rel(r.filePath, path)
		if err != nil {
			f(nil)
			return
		}
		path = p
	}

	parts := strings.Split(path, string(filepath.Separator))
	r.walkEntry(ctx, &r.TreeDir, parts, f)
}

func (r *treeRoot) walkEntry(ctx context.Context, dir *TreeDir, parts []string, f func(TreeEntry) bool) {
	for i, part := range parts {
		if !dir.IsInitialized() {
			if ctx != nil {
				// If the directory isn't initialized yet, then we initialize it
				// and walk the same directory again once it has been.
				dir.Init(ctx, func() { r.walkEntry(ctx, dir, parts[i:], f) })
			}
			return
		}

		entry, ok := dir.child[part]
		if !ok {
			f(nil)
			return
		}

		if i == len(parts)-1 {
			// The path points to this entry.
			f(entry)
			return
		}

		switch entry := entry.(type) {
		case *TreeFile:
			// The path doesn't point to a file, but we ended up with a file
			// anyway. We can't traverse further.
			f(nil)
			return
		case *TreeDir:
			// Keep traversing.
			dir = entry
			if !f(entry) {
				return
			}
		}
	}
}
