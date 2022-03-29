package jotup

import (
	"context"
	"fmt"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil"
)

type key struct {
	val  uint
	mods gdk.ModifierType
}

// Window is the main Jotup window.
type Window struct {
	*app.Window
	ctx  context.Context
	keys map[key]func() bool

	Stack   *gtk.Stack
	Greeter *GreeterPage
	Editor  *EditorPage
}

// NewWindowFromEditor creates a new Window that has the path of the given
// EditorPage opened.
func NewWindowFromEditor(ctx context.Context, editor *EditorPage) *Window {
	w := NewWindow(ctx)
	w.SwitchToEditor()
	w.Editor.loadFolder(editor.Files.Path())
	w.Editor.selectFile(editor.Editor.Path())
	return w
}

// NewWindow creates a new Window.
func NewWindow(ctx context.Context) *Window {
	win := app.FromContext(ctx).NewWindow()
	win.SetTitle("")
	win.SetDefaultSize(800, 600)
	win.NewWindowHandle() // empty header

	ctx = app.WithWindow(ctx, win)

	w := Window{
		Window: win,
		ctx:    ctx,
		keys:   make(map[key]func() bool),
	}

	binder := gtk.NewEventControllerKey()
	binder.ConnectKeyPressed(w.handleKeybinds)
	w.AddController(binder)

	w.Greeter = NewGreeter(ctx, w.Load)
	w.Editor = NewEditorPage(ctx)

	w.Stack = gtk.NewStack()
	w.Stack.SetTransitionType(gtk.StackTransitionTypeCrossfade)
	w.Stack.AddChild(w.Editor)
	w.Stack.AddChild(w.Greeter)

	w.SwitchToGreeter()

	// Intercept window closes in case the window is closing on an unsaved file.
	// Use a boolean to keep track of whether or not we've already asked, since
	// Close will form an infinite recursion otherwise.
	var wreckHavoc bool
	w.ConnectCloseRequest(func() bool {
		if !w.Editor.Editor.IsUnsaved() || wreckHavoc {
			return false // allow closing
		}
		w.Editor.AskBufferDestroy(func() {
			wreckHavoc = true
			w.Close()
		})
		return true
	})

	win.SetChild(w.Stack)

	gtkutil.BindActionMap(win, map[string]func(){
		"win.open":              w.Greeter.PromptOpenFolder,
		"win.open-copy":         w.Editor.OpenCopy,
		"win.refresh":           w.Editor.Files.Refresh,
		"win.switch-to-greeter": w.SwitchToGreeter,
	})

	return &w
}

// SwitchToEditor switches the visible page to the Editor page.
func (w *Window) SwitchToEditor() {
	w.Stack.SetVisibleChild(w.Editor)
}

// SwitchToGreeter switches the visible page to the Greeter (Home) page.
func (w *Window) SwitchToGreeter() {
	w.Stack.SetVisibleChild(w.Greeter)
}

// Load loads the given file or folder. It calls SwitchToEditor.
func (w *Window) Load(path string) {
	w.SwitchToEditor()
	w.Editor.Load(path)
}

// BindKey binds key to the given function. All keybinds added using this
// function will be invoked before any other application keybinds.
func (w *Window) BindKey(accel string, f func() bool) error {
	val, mods, ok := gtk.AcceleratorParse(accel)
	if !ok {
		return fmt.Errorf("invalid acceleration %q", accel)
	}

	w.keys[key{val, mods}] = f
	return nil
}

func (w *Window) handleKeybinds(keyval, _ uint, state gdk.ModifierType) (ok bool) {
	f, ok := w.keys[key{keyval, state}]
	if ok {
		return f()
	}
	return false
}
