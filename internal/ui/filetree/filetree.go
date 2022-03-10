package filetree

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// Tree is a file tree.
type Tree struct {
	*gtk.Overlay
	View *gtk.TreeView

	progress *gtk.ProgressBar
	loading  glib.SourceHandle

	ctx  context.Context
	root treeRoot
}

// NewTree creates a new Tree.
func NewTree(ctx context.Context) *Tree {
	t := Tree{ctx: ctx}

	t.View = gtk.NewTreeView()
	t.View.SetVExpand(true)
	t.View.SetHExpand(true)
	t.View.SetHeadersVisible(false)
	t.View.SetEnableTreeLines(true)
	t.View.SetReorderable(false)

	for i, col := range newTreeColumns() {
		t.View.InsertColumn(col, i)
	}

	t.View.SetSearchColumn(columnPath)
	t.View.SetEnableSearch(true)

	t.progress = gtk.NewProgressBar()
	t.progress.AddCSSClass("osd")
	t.progress.AddCSSClass("filetree-loading")
	t.progress.SetPulseStep(0.1)
	t.progress.SetHExpand(true)
	t.progress.SetVAlign(gtk.AlignStart)
	t.progress.SetCanTarget(false)
	t.progress.Hide()

	t.Overlay = gtk.NewOverlay()
	t.Overlay.AddOverlay(t.progress)
	t.Overlay.SetChild(t.View)

	return &t
}

// Refresh asynchronously refreshes the tree.
func (t *Tree) Refresh() {
	t.setBusy()
	t.root.Refresh(t.ctx, func() {
		t.View.ExpandToPath(t.root.RootPath())
		t.setDone()
	})
}

// Load asynchronously loads the given path.
func (t *Tree) Load(path string) {
	t.root = newTreeRoot(path)
	t.View.SetModel(t.root.Model())

	t.Refresh()
}

func (t *Tree) setBusy() {
	t.SetSensitive(false)
	t.progress.Show()

	if t.loading == 0 {
		t.loading = glib.TimeoutAdd(1000/60, func() bool {
			t.progress.Pulse()
			return true
		})
	}
}

func (t *Tree) setDone() {
	t.SetSensitive(true)
	t.progress.Hide()

	glib.SourceRemove(t.loading)
	t.loading = 0
}

func newTreeColumns() []*gtk.TreeViewColumn {
	return []*gtk.TreeViewColumn{
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererPixbuf()
			ren.SetAlignment(0, 0.5)

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, false)
			col.AddAttribute(ren, "icon-name", int(columnIcon))
			col.SetSizing(gtk.TreeViewColumnFixed)

			return col
		}(),
		func() *gtk.TreeViewColumn {
			ren := gtk.NewCellRendererText()

			col := gtk.NewTreeViewColumn()
			col.PackStart(ren, true)
			col.AddAttribute(ren, "text", int(columnName))
			col.SetSizing(gtk.TreeViewColumnAutosize)

			return col
		}(),
	}
}
