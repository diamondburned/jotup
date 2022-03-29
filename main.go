package main

import (
	"context"
	"os"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/components/logui"
	"github.com/diamondburned/gotkit/components/prefui"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/jotup/internal/jotup"
)

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

func main() {
	app := app.NewWithFlags(
		"com.diamondburned.jotup", "Jotup",
		gio.ApplicationHandlesOpen,
	)

	app.ConnectStartup(func() {
		gtksource.Init()
		adaptive.Init()

		// Load saved preferences.
		prefs.AsyncLoadSaved(app.Context(), nil)

		app.AddActions(map[string]func(){
			"app.preferences": func() { prefui.ShowDialog(app.Context()) },
			"app.logs":        func() { logui.ShowDefaultViewer(app.Context()) },
			"app.quit":        func() { app.Quit() },
		})
	})

	app.ConnectActivate(func() {
		w := jotup.NewWindow(app.Context())
		w.Show()
	})

	app.ConnectOpen(func(files []gio.Filer, hint string) {
		for _, file := range files {
			w := jotup.NewWindow(app.Context())
			w.Show()
			w.Load(file.Path())
		}
	})

	os.Exit(app.Run(context.Background(), os.Args))
}

/*
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
*/
