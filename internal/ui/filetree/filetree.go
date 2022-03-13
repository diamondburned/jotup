package filetree

import (
	"context"
	"path/filepath"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// Tree is a file tree.
type Tree struct {
	*gtk.Box
	Scroll struct {
		*gtk.ScrolledWindow
		View *gtk.TreeView
	}
	Actions *gtk.ActionBar

	spinbox *gtk.Revealer
	spinner *gtk.Spinner

	ctx  context.Context
	root treeRoot
}

var loadingCSS = cssutil.Applier("filetree-loading", `
	actionbar spinner.filetree-loading {
		margin: 0 6px;
	}
`)

var boxCSS = cssutil.Applier("filetree-box", `
	.filetree-box scrolledwindow undershoot.top,
	.filetree-box scrolledwindow undershoot.bottom {
		background-size:   100% 1px;
		background-repeat: no-repeat;
		background-image:  repeating-linear-gradient(90deg, @borders, @borders 5px, transparent 5px, transparent 10px);
		min-height: 0;
	}
	.filetree-box scrolledwindow undershoot.bottom {
		background-position: 0 100%;
	}
	.filetree-box actionbar > revealer > box {
		border-top: none;
	}
`)

// NewTree creates a new Tree.
func NewTree(ctx context.Context) *Tree {
	t := Tree{ctx: ctx}

	t.Scroll.View = gtk.NewTreeView()
	t.Scroll.View.AddCSSClass("filetree-view")
	t.Scroll.View.SetVExpand(true)
	t.Scroll.View.SetHExpand(true)
	t.Scroll.View.SetHeadersVisible(false)
	t.Scroll.View.SetReorderable(false)
	t.Scroll.View.SetActivateOnSingleClick(true)
	t.Scroll.View.SetRubberBanding(true)

	for i, col := range newTreeColumns() {
		t.Scroll.View.InsertColumn(col, i)
	}

	t.Scroll.View.SetTooltipColumn(columnName)
	t.Scroll.View.SetSearchColumn(columnPath)
	t.Scroll.View.SetEnableSearch(true)

	t.Scroll.View.ConnectRowExpanded(func(iter *gtk.TreeIter, path *gtk.TreePath) {
		d, ok := t.root.Entry(t.root.IterPath(iter)).(*TreeDir)
		if ok {
			t.setBusy()
			d.Init(t.ctx, func() {
				// Re-expand the path after loading.
				t.Scroll.View.ExpandToPath(d.TreePath())
				t.setDone()
			})
		}
	})

	t.Scroll.ScrolledWindow = gtk.NewScrolledWindow()
	t.Scroll.SetVExpand(true)
	t.Scroll.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	t.Scroll.SetChild(t.Scroll.View)

	t.spinner = gtk.NewSpinner()
	t.spinner.SetSizeRequest(32, 32)
	loadingCSS(t.spinner)

	t.spinbox = gtk.NewRevealer()
	t.spinbox.SetTransitionDuration(65)
	t.spinbox.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	t.spinbox.SetRevealChild(false)
	t.spinbox.SetChild(t.spinner)
	t.spinbox.NotifyProperty("reveal-child", func() {
		if t.spinbox.RevealChild() {
			t.spinner.Start()
		} else {
			t.spinner.Stop()
		}
	})

	t.Actions = gtk.NewActionBar()
	t.Actions.PackStart(t.spinbox)
	t.Actions.PackEnd(newFnButton("folder-new-symbolic", "New Folder", t.newFolder))
	t.Actions.PackEnd(newFnButton("document-new-symbolic", "New File", t.newFile))

	t.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	t.Box.Append(t.Scroll)
	t.Box.Append(t.Actions)
	boxCSS(t.Box)

	return &t
}

// Refresh asynchronously refreshes the tree.
func (t *Tree) Refresh() {
	t.SetSensitive(false)
	t.setBusy()
	t.root.Refresh(t.ctx, func() {
		t.Scroll.View.ExpandToPath(t.root.RootPath())
		t.setDone()
		t.SetSensitive(true)
	})
}

// Load asynchronously loads the given path. The path must be absolute,
// otherwise Load panics.
func (t *Tree) Load(path string) {
	if !filepath.IsAbs(path) {
		panic("Load not given an absolute path: " + path)
	}

	t.root = newTreeRoot(path)
	t.Scroll.View.SetModel(t.root.Model())

	t.Refresh()
}

// ConnectFileActivated connects f to be called when a file is selected. It
// does not call on folders.
func (t *Tree) ConnectFileActivated(f func(path string)) {
	t.Scroll.View.ConnectRowActivated(func(path *gtk.TreePath, column *gtk.TreeViewColumn) {
		// TODO: handle symlinks: resolve the paths, expand it fully, then call
		// f. If the path points to outside the workspace, then just ignore.
		switch entry := t.root.EntryFromTreePath(path).(type) {
		case *TreeFile:
			f(entry.FilePath())
		}
	})
}

// SetSensitive sets whether or not the Tree is sensitive.
func (t *Tree) SetSensitive(sensitive bool) {
	t.Scroll.SetSensitive(sensitive)
	t.Actions.SetCanTarget(sensitive)
}

// RelPath returns the path relative to the Tree's root path. If path is not an
// absolute path that points to within the root path, then the original string
// is returned.
func (t *Tree) RelPath(path string) string {
	p, err := filepath.Rel(t.root.FilePath(), path)
	if err != nil {
		return path
	}
	return p
}

// SetUnsaved sets whether the path is saved or not.
func (t *Tree) SetUnsaved(path string, unsaved bool) {
	t.root.WalkEntry(path, func(entry TreeEntry) bool {
		switch entry := entry.(type) {
		case *TreeDir:
			// Keep track of the number of unsaved files in the directory as we
			// traverse down.
			if unsaved {
				entry.unsaved++
			} else {
				entry.unsaved--
			}
			entry.SetUnsaved(entry.unsaved > 0)
		case *TreeFile:
			entry.SetUnsaved(unsaved)
		}
		return true
	})
}

func (t *Tree) newFile() {}

func (t *Tree) newFolder() {}

func (t *Tree) setBusy() {
	// t.progress.Show()
	// t.spinner.Show()
	t.spinbox.SetRevealChild(true)
}

func (t *Tree) setDone() {
	// t.progress.Hide()
	// t.spinner.Hide()
	t.spinbox.SetRevealChild(false)
}

func newTreeColumns() []*gtk.TreeViewColumn {
	return []*gtk.TreeViewColumn{
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererPixbuf()
			ren.SetAlignment(0, 0.5)
			ren.SetPadding(0, 4)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, false)
			col.AddAttribute(ren, "icon-name", int(columnIcon))
			col.AddAttribute(ren, "sensitive", int(columnSensitive))
			col.SetSizing(gtk.TreeViewColumnFixed)

			return col
		}(),
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererText()
			ren.SetPadding(3, 4)
			ren.SetObjectProperty("ellipsize", pango.EllipsizeMiddle)
			ren.SetObjectProperty("ellipsize-set", true)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, true)
			col.AddAttribute(ren, "markup", int(columnName))
			col.AddAttribute(ren, "sensitive", int(columnSensitive))
			col.SetSizing(gtk.TreeViewColumnAutosize)
			col.SetExpand(true)

			return col
		}(),
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererText()
			ren.SetPadding(3, 0)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, false)
			col.AddAttribute(ren, "text", int(columnUnsaved))
			col.AddAttribute(ren, "sensitive", int(columnSensitive))
			col.SetSizing(gtk.TreeViewColumnAutosize)

			return col
		}(),
	}
}

func newFnButton(icon, tooltip string, f func()) *gtk.Button {
	button := gtk.NewButtonFromIconName(icon)
	button.SetTooltipText(tooltip)
	button.ConnectClicked(f)
	return button
}
