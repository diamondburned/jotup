package jotup

import (
	"context"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/jotup/internal/jotup/greet"
)

// GreeterPage is the main page with the big Jotup text.
type GreeterPage struct {
	*gtk.Box
	Header *gtk.HeaderBar
	Body   *greet.View
}

// NewGreeter creates a new Greeter page.
func NewGreeter(ctx context.Context, open func(string)) *GreeterPage {
	p := GreeterPage{}
	p.Body = greet.NewView(ctx, open)

	p.Header = gtk.NewHeaderBar()
	p.Header.SetShowTitleButtons(true)

	p.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	p.Box.SetHExpand(true)
	p.Box.SetVExpand(true)
	p.Box.Append(p.Header)
	p.Box.Append(p.Body)

	return &p
}

// PromptOpenFolder calls p.Body's. The greeter already implements code for
// asking the user to open a folder. We wired a callback from that to open().
func (p *GreeterPage) PromptOpenFolder() {
	p.Body.PromptOpenFolder()
}
