package filetree

import (
	"context"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

type treeColumn = int

const (
	columnIcon treeColumn = iota
	columnName
	columnPath
	// TODO: add column for system attribute marks (e.g. errors, loading, etc.)
)

var allTreeColumns = []treeColumn{
	columnIcon,
	columnName,
	columnPath,
}

/*
func updateFolderIter(ctx context.Context, store *gtk.TreeStore, iter *gtk.TreeIter, path string) {
	gtkutil.Async(ctx, func() func() {
		s, err := os.Stat(path)
		if err != nil {
			return func() {
				store.Set(iter, allTreeColumns, filePseudoError(err))
			}
		}

		return func() {
			values := fileColumnValues(path, s)
			store.Set(iter, allTreeColumns, values)
		}
	})
	go func() {
	}()
}
*/

func filePseudoError(err error) []glib.Value {
	return []glib.Value{
		*glib.NewValue("dialog-error-symbolic"),
		*glib.NewValue(err.Error()),
		*glib.NewValue(""),
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
		*glib.NewValue(filepath.Base(path)),
		*glib.NewValue(path),
	}
}

type treePath struct {
	*gtk.TreePath
	dir    *treeDir
	expand bool
}

// treeEntry describes one entry (row) matching up to a file.
type treeEntry interface {
	IsDir() bool
	Remove() bool
	TreeIter() (*gtk.TreeIter, bool)
}

type treeFile struct {
	store    *gtk.TreeStore
	treePath *gtk.TreePath
	filePath string
}

func newTreeFile(store *gtk.TreeStore, root *gtk.TreeIter, path string) *treeFile {
	return &treeFile{
		store:    store,
		treePath: store.Path(root),
		filePath: path,
	}
}

// IsDir returns false.
func (f *treeFile) IsDir() bool { return false }

// TreeIter returns f's TreeIter.
func (f *treeFile) TreeIter() (*gtk.TreeIter, bool) {
	return f.store.Iter(f.treePath)
}

// Remove removes f from the store.
func (f *treeFile) Remove() bool {
	if iter, ok := f.TreeIter(); ok {
		f.store.Remove(iter)
		return true
	}
	return false
}

type treeDir struct {
	treeFile
	child map[string]treeEntry // name -> treeEntry

	// temp is nilable.
	temp *gtk.TreeIter

	expand     bool
	refreshing bool
}

func newTreeDir(store *gtk.TreeStore, root *gtk.TreeIter, path string) *treeDir {
	dir := treeDir{treeFile: *newTreeFile(store, root, path)}
	dir.temp = store.Append(root)
	return &dir
}

func (d *treeDir) IsDir() bool { return true }

func (d *treeDir) Clear() {
	for filePath, treeEntry := range d.child {
		treeEntry.Remove()
		delete(d.child, filePath)
	}
}

// Init initializes the directory asynchronously if it hasn't been already.
func (d *treeDir) Init(ctx context.Context) {
	if !d.refreshing && d.child == nil {
		var wg sync.WaitGroup
		d.Refresh(ctx, &wg)
	}
}

// Refresh refresshes the directory. If the directory contains expanded children
// directories, then it's refreshed as well.
func (d *treeDir) Refresh(ctx context.Context, wg *sync.WaitGroup) {
	if d.refreshing {
		return
	}
	d.refreshing = true

	if d.temp != nil {
		d.store.Remove(d.temp)
		d.temp = nil
	}

	// Quick assert before going asynchronous.
	if _, ok := d.TreeIter(); !ok {
		return
	}

	wg.Add(1)
	finalize := func() {
		wg.Done()
		d.refreshing = false
	}

	gtkutil.Async(ctx, func() func() {
		time.Sleep(5 * time.Second)
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

			child := make(map[string]treeEntry, len(files))

			for _, file := range files {
				entry, ok := d.child[file.Name()]
				if ok && entry.IsDir() == file.IsDir() {
					// Check if the entry is a directory. If yes, expand it.
					if d, ok := entry.(*treeDir); ok && d.expand {
						d.Refresh(ctx, wg)
					}
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
	treeDir
	entries map[string]treeEntry // TreePath -> treeEntry
}

func newTreeRoot(path string) treeRoot {
	store := gtk.NewTreeStore([]glib.Type{
		glib.TypeString,
		glib.TypeString,
		glib.TypeString,
	})

	root := store.Append(nil)
	store.Set(root, allTreeColumns, []glib.Value{
		*glib.NewValue("folder-symbolic"),
		*glib.NewValue(filepath.Base(path)),
		*glib.NewValue(path),
	})

	return treeRoot{
		treeDir: *newTreeDir(store, root, path),
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

// Refresh refreshes the whole tree root.
func (r *treeRoot) Refresh(ctx context.Context, done func()) {
	var wg sync.WaitGroup
	r.treeDir.Refresh(ctx, &wg)

	gtkutil.Async(ctx, func() func() {
		wg.Wait()
		return done
	})
}

// entry gets the treeEntry value from the given path. If the path is not known,
// then nil is returned.
//
// If the given path is absolute, then the root path is used to resolve the
// relative path. If the root path and the absolute base does not match, then an
// error is assumed and nil is returned.
func (r *treeRoot) entry(path string) treeEntry {
	now := time.Now()
	defer func() { log.Println("entry() took", time.Since(now)) }()

	if filepath.IsAbs(path) {
		p, err := filepath.Rel(r.filePath, path)
		if err != nil {
			return nil
		}
		path = p
	}

	parts := filepath.SplitList(path)

	dir := &r.treeDir

	for i, part := range parts {
		entry, ok := dir.child[part]
		if !ok {
			return nil
		}

		if i == len(parts)-1 {
			// The path points to this entry.
			return entry
		}

		switch entry := entry.(type) {
		case *treeFile:
			// The path doesn't point to a file, but we ended up with a file
			// anyway. We can't traverse further.
			return nil
		case *treeDir:
			// Keep traversing.
			dir = entry
		}
	}

	return nil
}

func (r *treeRoot) MarkExpanded(ctx context.Context, iter *gtk.TreeIter, expanded bool) {
	// TODO: figure out a more optimized way. We can keep track of gtk.TreePath
	// strings to all entries.
	pathValue := r.store.Value(iter, columnPath)
	path := pathValue.String()

	if dir, ok := r.entry(path).(*treeDir); ok {
		dir.expand = expanded
		if expanded {
			dir.Init(ctx)
		}
	}
}
