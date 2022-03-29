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
	"github.com/diamondburned/jotup/internal/jotup/components/toast"

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

	Box     *gtk.Box
	Source  *gtksource.View
	Preview *gtk.Box

	Minimap *gtksource.Map
	Buffer  *gtksource.Buffer
	File    *gtksource.File

	progrev  *gtk.Revealer
	progress *gtk.ProgressBar

	ctx  context.Context
	ctrl Controller

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
	v.File = gtksource.NewFile()
	v.Buffer = gtksource.NewBuffer(nil)
	v.Buffer.ConnectChanged(func() { v.markEdited(true) })

	v.Toast = toast.NewToast(gtk.PackStart)
	v.Toast.SetLog(true)
	v.Toast.SetTimeout(5 * time.Second)

	v.Source = gtksource.NewViewWithBuffer(v.Buffer)
	v.Source.SetEnableSnippets(true)
	v.Source.SetMonospace(true)
	v.Source.SetEditable(false)
	v.Source.SetBottomMargin(100)

	gutterChange := func() { moveLineNumLast(v.Source) }
	bindBoolProp(v.Source, showLineMarks, "show-line-marks", gutterChange)
	bindBoolProp(v.Source, showLineNumbers, "show-line-numbers", gutterChange)
	bindBoolProp(v.Source, insertSpaces, "insert-spaces-instead-of-tabs")
	bindBoolProp(v.Source, highlightCurrentLine, "highlight-current-line")
	bindBoolProp(v.Source, autoIndent, "auto-indent")

	bindIntProp(v.Source, tabWidth, "tab-width")
	bindIntProp(v.Source, lineIndent, "indent")
	bindIntProp(v.Source, columnLine, "right-margin-position", func() {
		v.Source.SetShowRightMargin(columnLine.Value() > 0)
	})

	highlightSyntax.SubscribeWidget(v.Source, func() {
		v.Buffer.SetHighlightSyntax(highlightSyntax.Value())
	})

	scheme.SubscribeWidget(v.Source, func() {
		scheme := scheme.GuessValue(v.Source)
		v.Buffer.SetStyleScheme(scheme)

		v.RemoveCSSClass("editor-dark")
		if schemeIsDark(scheme) {
			v.AddCSSClass("editor-dark")
		}
	})

	lineWrap.SubscribeWidget(v.Source, func() {
		if lineWrap.Value() {
			v.Source.SetWrapMode(gtk.WrapWordChar)
		} else {
			v.Source.SetWrapMode(gtk.WrapNone)
		}
	})

	textScroll := gtk.NewScrolledWindow()
	textScroll.SetVExpand(true)
	textScroll.SetHExpand(true)
	textScroll.SetChild(v.Source)

	minimapBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	minimapBox.Append(textScroll)

	showMinimap.SubscribeWidget(v.Source, func() {
		if v.Minimap != nil {
			minimapBox.Remove(v.Minimap)
			textScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAlways)
			v.Minimap = nil
		}

		if showMinimap.Value() {
			v.Minimap = gtksource.NewMap()
			v.Minimap.SetView(v.Source)
			v.Minimap.SetHAlign(gtk.AlignEnd)
			v.Minimap.SetVAlign(gtk.AlignFill)
			v.Minimap.SetVExpand(true)
			minimapCSS(v.Minimap)

			minimapBox.Append(v.Minimap)
			textScroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyExternal)
		}
	})

	v.Preview = gtk.NewBox(gtk.OrientationHorizontal, 0)

	v.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Box.AddCSSClass("editor-viewbox")
	v.Box.Append(minimapBox)
	v.Box.Append(v.Preview)

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
			v.Save()
			return true
		},
	})

	return &v
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
		v.File.SetLocation(file)
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

	loader := gtksource.NewFileLoader(v.Buffer, v.File)
	loader.LoadAsync(v.ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		defer func() {
			v.UnsetBusy()
			v.untrack = false
		}()

		if err := loader.LoadFinish(result); err != nil {
			v.Clear()
			v.Buffer.InsertMarkup(v.Buffer.StartIter(), fmt.Sprintf(
				`<span color="red"><b>Error:</b></span> %s`,
				html.EscapeString(err.Error()),
			))
			return
		}

		file := v.File.Location()
		langman := gtksource.LanguageManagerGetDefault()
		v.Buffer.SetLanguage(langman.GuessLanguage(file.Basename(), ""))
		v.Source.SetEditable(true)
	})
}

// Save asynchronously saves the file.
func (v *View) Save() {
	v.save(nil)
}

func (v *View) save(done func(error)) {
	v.SetBusy(false)

	saver := gtksource.NewFileSaver(v.Buffer, v.File)
	saver.SaveAsync(v.ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		v.UnsetBusy()

		err := saver.SaveFinish(result)
		if err != nil {
			v.Toast.Show("Error: " + err.Error())
		} else {
			v.markEdited(false)
		}

		if done != nil {
			done(err)
		}
	})
}

// IsUnsaved returns true if the View still has changes that haven't been saved
// yet.
func (v *View) IsUnsaved() bool { return v.unsaved }

// Clear clears the buffer. It only works if the buffer has been saved.
func (v *View) Clear() {
	// Allow clearing while we're not tracking, so we can call this internally.
	if !v.untrack || !v.unsaved {
		v.Buffer.SetText("")
		v.Buffer.SetLanguage(nil)
		v.File.SetLocation(nil)
		v.Source.SetEditable(false)
	}
}

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

// ActionFuncs returns the editor actions for a menu. The prefix is "editor.".
func (v *View) ActionFuncs() map[string]func() {
	emit := func(name string, args ...interface{}) func() {
		return func() { v.Source.Emit(name, args...) }
	}
	return map[string]func(){
		"editor.save":                     v.Save,
		"editor.undo":                     func() { v.Buffer.Emit("undo") },
		"editor.redo":                     func() { v.Buffer.Emit("redo") },
		"editor.cut":                      emit("cut-clipboard"),
		"editor.copy":                     emit("copy-clipboard"),
		"editor.paste":                    emit("paste-clipboard"),
		"editor.select-all":               emit("select-all", true),
		"editor.unselect-all":             emit("select-all", false),
		"editor.insert-emojis":            emit("insert-emojis"),
		"editor.move-line-up":             emit("move-lines", false),
		"editor.move-line-down":           emit("move-lines", true),
		"editor.join-lines":               emit("join-lines"),
		"editor.move-to-matching-bracket": emit("move-to-matching-bracket"),
		"editor.change-case-lower":        emit("change-case", gtksource.SourceChangeCaseLower),
		"editor.change-case-upper":        emit("change-case", gtksource.SourceChangeCaseUpper),
		"editor.change-case-toggle":       emit("change-case", gtksource.SourceChangeCaseToggle),
		"editor.change-case-title":        emit("change-case", gtksource.SourceChangeCaseTitle),
	}
}
