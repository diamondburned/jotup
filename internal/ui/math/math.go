package math

import (
	"log"
	"time"

	"github.com/diamondburned/gotk4-lasem/pkg/lasem"
	"github.com/diamondburned/gotk4/pkg/cairo"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil"
)

const emptyMathML = `<math xmlns="http://www.w3.org/1998/Math/MathML"></math>`

var emptyDOM *lasem.DOMDocument

func init() {
	d, err := lasem.NewDOMDocumentFromMemory(emptyMathML)
	if err != nil {
		panic("BUG: cannot init zero-state DOMDocument: " + err.Error())
	}
	emptyDOM = d
}

// MathView is a DrawingArea canvas that renders math.
type MathView struct {
	*gtk.Stack
	area  *gtk.DrawingArea
	error *errorView

	dom   *lasem.DOMDocument
	view  *lasem.DOMView
	color [4]float64 // RGBA
	size  [2]int
}

// NewMathView creates a new empty MathView.
func NewMathView() *MathView {
	v := MathView{}
	v.area = gtk.NewDrawingArea()
	v.area.AddCSSClass("math-canvas")
	v.area.SetDrawFunc(v.draw)

	v.error = newErrorView()

	v.Stack = gtk.NewStack()
	v.Stack.SetTransitionDuration(0)
	v.Stack.AddChild(v.error)
	v.Stack.AddChild(v.area)

	v.setDOMDocument(emptyDOM)

	gtkutil.OnFirstMap(v, func() {
		// Use the theme's foreground color.
		if fg, ok := v.StyleContext().LookupColor("theme_fg_color"); ok {
			v.color = [4]float64{
				float64(fg.Red()),
				float64(fg.Green()),
				float64(fg.Blue()),
				float64(fg.Alpha()),
			}
			v.QueueDraw()
		} else {
			log.Println("no theme_fg_color")
		}
	})

	return &v
}

// SetScale sets the scale (size) of the math canvas.
func (v *MathView) SetScale(scale float64) {
	v.view.SetResolution(98 * scale)
}

// ShowError shows an error on the MathView.
func (v *MathView) ShowError(err error) {
	v.error.SetError(err)
	v.Stack.SetVisibleChild(v.error)
}

// SetMathML sets the MathML data to render.
func (v *MathView) ShowMathML(data string) {
	t := time.Now()
	defer func() { log.Println("MathML DOM render took", time.Since(t)) }()

	d, err := lasem.NewDOMDocumentFromMemory(data)
	if err != nil {
		v.ShowError(err)
		return
	}

	v.setDOMDocument(d)
}

func (v *MathView) setDOMDocument(d *lasem.DOMDocument) {
	v.dom = d

	// Set the colors.
	if d != emptyDOM {
		mlElem, ok := d.DocumentElement().(*lasem.MathMLMathElement)
		if ok {
			style := mlElem.DefaultStyle()
			style.SetMathColor(v.color[0], v.color[1], v.color[2], v.color[3])
		}
	}

	if v.view == nil {
		v.view = lasem.BaseDOMView(d.CreateView())
		v.view.SetResolution(98)
	} else {
		v.view.SetDocument(d)
	}

	v.Stack.SetVisibleChild(v.area)
	v.area.QueueDraw()
}

func (v *MathView) draw(_ *gtk.DrawingArea, cr *cairo.Context, w, h int) {
	if v.size != [2]int{w, h} {
		v.size = [2]int{w, h}

		box := lasem.NewBox(0, 0, float64(w), float64(h))
		v.view.SetViewportPixels(&box)
	}

	v.view.Render(cr, 0, 0)
}
