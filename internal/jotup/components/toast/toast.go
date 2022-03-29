// Package toast supplies widgets that mimic toast notifications displayed
// within the application.
package toast

import (
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

// Toast is a proper toast notification. It is implemented as a Bar with
// additional styling.
type Toast struct {
	*Bar
}

var toastCSS = cssutil.Applier("toast-toast", `
	.toast-toast {
		margin: 0;
	}
	.toast-toast > revealer > box {
		margin: 8px;
		border: none;
		box-shadow: none;
		border-radius: 999px;
		background-color: alpha(black, 0.7);
	}
	.toast-toast * {
		color: white;
	}
	.toast-toast .toast-text {
		margin-left: 12px;
	}
	.toast-toast button {
		border: none;
		margin: 2px 0;
		min-height: 30px;
		background: rgba(255, 255, 255, 0.15);
	}
	.toast-toast button.text-button {
		padding: 0 6px;
	}
	.toast-toast button.close {
		padding: 0;
		margin:  2px;
		background: none;
		min-width:  30px;
		min-height: 30px;
	}
	.toast-toast button:hover {
		background: rgba(255, 255, 255, 0.25);
	}
	.toast-toast button:active {
		background: rgba(255, 255, 255, 0.45);
	}
`)

// NewToast creates a new Toast.
func NewToast(pack gtk.PackType) *Toast {
	t := &Toast{Bar: NewBar(pack)}
	t.SetHExpand(false)
	t.SetHAlign(gtk.AlignCenter)
	toastCSS(t)
	return t
}
