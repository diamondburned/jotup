package blocks

import (
	"context"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/jotup/internal/jotup/editor/md"
	"github.com/diamondburned/jotup/internal/jotup/editor/md/gtkmd"
	"github.com/yuin/goldmark/ast"
)

var textCSS = cssutil.Applier("gmd-text", `
	textview.gmd-text,
	textview.gmd-text text {
		background-color: transparent;
		color: @theme_fg_color;
	}
`)

// NewDefaultTextView creates a new TextView that TextBlock uses.
func NewDefaultTextView(buf *gtk.TextBuffer) *gtk.TextView {
	tview := gtk.NewTextViewWithBuffer(buf)
	tview.SetEditable(false)
	tview.SetCursorVisible(false)
	tview.SetVExpand(true)
	tview.SetHExpand(true)
	tview.SetWrapMode(gtk.WrapWordChar)

	textCSS(tview)
	md.SetTabSize(tview)

	return tview
}

type TextBlock struct {
	*gtk.TextView
	buf  *gtk.TextBuffer
	iter *gtk.TextIter
	view *MarkdownViewer
	ctx  context.Context
}

var _ gtkmd.TextWidgetChild = (*TextBlock)(nil)

// NewTextBlock creates a new TextBlock. Everything within state's ast.Node will
// be walked through.
func NewTextBlock(ctx context.Context, state *ContainerState) *TextBlock {
	text := TextBlock{
		view: state.Viewer,
		buf:  gtk.NewTextBuffer(state.Viewer.TagTable()),
		ctx:  ctx,
	}

	text.iter = text.buf.StartIter()
	text.TextView = NewDefaultTextView(text.buf)

	text.buf.SetEnableUndo(false)
	text.AddCSSClass("gmd-textblock")

	text.walk()
	return &text
}

// walk walks the given ast.Node recursively.
func (b *TextBlock) walk(node ast.Node) ast.WalkStatus {
	panic("implement me")
}

// ConnectLinkHandler connects the hyperlink handler into the TextBlock. Call
// this method if the TextBlock has a link. Only the first call will bind the
// handler.
func (b *TextBlock) ConnectLinkHandler() {
	BindLinkHandler(b.TextView, func(url string) { app.OpenURI(b.ctx, url) })
}

// Iter returns the internal TextBlock's iterator. The user must use this for
// any mutable operation, as most of TextBlock's methods will also use this
// iterator. Not doing so will result in undefined behavior.
func (b *TextBlock) Iter() *gtk.TextIter {
	return b.iter
}

// TrailingNewLines counts the number of trailing new lines up to 2.
func (b *TextBlock) TrailingNewLines() int {
	if !b.IsNewLine() {
		return 0
	}

	seeker := b.iter.Copy()

	for i := 0; i < 2; i++ {
		if !seeker.BackwardChar() || rune(seeker.Char()) != '\n' {
			return i
		}
	}

	return 2
}

// IsNewLine returns true if the iterator is currently on a new line.
func (b *TextBlock) IsNewLine() bool {
	if !b.iter.BackwardChar() {
		// empty buffer, so consider yes
		return true
	}

	// take the character, then undo the backward immediately
	char := rune(b.iter.Char())
	b.iter.ForwardChar()

	return char == '\n'
}

// EndLine ensures that the given amount of new lines will be put before the
// iterator. It accounts for existing new lines in the buffer.
func (b *TextBlock) EndLine(amount int) {
	b.InsertNewLines(amount - b.TrailingNewLines())
}

// InsertNewLines inserts n new lines without checking for existing new lines.
// Most users should use EndLine instead. If n < 1, then no insertion is done.
func (b *TextBlock) InsertNewLines(n int) {
	if n < 1 {
		return
	}
	b.buf.Insert(b.iter, strings.Repeat("\n", n))
}

// Tag gets an existing tag or creates a new empty one with the given name.
func (b *TextBlock) Tag(tagName string) *gtk.TextTag {
	return emptyTag(b.view.TagTable(), tagName)
}

func emptyTag(table *gtk.TextTagTable, tagName string) *gtk.TextTag {
	if tag := table.Lookup(tagName); tag != nil {
		return tag
	}

	tag := gtk.NewTextTag(tagName)
	if !table.Add(tag) {
		log.Panicf("failed to add new tag %q", tagName)
	}

	return tag
}

// HTMLTag returns a tag from the md.HTMLTags table. One is added if it's not
// already in the shared TagsTable.
func (b *TextBlock) HTMLTag(tagName string) *gtk.TextTag {
	return md.HTMLTags.FromTable(b.view.TagTable(), tagName)
}

// tagNameBounded wraps around tagBounded.
func (b *TextBlock) tagNameBounded(tagName string, f func()) {
	b.tagBounded(b.HTMLTag(tagName), f)
}

// tagBounded saves the current offset and calls f, expecting the function to
// use s.iter. Then, the tag with the given name is applied on top.
func (b *TextBlock) tagBounded(tag *gtk.TextTag, f func()) {
	start := b.iter.Offset()
	f()
	startIter := b.buf.IterAtOffset(start)
	b.buf.ApplyTag(tag, startIter, b.iter)
}
