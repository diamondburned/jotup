package editor

import (
	"context"

	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gio/v2"
)

type fileState struct {
	*gtksource.File
	loader *gtksource.FileLoader
	saver  *gtksource.FileSaver
}

func newFileState(buffer *gtksource.Buffer) *fileState {
	f := gtksource.NewFile()

	return &fileState{
		File:   f,
		loader: gtksource.NewFileLoader(buffer, f),
		saver:  gtksource.NewFileSaver(buffer, f),
	}
}

func (f *fileState) Save(ctx context.Context, done func(error)) {
	f.saver.SaveAsync(ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		done(f.saver.SaveFinish(result))
	})
}

func (f *fileState) Load(ctx context.Context, file gio.Filer, done func(error)) {
	f.SetLocation(file)
	f.loader.LoadAsync(ctx, int(glib.PriorityHigh), nil, func(result gio.AsyncResulter) {
		done(f.loader.LoadFinish(result))
	})
}
