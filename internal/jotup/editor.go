package jotup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/jotup/internal/jotup/editor"
	"github.com/diamondburned/jotup/internal/jotup/filetree"
)

// EditorPage is the page containing the file tree and the text editor.
type EditorPage struct {
	*adaptive.Fold
	ctx context.Context

	Left      *gtk.Box
	LeftLabel *gtk.Label
	Files     *filetree.Tree

	Right       *gtk.Box
	RightLabel  *gtk.Label
	RightButton *gtk.MenuButton
	Editor      *editor.View
}

// NewEditorPage creates a new EditorPage.
func NewEditorPage(ctx context.Context) *EditorPage {
	p := EditorPage{ctx: ctx}
	p.Fold = adaptive.NewFold(gtk.PosLeft)
	p.Fold.SetFoldThreshold(600)
	p.Fold.SetFoldWidth(250)

	/*
	 * Left
	 */

	p.LeftLabel = gtk.NewLabel("Jotup")
	p.LeftLabel.AddCSSClass("title")
	p.LeftLabel.AddCSSClass("main-branding")
	p.LeftLabel.SetEllipsize(pango.EllipsizeStart)
	p.LeftLabel.SetXAlign(0)
	p.LeftLabel.SetHExpand(true)

	leftControls := gtk.NewWindowControls(gtk.PackStart)

	leftTop := gtk.NewStack()
	leftTop.SetHExpand(true)
	leftTop.SetTransitionType(gtk.StackTransitionTypeNone)
	leftTop.AddChild(p.LeftLabel)
	leftTop.AddChild(leftControls)

	// Do some slight trickery with this widget: if there are no buttons, then
	// we can show the branding label. Otherwise, show the button.
	updateControls := func() {
		if leftControls.Empty() {
			leftTop.SetVisibleChild(p.LeftLabel)
			p.LeftLabel.Show()
		} else {
			leftTop.SetVisibleChild(leftControls)
			p.LeftLabel.Hide()
		}
	}

	leftControls.NotifyProperty("empty", updateControls)
	updateControls()

	leftMenu := gtk.NewMenuButton()
	leftMenu.SetVAlign(gtk.AlignCenter)
	leftMenu.SetIconName("open-menu-symbolic")
	leftMenu.SetMenuModel(gtkutil.CustomMenuItems(
		gtkutil.MenuItem("_Open", "win.open"),
		gtkutil.MenuItem("Open a _Copy", "win.open-copy"),
		gtkutil.MenuItem("_Refresh Folder", "win.refresh"),
		gtkutil.MenuItem("Back to _Home", "win.switch-to-greeter"),
		gtkutil.MenuSeparator(""),
		gtkutil.MenuItem("_Preferences", "app.preferences"),
		gtkutil.MenuItem("_Logs", "app.logs"),
		gtkutil.MenuItem("_Quit", "app.quit"),
	))

	leftTopBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	leftTopBox.SetHExpand(true)
	leftTopBox.Append(leftTop)
	leftTopBox.Append(leftMenu)

	leftHeader := gtk.NewWindowHandle()
	leftHeader.SetChild(leftTopBox)

	p.Files = filetree.NewTree(ctx)
	p.Files.ConnectFileActivated(func(path string) {
		p.Editor.Load(path)
		p.RightLabel.SetText(p.Files.RelPath(path))
	})

	p.Left = gtk.NewBox(gtk.OrientationVertical, 0)
	p.Left.AddCSSClass("main-left")
	p.Left.Append(leftHeader)
	p.Left.Append(p.Files)

	/*
	 * Right
	 */

	back := adaptive.NewFoldRevealButton()
	back.Button.SetHAlign(gtk.AlignCenter)
	back.Button.SetVAlign(gtk.AlignCenter)
	back.SetIconName("view-list-symbolic")
	back.ConnectFold(p.Fold)

	p.RightLabel = gtk.NewLabel("")
	p.RightLabel.SetHExpand(true)
	p.RightLabel.SetXAlign(0)

	p.RightButton = gtk.NewMenuButton()
	p.RightButton.SetIconName("document-properties-symbolic")
	p.RightButton.SetMenuModel(gtkutil.CustomMenuItems(
		gtkutil.MenuItem("Save", "editor.save"),
		gtkutil.MenuItem("Save As...", "editor.save-as"), // TODO
		gtkutil.MenuItem("Print...", "editor.print"),     // TODO
		gtkutil.MenuSeparator(""),
		gtkutil.MenuItem("Find...", "editor.find"),                         // TODO
		gtkutil.MenuItem("Find and Replace...", "editor.find-and-replace"), // TODO
		gtkutil.MenuSeparator(""),
		gtkutil.MenuItem("Join Lines", "editor.join-lines"),
		gtkutil.MenuItem("Move Line Up", "editor.move-line-up"),
		gtkutil.MenuItem("Move Line Down", "editor.move-line-down"),
		gtkutil.MenuSeparator(""),
		gtkutil.MenuItem("Move to Matching Bracket", "editor.move-to-matching-bracket"),
		gtkutil.MenuSeparator(""),
		gtkutil.Submenu("Change Case...", []gtkutil.PopoverMenuItem{
			gtkutil.MenuItem("To Lower", "editor.change-case-lower"),
			gtkutil.MenuItem("To Upper", "editor.change-case-lower"),
			gtkutil.MenuItem("To Title", "editor.change-case-title"),
			gtkutil.MenuItem("Toggle", "editor.change-case-toggle"),
		}),
	))

	rightHeader := gtk.NewHeaderBar()
	rightHeader.SetShowTitleButtons(false)
	rightHeader.SetTitleWidget(emptyWidget())
	rightHeader.PackStart(back)
	rightHeader.PackStart(p.RightLabel)
	rightHeader.PackEnd(gtk.NewWindowControls(gtk.PackEnd))
	rightHeader.PackEnd(p.RightButton)

	p.Editor = editor.NewView(ctx, (*editorController)(&p))
	gtkutil.BindActionMap(p, p.Editor.ActionFuncs())

	p.Right = gtk.NewBox(gtk.OrientationVertical, 0)
	p.Right.AddCSSClass("main-right")
	p.Right.Append(rightHeader)
	p.Right.Append(p.Editor)

	/*
	 * Finalize
	 */

	p.Fold.SetSideChild(p.Left)
	p.Fold.SetChild(p.Right)

	return &p
}

func emptyWidget() gtk.Widgetter {
	return gtk.NewBox(gtk.OrientationHorizontal, 0)
}

// AskBufferDestroy implements editor.Controller.
func (p *EditorPage) AskBufferDestroy(do func()) {
	if !p.Editor.IsUnsaved() {
		glib.IdleAdd(do)
		return
	}

	window := app.WindowFromContext(p.ctx)
	dialog := gtk.NewMessageDialog(
		&window.Window,
		gtk.DialogDestroyWithParent|gtk.DialogModal|gtk.DialogUseHeaderBar,
		gtk.MessageQuestion,
		gtk.ButtonsYesNo,
	)

	dialog.AddCSSClass("ask-buffer-destroy")
	dialog.SetMarkup("File not saved!")
	dialog.SetObjectProperty("secondary-text", "Proceed?")

	// Color the Yes button red.
	button := dialog.WidgetForResponse(int(gtk.ResponseYes)).(*gtk.Button)
	button.AddCSSClass("destructive-action")

	dialog.ConnectResponse(func(resp int) {
		if resp == int(gtk.ResponseYes) {
			p.Editor.DiscardChanges()
			do()
		}
		dialog.Destroy()
	})
	dialog.Show()
}

// Load loads the given file or folder.
func (p *EditorPage) Load(path string) {
	p.AskBufferDestroy(func() {
		gtkutil.Async(p.ctx, func() func() {
			s, err := os.Stat(path)
			if err != nil {
				app.Error(p.ctx, err)
				return nil
			}

			return func() {
				if s.IsDir() {
					p.loadFolder(path)
				} else {
					p.loadFile(path)
				}
			}
		})
	})
}

func (p *EditorPage) loadFile(path string) {
	dir := filepath.Dir(path)
	p.Files.Load(dir)
	p.LeftLabel.SetText(formatPath(dir))
	p.selectFile(path)
}

func (p *EditorPage) selectFile(path string) {
	p.Editor.DiscardChanges()
	if path != "" {
		p.Files.SelectPath(path)
	}
}

func (p *EditorPage) loadFolder(path string) {
	if path == "" {
		return
	}

	p.Files.Load(path)
	p.LeftLabel.SetText(formatPath(path))
	p.Editor.DiscardChanges()
}

func formatPath(path string) string {
	path = filepath.Clean(path)

	home, err := os.UserHomeDir()
	if err == nil {
		p, err := filepath.Rel(home, path)
		if err == nil {
			path = "~" + string(filepath.Separator) + p
		}
	}

	return path + "/"
}

// OpenCopy opens a new Window with the Editor page's paths.
func (p *EditorPage) OpenCopy() {
	w := NewWindowFromEditor(p.ctx, p)
	w.Show()
}

type editorController EditorPage

const unsavedMark = " " + filetree.UnsavedDot

// InvalidateUnsaved implements editor.Controller.
func (c *editorController) InvalidateUnsaved() {
	name := c.Files.RelPath(c.Editor.Path())
	if c.Editor.IsUnsaved() {
		c.RightLabel.SetText(name + unsavedMark)
		c.Files.SetUnsaved(name, true)
	} else {
		c.RightLabel.SetText(name)
		c.Files.SetUnsaved(name, false)
	}
}

func (c *editorController) AskBufferDestroy(do func()) {
	(*EditorPage)(c).AskBufferDestroy(do)
}
