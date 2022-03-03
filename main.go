package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/jotup/internal/extern/js/katex"
	"github.com/diamondburned/jotup/internal/ui/math"
)

func main() {
	app := app.New("com.diamondburned.jotup", "Jotup")
	app.ConnectActivate(activate)

	os.Exit(app.Run(context.Background(), os.Args))
}

func activate(ctx context.Context) {
	app := app.FromContext(ctx)

	mathv := math.NewMathView()
	mathv.SetScale(1.15)

	maths := gtk.NewScrolledWindow()
	maths.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	maths.SetChild(mathv)

	loadable := adaptive.NewLoadablePage()
	loadable.SetHExpand(true)
	loadable.SetLoading()

	latex := gtk.NewTextView()

	latexs := gtk.NewScrolledWindow()
	latexs.SetHExpand(true)
	latexs.SetPolicy(gtk.PolicyNever, gtk.PolicyAutomatic)
	latexs.SetChild(latex)

	lbuf := latex.Buffer()
	lbuf.SetText("\\R")

	var katexMod *katex.Module
	rerender := func() {
		if katexMod == nil {
			return
		}

		start, end := lbuf.Bounds()
		text := lbuf.Text(start, end, false)

		m, err := katexMod.Render(text, true)
		if err != nil {
			mathv.ShowError(err)
			return
		}
		mathv.ShowMathML(m)
	}

	lbuf.ConnectChanged(rerender)
	go func() {
		t := time.Now()
		m, err := katex.NewModule(ctx)
		log.Println("katex.NewModule took", time.Since(t))

		if err != nil {
			glib.IdleAdd(func() { loadable.SetError(err) })
			return
		}

		glib.IdleAdd(func() {
			katexMod = m
			loadable.SetChild(maths)
			rerender()
		})
	}()

	grid := gtk.NewGrid()
	grid.Attach(latexs, 0, 0, 1, 1)
	grid.Attach(loadable, 1, 0, 1, 1)

	win := app.NewWindow()
	win.SetTitle("Scratch")
	win.SetChild(grid)
	win.Show()
}
