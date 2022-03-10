package main

import (
	"context"
	"log"
	"os"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/jotup/internal/extern/js/asciimath"
	"github.com/diamondburned/jotup/internal/ui/filetree"
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

func activate(ctx context.Context) {
	app := app.FromContext(ctx)

	tree := filetree.NewTree(ctx)
	tree.Load("/home/diamond/Music")

	win := app.NewWindow()
	win.SetTitle("Scratch")
	win.SetChild(tree)
	win.Show()
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
