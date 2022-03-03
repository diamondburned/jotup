package math

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

type errorView struct {
	*gtk.Label
}

var errorCSS = cssutil.Applier("math-error", `
	.math-error {
		border: 1px solid @error_color;
		color:  @error_color;
		font-family: serif;
	}
`)

func newErrorView() *errorView {
	v := errorView{}
	v.Label = gtk.NewLabel("")
	v.AddCSSClass("math-error")
	v.SetWrapMode(pango.WrapWordChar)
	v.SetMaxWidthChars(72)
	// Prevent Label from stretching parent container.
	v.SetLayoutManager(gtk.NewFixedLayout())
	return &v
}

func (v *errorView) SetError(err error) {
	v.SetText(err.Error())
}
