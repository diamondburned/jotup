package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/jotup/internal/extern/js/asciimath"
	"github.com/diamondburned/jotup/internal/ui/editor"
	"github.com/diamondburned/jotup/internal/ui/filetree"
	"github.com/diamondburned/jotup/internal/ui/greet"
	"github.com/diamondburned/jotup/internal/ui/math"
)

func main() {
	app := app.New("com.diamondburned.jotup", "Jotup")
	app.ConnectActivate(func(context.Context) {
		gtksource.Init()
		adaptive.Init()
	})
	app.ConnectActivate(activate)

	os.Exit(app.Run(context.Background(), os.Args))
}

type key struct {
	val  uint
	mods gdk.ModifierType
}

type window struct {
	*app.Window
	ctx  context.Context
	keys map[key]func() bool

	Stack *gtk.Stack
	Greet struct {
		*gtk.Box
		Header *gtk.HeaderBar
		Body   *greet.View
	}
	Main struct {
		*adaptive.Fold
		Left struct {
			*gtk.Box
			Title *gtk.Label // possibly hidden
			Files *filetree.Tree
		}
		Right struct {
			*gtk.Box
			Header *gtk.Label
			Editor *editor.View
		}
	}
}

var _ = cssutil.WriteCSS(`
	windowhandle, headerbar {
		min-height: 0;
	}
	windowhandle > box {
		min-height: 40px;
	}
	windowhandle button:not(:hover):not(:active):not(:checked) {
		background: none;
		box-shadow: none;
		border: none;
	}
	scrolledwindow scrollbar {
		box-shadow: none;
		background: none;
		border: none;
	}
	headerbar .title {
		font-weight: bold;
		padding-right: 8px;
	}
	.main-left {
		border-right: 1px solid @borders;
	}
	.main-left > windowhandle > box {
		margin: 0 4px;
	}
	.main-right headerbar {
		border-bottom: 1px solid @borders;
	}
	.editor-viewbox gutter,
	.adaptive-sidebar-reveal-button {
		min-width: 50px;
	}
	.adaptive-sidebar-reveal-button button {
		margin: 0;
	}
	.main-right windowhandle > box {
		padding-left: 0;
	}
	.main-right windowhandle label {
		margin-left: 2px;
	}
`)

var initialized bool

func activate(ctx context.Context) {
	if !initialized {
		initialized = true

		// Load saved preferences.
		prefs.AsyncLoadSaved(ctx, nil)

		app := app.FromContext(ctx)
		app.AddActions(map[string]func(){
			"app.preferences": func() { prefui.ShowDialog(ctx) },
			// "app.about":       func() { about.Show(ctx) },
			"app.logs": func() { logui.ShowDefaultViewer(ctx) },
			"app.quit": func() { app.Quit() },
		})
	}

	win := app.FromContext(ctx).NewWindow()
	win.SetTitle("")
	win.SetDefaultSize(800, 600)
	win.NewWindowHandle() // empty header

	ctx = app.WithWindow(ctx, win)

	w := window{}
	w.ctx = ctx
	w.keys = make(map[key]func() bool)
	w.Window = win

	binder := gtk.NewEventControllerKey()
	binder.ConnectKeyPressed(w.handleKeybinds)
	w.AddController(binder)

	w.createMain(ctx)
	w.createGreeter(ctx)

	w.Stack = gtk.NewStack()
	w.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	w.Stack.AddChild(w.Main)
	w.Stack.AddChild(w.Greet)

	w.SwitchToGreeter()

	// Intercept window closes in case the window is closing on an unsaved file.
	// Use a boolean to keep track of whether or not we've already asked, since
	// Close will form an infinite recursion otherwise.
	var wreckHavoc bool
	w.ConnectCloseRequest(func() bool {
		if /* !w.Main.Right.Editor.IsUnsaved() || */ wreckHavoc {
			return false // allow closing
		}
		w.AskBufferDestroy(func() {
			wreckHavoc = true
			w.Close()
		})
		return true
	})

	win.SetChild(w.Stack)
	win.Show()
}

func (w *window) SwitchToMain() {
	w.Stack.SetVisibleChild(w.Main)
}

func (w *window) createMain(ctx context.Context) {
	w.Main.Fold = adaptive.NewFold(gtk.PosLeft)
	w.Main.SetFoldThreshold(600)
	w.Main.SetFoldWidth(250)

	/*
	 * Left
	 */

	w.Main.Left.Title = gtk.NewLabel("Jotup")
	w.Main.Left.Title.AddCSSClass("title")
	w.Main.Left.Title.AddCSSClass("main-branding")
	w.Main.Left.Title.SetEllipsize(pango.EllipsizeStart)
	w.Main.Left.Title.SetXAlign(0)
	w.Main.Left.Title.SetHExpand(true)

	leftControls := gtk.NewWindowControls(gtk.PackStart)

	leftTop := gtk.NewStack()
	leftTop.SetHExpand(true)
	leftTop.SetTransitionType(gtk.StackTransitionTypeNone)
	leftTop.AddChild(w.Main.Left.Title)
	leftTop.AddChild(leftControls)

	// Do some slight trickery with this widget: if there are no buttons, then
	// we can show the branding label. Otherwise, show the button.
	updateControls := func() {
		if leftControls.Empty() {
			leftTop.SetVisibleChild(w.Main.Left.Title)
			w.Main.Left.Title.Show()
		} else {
			leftTop.SetVisibleChild(leftControls)
			w.Main.Left.Title.Hide()
		}
	}

	leftControls.NotifyProperty("empty", updateControls)
	updateControls()

	leftMenu := gtk.NewMenuButton()
	leftMenu.SetVAlign(gtk.AlignCenter)
	leftMenu.SetIconName("open-menu-symbolic")
	leftMenu.SetMenuModel(gtkutil.MenuPair([][2]string{
		{"Preferences", "app.preferences"},
		{"Logs", "app.logs"},
		{"Quit", "app.quit"},
	}))

	leftTopBox := gtk.NewBox(gtk.OrientationHorizontal, 0)
	leftTopBox.SetHExpand(true)
	leftTopBox.Append(leftTop)
	leftTopBox.Append(leftMenu)

	leftHeader := gtk.NewWindowHandle()
	leftHeader.SetChild(leftTopBox)

	w.Main.Left.Files = filetree.NewTree(ctx)
	w.Main.Left.Files.ConnectFileActivated(func(path string) {
		w.Main.Right.Editor.Load(path)
		w.Main.Right.Header.SetText(w.Main.Left.Files.RelPath(path))
	})

	w.Main.Left.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	w.Main.Left.AddCSSClass("main-left")
	w.Main.Left.Append(leftHeader)
	w.Main.Left.Append(w.Main.Left.Files)

	/*
	 * Right
	 */

	back := adaptive.NewFoldRevealButton()
	back.Button.SetHAlign(gtk.AlignCenter)
	back.Button.SetVAlign(gtk.AlignCenter)
	back.SetIconName("view-list-symbolic")
	back.ConnectFold(w.Main.Fold)

	w.Main.Right.Header = gtk.NewLabel("")
	w.Main.Right.Header.SetHExpand(true)
	w.Main.Right.Header.SetXAlign(0)

	rightHeader := gtk.NewHeaderBar()
	rightHeader.SetShowTitleButtons(false)
	rightHeader.SetTitleWidget(emptyWidget())
	rightHeader.PackStart(back)
	rightHeader.PackStart(w.Main.Right.Header)
	rightHeader.PackEnd(gtk.NewWindowControls(gtk.PackEnd))

	w.Main.Right.Editor = editor.NewView(ctx, w)

	w.Main.Right.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	w.Main.Right.AddCSSClass("main-right")
	w.Main.Right.Append(rightHeader)
	w.Main.Right.Append(w.Main.Right.Editor)

	/*
	 * Finalize
	 */

	w.Main.SetSideChild(w.Main.Left)
	w.Main.SetChild(w.Main.Right)
}

func emptyWidget() gtk.Widgetter {
	return gtk.NewBox(gtk.OrientationHorizontal, 0)
}

func (w *window) SwitchToGreeter() {
	w.Stack.SetVisibleChild(w.Greet)
}

func (w *window) createGreeter(ctx context.Context) {
	w.Greet.Body = greet.NewView(ctx, w.LoadFolder)

	w.Greet.Header = gtk.NewHeaderBar()
	w.Greet.Header.SetShowTitleButtons(true)

	w.Greet.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	w.Greet.Box.SetHExpand(true)
	w.Greet.Box.SetVExpand(true)
	w.Greet.Box.Append(w.Greet.Header)
	w.Greet.Box.Append(w.Greet.Body)
}

func (w *window) LoadFolder(path string) {
	// TODO: unsaved dialog

	w.Main.Left.Files.Load(path)
	w.Main.Left.Title.SetText(cleanPath(path))
	w.SwitchToMain()
}

func cleanPath(path string) string {
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

const unsavedMark = " " + filetree.UnsavedDot

// InvalidateUnsaved implements editor.Controller.
func (w *window) InvalidateUnsaved() {
	edit := w.Main.Right.Editor
	name := w.Main.Left.Files.RelPath(edit.Path())

	if edit.IsUnsaved() {
		w.Main.Right.Header.SetText(name + unsavedMark)
		w.Main.Left.Files.SetUnsaved(name, true)
	} else {
		w.Main.Right.Header.SetText(name)
		w.Main.Left.Files.SetUnsaved(name, false)
	}
}

// AskBufferDestroy implements editor.Controller.
func (w *window) AskBufferDestroy(do func()) {
	if !w.Main.Right.Editor.IsUnsaved() {
		glib.IdleAdd(do)
		return
	}

	dialog := gtk.NewMessageDialog(
		&w.Window.Window,
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
			w.Main.Right.Editor.DiscardChanges()
			do()
		}
		dialog.Destroy()
	})
	dialog.Show()
}

// BindKey binds key to the given function. All keybinds added using this
// function will be invoked before any other application keybinds.
func (w *window) BindKey(accel string, f func() bool) error {
	val, mods, ok := gtk.AcceleratorParse(accel)
	if !ok {
		return fmt.Errorf("invalid acceleration %q", accel)
	}

	w.keys[key{val, mods}] = f
	return nil
}

func (w *window) handleKeybinds(keyval, _ uint, state gdk.ModifierType) (ok bool) {
	f, ok := w.keys[key{keyval, state}]
	if ok {
		return f()
	}
	return false
}

func activate2(ctx context.Context) {
	app := app.FromContext(ctx)

	langman := gtksource.LanguageManagerGetDefault()
	llatex := langman.Language("latex")

	lbuffer := gtksource.NewBufferWithLanguage(llatex)
	latex := gtksource.NewViewWithBuffer(lbuffer)

	latexs := gtk.NewScrolledWindow()
	latexs.SetHExpand(true)
	latexs.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	latexs.SetChild(latex)

	mathv := math.NewMathTransformer()
	mathv.SetHExpand(true)
	mathv.SetScale(1.15)

	maths := gtk.NewScrolledWindow()
	maths.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	maths.SetChild(mathv)

	lbuf := latex.Buffer()
	lbuf.ConnectChanged(func() {
		start, end := lbuf.Bounds()
		mathv.ShowText(lbuf.Text(start, end, false))
	})
	lbuf.SetText("R")

	go func() {
		// m, err := katex.NewModule(ctx)
		m, err := asciimath.NewModule(ctx)
		if err != nil {
			glib.IdleAdd(func() { mathv.ShowError(err) })
			return
		}

		glib.IdleAdd(func() {
			mathv.SetTransformer(func(str string) (string, error) {
				// return m.Render(str, true)
				return m.Render(str)
			})
		})
	}()

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.SetHomogeneous(true)
	box.Append(latexs)
	box.Append(maths)

	win := app.NewWindow()
	win.SetTitle("Scratch")
	win.SetChild(box)
	win.Show()

	fchoose := gtk.NewFileChooserNative("test", &win.Window, gtk.FileChooserActionSelectFolder, "Choose", "Cancel")
	fchoose.ConnectResponse(func(resp int) {
		if resp == int(gtk.ResponseAccept) {
			log.Println("path =", fchoose.File().Path())
		}
	})
	fchoose.Show()
}
