// Package editor implements a Markdown editor with preview.
package editor

import (
	"context"
	"fmt"
	"html"
	"time"

	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/jotup/internal/ui/components/toast"

	coreglib "github.com/diamondburned/gotk4/pkg/core/glib"
)

var lineWrap = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Wrap Lines",
	Section:     "Editor",
	Description: "Wrap lines that are too long.",
})

var showLineNumbers = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Line Numbers",
	Section:     "Editor",
	Description: "Show line numbers.",
})

var showLineMarks = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Line Marks",
	Section:     "Editor",
	Description: "Show line marks.",
})

var highlightCurrentLine = prefs.NewBool(true, prefs.PropMeta{
	Name:    "Highlight Current Line",
	Section: "Editor",
})

var autoIndent = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Auto-indent",
	Section:     "Editor",
	Description: "Automatically indent on typing.",
})

var insertSpaces = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Insert Spaces",
	Section:     "Editor",
	Description: "Insert spaces instead of tabs on Tab key.",
})

var highlightSyntax = prefs.NewBool(true, prefs.PropMeta{
	Name:        "Highlight Syntax",
	Section:     "Editor",
	Description: "Enable syntax highlighting.",
})

var showMinimap = prefs.NewBool(false, prefs.PropMeta{
	Name:        "Show Minimap",
	Section:     "Editor",
	Description: "Show the minimap of the entire file on the right.",
})

func bindBoolProp(w *gtksource.View, boolProp *prefs.Bool, prop string, fs ...func()) {
	boolProp.SubscribeWidget(w, func() {
		w.SetObjectProperty(prop, boolProp.Value())
		for _, f := range fs {
			f()
		}
	})
}

var tabWidth = prefs.NewInt(4, prefs.IntMeta{
	Name:        "Tab Width",
	Section:     "Editor",
	Description: "The width (in characters) of a single tab character.",
	Min:         0,
	Max:         32,
})

var lineIndent = prefs.NewInt(0, prefs.IntMeta{
	Name:    "Line Indent",
	Section: "Editor",
	Description: "The amount of padding in each non-wrapped line. " +
		"If negative, then wrapped lines are padded instead.",
})

var columnLine = prefs.NewInt(80, prefs.IntMeta{
	Name:        "Column Line",
	Section:     "Editor",
	Description: "Show a dim vertical line at the column position as a hint.",
})

func bindIntProp(w *gtksource.View, intProp *prefs.Int, prop string, fs ...func()) {
	intProp.SubscribeWidget(w, func() {
		w.SetObjectProperty(prop, intProp.Value())
		for _, f := range fs {
			f()
		}
	})
}

// Controller describes the parent widget that controls the editor.View widget.
type Controller interface {
	// InvalidateUnsaved invalidates whether or not the file is unsaved. The
	// parent controller should implement this method to indicate the user
	// whether a file still has unsaved changes.
	InvalidateUnsaved()
	// AskBufferDestroy informs the user for a potentially destructive buffer
	// action because of unsaved changes.
	AskBufferDestroy(destroy func())
}

// View is a Markdown editor sandwiched with a Markdown preview.
type View struct {
	*gtk.Overlay
	Toast *toast.Toast
	Box   struct {
		*gtk.Box
		View    *gtksource.View
		Preview *gtk.Box // TODO
	}

	progrev  *gtk.Revealer
	progress *gtk.ProgressBar

	ctx  context.Context
	ctrl Controller

	minimap *gtksource.Map
	buffer  *gtksource.Buffer
	file    *gtksource.File

	path    string
	untrack bool // only change within AskBufferDestroy
	unsaved bool
}

var loadingCSS = cssutil.Applier("editor-loading", `
	.editor-loading trough {
		min-height: 0px;
	}
	.editor-loading progress {
		min-height: 2px;
	}
`)

// toast dark styling override
var _ = cssutil.WriteCSS(`
	.editor-dark .toast-toast > revealer > box {
		background-color: alpha(white, 0.2);
	}
`)

var minimapCSS = cssutil.Applier("editor-minimap", `
	.editor-minimap child:active slider {
		background-color: rgba(0, 0, 0, 0.25);
	}
	.editor-dark .editor-minimap child:active slider {
		background-color: rgba(255, 255, 255, 0.25);
	}
`)

// NewView creates a new View.
func NewView(ctx context.Context, ctrl Controller) *View {
	v := View{ctx: ctx, ctrl: ctrl}
	v.file = gtksource.NewFile()
	v.buffer = gtksource.NewBuffer(nil)
	v.buffer.ConnectChanged(func() { v.markEdited(true) })

	v.Toast = toast.NewToast(gtk.PackStart)
	v.Toast.SetLog(true)
	v.Toast.SetTimeout(5 * time.Second)

	v.Box.View = gtksource.NewViewWithBuffer(v.buffer)
	v.Box.View.SetEnableSnippets(true)
	v.Box.View.SetMonospace(true)
	v.Box.View.SetEditable(false)
	v.Box.View.SetBottomMargin(100)

	gutterChange := func() { moveLineNumLast(v.Box.View) }
	bindBoolProp(v.Box.View, showLineMarks, "show-line-marks", gutterChange)
	bindBoolProp(v.Box.View, showLineNumbers, "show-line-numbers", gutterChange)
	bindBoolProp(v.Box.View, insertSpaces, "insert-spaces-instead-of-tabs")
	bindBoolProp(v.Box.View, highlightCurrentLine, "highlight-current-line")
	bindBoolProp(v.Box.View, autoIndent, "auto-indent")

	bindIntProp(v.Box.View, tabWidth, "tab-width")
	bindIntProp(v.Box.View, lineIndent, "indent")
	bindIntProp(v.Box.View, columnLine, "right-margin-position", func() {
		v.Box.View.SetShowRightMargin(columnLine.Value() > 0)
	})

	highlightSyntax.SubscribeWidget(v.Box.View, func() {
		v.buffer.SetHighlightSyntax(highlightSyntax.Value())
	})

	scheme.SubscribeWidget(v.Box.View, func() {
		scheme := scheme.GuessValue(v.Box.View)
		v.buffer.SetStyleScheme(scheme)

		v.RemoveCSSClass("editor-dark")
		if schemeIsDark(scheme) {
			v.AddCSSClass("editor-dark")
		}
	})

	lineWrap.SubscribeWidget(v.Box.View, func() {
		if lineWrap.Value() {
			v.Box.View.SetWrapMode(gtk.WrapWordChar)
		} else {
			v.Box.View.SetWrapMode(gtk.WrapNone)
		}
	})

	textScroll := gtk.NewScrolledWindow()
	textScroll.SetVExpand(true)
	textScroll.SetHExpand(true)
	textScroll.SetChild(v.Box.View)

	minimapBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	minimapBox.Append(textScroll)

	showMinimap.SubscribeWidget(v.Box.View, func() {
		if v.minimap != nil {
			minimapBox.Remove(v.minimap)
			textScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAlways)
			v.minimap = nil
		}

		if showMinimap.Value() {
			v.minimap = gtksource.NewMap()
			v.minimap.SetView(v.Box.View)
			v.minimap.SetHAlign(gtk.AlignEnd)
			v.minimap.SetVAlign(gtk.AlignFill)
			v.minimap.SetVExpand(true)
			minimapCSS(v.minimap)

			minimapBox.Append(v.minimap)
			textScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyExternal)
		}
	})

	v.Box.Preview = gtk.NewBox(gtk.OrientationHorizontal, 0)

	v.Box.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Box.AddCSSClass("editor-viewbox")
	v.Box.Append(minimapBox)
	v.Box.Append(v.Box.Preview)

	v.progress = gtk.NewProgressBar()
	v.progress.AddCSSClass("osd")
	v.progress.SetPulseStep(0.05)
	v.progress.SetCanTarget(false)
	loadingCSS(v.progress)

	v.progrev = gtk.NewRevealer()
	v.progrev.SetTransitionDuration(50)
	v.progrev.SetTransitionType(gtk.RevealerTransitionTypeCrossfade)
	v.progrev.SetHExpand(true)
	v.progrev.SetVAlign(gtk.AlignStart)
	v.progrev.SetChild(v.progress)

	var progressHandle glib.SourceHandle
	v.progrev.NotifyProperty("reveal-child", func() {
		if progressHandle > 0 {
			glib.SourceRemove(progressHandle)
			progressHandle = 0
		}
		if v.progrev.RevealChild() {
			progressHandle = glib.TimeoutAddPriority(
				1000/30, glib.PriorityDefaultIdle,
				func() bool {
					v.progress.Pulse()
					return true
				},
			)
		}
	})
	v.progrev.SetRevealChild(false)

	v.Overlay = gtk.NewOverlay()
	v.Overlay.SetHExpand(true)
	v.Overlay.AddOverlay(v.progrev)
	v.Overlay.AddOverlay(v.Toast)
	v.Overlay.SetChild(v.Box)

	gtkutil.BindKeys(v, map[string]func() bool{
		"<Ctrl>S": func() bool {
			v.SetBusy(false)
			v.Save(func(err error) { v.UnsetBusy() })
			return true
		},
	})

	return &v
}

// SetOrientation sets the View's orientation, which determines whether the
// preview is at the right or at the bottom.
func (v *View) SetOrientation(orientation gtk.Orientation) {
	v.Box.SetOrientation(orientation)
}

// Path returns the View's path to the currently edited file.
func (v *View) Path() string {
	return v.path
}

// Load asynchronously loads the file at path into the buffer.
func (v *View) Load(path string) {
	v.LoadFile(gio.NewFileForPath(path))
}

// LoadFile asynchronously loads the gio.File into the buffer.
func (v *View) LoadFile(file gio.Filer) {
	v.ctrl.AskBufferDestroy(func() {
		v.path = file.Path()
		v.file.SetLocation(file)
		v.Refresh()
	})
}

// SetBusy sets the editor into a busy state. If disable is true, then the user
// cannot interact with the editor.
func (v *View) SetBusy(disable bool) {
	v.Box.SetSensitive(!disable)
	v.progrev.SetRevealChild(true)
}

// UnsetBusy unsets busy state.
func (v *View) UnsetBusy() {
	v.Box.SetSensitive(true)
	v.progrev.SetRevealChild(false)
}

// Refresh refreshes the editor to reload the current file.
func (v *View) Refresh() {
	v.untrack = true
	v.SetBusy(true)

	loader := gtksource.NewFileLoader(v.buffer, v.file)
	loader.LoadAsync(v.ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		v.UnsetBusy()
		v.untrack = false

		if err := loader.LoadFinish(result); err != nil {
			v.buffer.Delete(v.buffer.Bounds())
			v.buffer.InsertMarkup(v.buffer.StartIter(), fmt.Sprintf(
				`<span color="red"><b>Error:</b></span> %s`,
				html.EscapeString(err.Error()),
			))

			// Ensure the buffer is at a fresh state and can't be saved.
			v.buffer.SetLanguage(nil)
			v.file.SetLocation(nil)
			v.Box.View.SetEditable(false)
			return
		}

		file := v.file.Location()
		langman := gtksource.LanguageManagerGetDefault()
		v.buffer.SetLanguage(langman.GuessLanguage(file.Basename(), ""))
		v.Box.View.SetEditable(true)
	})
}

// Save asynchronously saves the file. The given callback is invoked when it's
// done.
func (v *View) Save(done func(error)) {
	v.SetBusy(false)

	saver := gtksource.NewFileSaver(v.buffer, v.file)
	saver.SaveAsync(v.ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		v.UnsetBusy()

		if err := saver.SaveFinish(result); err != nil {
			v.Toast.Show("Error: " + err.Error())
			done(err)
		} else {
			v.markEdited(false)
			done(nil)
		}
	})
}

// IsUnsaved returns true if the View still has changes that haven't been saved
// yet.
func (v *View) IsUnsaved() bool { return v.unsaved }

// DiscardChanges resets the unsaved state. The buffer isn't actually refreshed,
// so it still contains unsaved changes. The caller should manually refresh it
// if the buffer isn't going to be wiped.
func (v *View) DiscardChanges() {
	v.unsaved = false
	v.ctrl.InvalidateUnsaved()
}

func (v *View) markEdited(edited bool) {
	if !v.untrack {
		v.unsaved = edited
		v.ctrl.InvalidateUnsaved()
	}
}

func isGutterRendererText(obj glib.Objector) bool {
	t := coreglib.InternObject(obj).TypeFromInstance().String()
	return t == "GtkSourceGutterRendererLines"
}

// moveLineNumLast puts the line numbers on the rightmost column near the texts.
func moveLineNumLast(view *gtksource.View) {
	gutter := view.Gutter(gtk.TextWindowLeft)

	// Walk the tree to get the number of widgets.
	var num int
	for w := gutter.FirstChild(); w != nil; w = gtk.BaseWidget(w).NextSibling() {
		num++
	}

	// Find the line number gutter.
	for w := gutter.FirstChild(); w != nil; w = gtk.BaseWidget(w).NextSibling() {
		if isGutterRendererText(w) {
			gutter.Reorder(w.(gtksource.GutterRendererer), num)
			break
		}
	}
}
