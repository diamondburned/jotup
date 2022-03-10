// Package editor implements a Markdown editor with preview.
package editor

import (
	"context"
	"sync"

	"github.com/diamondburned/adaptive"
	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
)

// TODO: remove use of LoadablePage

// View is a Markdown editor sandwiched with a Markdown preview.
type View struct {
	*adaptive.LoadablePage
	Box struct {
		*gtk.Box
		View    *gtksource.View
		Preview *gtk.Box // TODO
	}

	ctx context.Context

	buffer *gtksource.Buffer
	file   *fileState
}

var (
	markdownLanguage *gtksource.Language
	languageOnce     sync.Once
)

// NewView creates a new View.
func NewView(ctx context.Context) *View {
	languageOnce.Do(func() {
		langman := gtksource.LanguageManagerGetDefault()
		markdownLanguage = langman.Language("markdown")
	})

	v := View{}
	v.buffer = gtksource.NewBufferWithLanguage(markdownLanguage)

	v.Box.View = gtksource.NewViewWithBuffer(v.buffer)
	v.Box.Preview = gtk.NewBox(gtk.OrientationHorizontal, 0)

	v.Box.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Box.AddCSSClass("editor-viewbox")

	v.LoadablePage = adaptive.NewLoadablePage()
	v.LoadablePage.SetChild(v.Box)

	return &v
}

// SetOrientation sets the View's orientation, which determines whether the
// preview is at the right or at the bottom.
func (v *View) SetOrientation(orientation gtk.Orientation) {
	v.Box.SetOrientation(orientation)
}

// LoadPath asynchronously loads the file at path into the buffer.
func (v *View) LoadPath(path string) {
	v.Load(gio.NewFileForPath(path))
}

// Load asynchronously loads the gio.File into the buffer.
func (v *View) Load(file gio.Filer) {
	v.Box.SetSensitive(false)
	v.file.Load(v.ctx, file, func(err error) {
		v.Box.SetSensitive(true)
		v.LoadablePage.SetError(err)
	})
}

// Save asynchronously saves the file. The given callback is invoked when it's
// done.
func (v *View) Save(done func(error)) {
	// TODO: add spinner
	v.file.Save(v.ctx, func(err error) {
		done(err)
	})
}
