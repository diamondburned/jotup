package toast

import (
	"log"
	"time"

	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/gtkutil"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// Bar is a toast popup in the form of a bar. It is different from a regular
// toast notification, which typically doesn't stretch across the screen.
type Bar struct {
	*gtk.InfoBar
	Label *gtk.Label

	current currentToast
	timeout time.Duration
	log     bool
}

var infoBarCSS = cssutil.Applier("toast-infobar", `
	.toast-infobar > revealer > box {
		padding: 2px;
	}
	.toast-infobar button {
		padding: 2px;
	}
`)

type currentToast struct {
	actions []gtkutil.ActionData
	buttons []*gtk.Button
	handle  glib.SourceHandle
}

// NewBar creates a new Bar. If PackStart, then the Bar is shown on top.
// Otherwise, it's shown at the bottom.
func NewBar(pack gtk.PackType) *Bar {
	b := Bar{}
	b.Label = gtk.NewLabel("")
	b.Label.AddCSSClass("toast-text")
	b.Label.SetEllipsize(pango.EllipsizeEnd)
	b.Label.SetXAlign(0)

	b.InfoBar = gtk.NewInfoBar()
	b.InfoBar.SetShowCloseButton(true)
	b.InfoBar.SetRevealed(false)
	b.InfoBar.SetHExpand(true)
	b.InfoBar.AddChild(b.Label)
	b.InfoBar.ConnectResponse(func(resp int) {
		switch resp {
		case int(gtk.ResponseClose):
			b.dismiss()
		default:
			if 0 <= resp && resp < len(b.current.actions) {
				b.current.actions[resp].Func()
			}
		}
	})

	infoBarCSS(b.InfoBar)

	switch pack {
	case gtk.PackStart:
		b.InfoBar.SetVAlign(gtk.AlignStart)
	case gtk.PackEnd:
		b.InfoBar.SetVAlign(gtk.AlignEnd)
	}

	return &b
}

// SetTimeout sets the timeout that the bar should hide. If timeout is 0, then
// the bar won't automatically dismiss itself.
func (b *Bar) SetTimeout(timeout time.Duration) { b.timeout = timeout }

// SetLog sets whether messages put through Show should be logged down. This is
// useful if the bar is used for errors. In case the caller wants to
// conditionally use the logger, set this to false and manually invoke it.
func (b *Bar) SetLog(log bool) { b.log = log }

// Show shows the Bar, optionally displaying buttons at the end.
func (b *Bar) Show(text string, actions ...gtkutil.ActionData) {
	if b.log {
		log.Println("Toast:", text)
	}

	// Force dismiss to clear the previous toast if any.
	b.dismiss()

	b.current.actions = actions
	b.current.buttons = make([]*gtk.Button, len(actions))
	for i, action := range actions {
		b.current.buttons[i] = b.InfoBar.AddButton(action.Name, i)
	}

	if b.timeout > 0 {
		b.current.handle = glib.TimeoutAdd(uint(b.timeout/time.Millisecond), b.dismiss)
	}

	b.Label.SetLabel(text)
	b.Label.SetTooltipText(text)
	b.InfoBar.SetRevealed(true)
}

// Dismiss hides the bar.
func (b *Bar) Dismiss() {
	b.Response(int(gtk.ResponseClose))
}

func (b *Bar) dismiss() {
	// Undo all buttons if any.
	for _, button := range b.current.buttons {
		b.InfoBar.RemoveActionWidget(button)
	}

	// Remove the hidden handler if there's one.
	if b.current.handle > 0 {
		glib.SourceRemove(b.current.handle)
	}

	b.current = currentToast{}
	b.InfoBar.SetRevealed(false)
}
