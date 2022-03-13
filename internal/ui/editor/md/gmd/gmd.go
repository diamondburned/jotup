package gmd

import (
	"container/list"
	"context"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/core/glib"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
	"github.com/diamondburned/jotup/internal/ui/editor/md"
	"github.com/diamondburned/jotup/internal/ui/editor/md/hl"
	"github.com/yuin/goldmark/ast"
	"golang.org/x/net/html"
)

// WidgetChild describes the minimum interface of a child within the widget
// tree.
type WidgetChild interface {
	gtk.Widgetter
}

// TextWidgetChild is a WidgetChild that also embeds a TextBlock. All children
// that has appendable texts should implement this.
type TextWidgetChild interface {
	WidgetChild
	TextBlock() *TextBlock
}

// Renderers is a registry mapping known Markdown node kinds to rendering
// functins.
type Renderers map[ast.NodeKind]RenderFunc

// With creates a copy of r with other combined in.
func (r Renderers) With(other Renderers) Renderers {
	new := make(Renderers, len(r)+len(other))
	for k, fn := range r {
		new[k] = fn
	}
	for k, fn := range other {
		new[k] = fn
	}
	return new
}

// RenderFunc describes a function that renders a Markdown node into the given
// ContainerState and returns the newly created widget, which may contain
// additional ContainerStates recursed into the Node.
type RenderFunc func(context.Context, *ContainerState) (WidgetChild, ast.WalkStatus)

var defaultRenderers = make(Renderers)

// RegisterDefaultRenderer registers a default renderer for a Markdown node
// kind. If the function is called on an existing node, then it panics.
func RegisterDefaultRenderer(kind ast.NodeKind, f RenderFunc) {
	_, ok := defaultRenderers[kind]
	if ok {
		panic("duplicate handler for " + kind.String())
	}
	defaultRenderers[kind] = f
}

// MarkdownViewer is a widget that renders a Markdown AST node into widgets. All
// widgets within the viewer are strictly immutable.
type MarkdownViewer struct {
	*gtk.Box
	context context.Context
	table   *gtk.TextTagTable
	state   *ContainerState
}

// NewMarkdownViewer creates a new Markdown viewer.
func NewMarkdownViewer(ctx context.Context, r Renderers) *MarkdownViewer {
	v := MarkdownViewer{
		Box: gtk.NewBox(gtk.OrientationVertical, 0),
	}
}

// TagTable returns the viewer's shared TextTagTable.
func (v *MarkdownViewer) TagTable() *gtk.TextTagTable {
	return v.table
}

// SetNode sets the AST node to be shown in the Markdown viewer.
func (v *MarkdownViewer) SetNode(node ast.Node) {
	// v.diffNode(node)
	v.resetNode(node)
}

func (v *MarkdownViewer) diffNode(node ast.Node) {
	panic("implement me")
}

func (v *MarkdownViewer) resetNode(node ast.Node) {}

// ContainerState is the state of a single level of a Markdown node boxed inside
// a container of widgets.
type ContainerState struct {
	// Widgetter is the container widget holding added widgets. Its underlying
	// widget is a Box, but the user should make no assumption about that.
	gtk.Widgetter
	// Node is the current node that belongs to the ContainerState.
	Node ast.Node
	// Viewer is the top-level Markdown viewer. It is the same for all new
	// ContainerStates created underneath the same Viewer.
	Viewer *MarkdownViewer

	// internal state
	box     *gtk.Box
	list    *list.List
	current *list.Element
}

func newContainerBox() *gtk.Box {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	return box
}

// newContainerState is used internally.
func newContainerState(node ast.Node, viewer *MarkdownViewer) *ContainerState {
	s := ContainerState{Viewer: viewer}
	return s.Descend(node)
}

// Descend creates a new ContainerState instance that's derived from the current
// ContainerState. It has a completely new list of children widgets and a
// Markdown node, but the Viewer field is retained.
func (s *ContainerState) Descend(node ast.Node) *ContainerState {
	parent := newContainerBox()

	return &ContainerState{
		Widgetter: parent,
		Node:      node,
		Viewer:    s.Viewer,

		box:  parent,
		list: list.New(),
	}
}

// Current returns the current WidgetChild instance.
func (s *ContainerState) Current() WidgetChild {
	if s.current != nil {
		return s.current.Value.(WidgetChild)
	}
	return nil
}

// text returns the textBlock that is within any writable block.

// TextBlock returns either the current widget if it's a Text widget (*TextBlock
// or TextWidgetChild), or it creates a new *TextBlock.
func (s *ContainerState) TextBlock() *TextBlock {
	switch text := s.Current().(type) {
	case *TextBlock:
		return text
	case TextWidgetChild:
		return text.TextBlock()
	default:
		return s.paragraph()
	}
}

func (s *ContainerState) endLine(n *html.Node, amount int) {
	if amount < 1 {
		return
	}

	switch block := s.currentValue().(type) {
	case *TextBlock:
		block.endLine(n, amount)
	case *quoteBlock:
		block.state.endLine(n, amount)
	case *codeBlock:
		block.text.endLine(n, amount)
	default:
		s.finalizeBlock()
	}
}

// finalizeBlock finalizes the current block. Any later use of the state will
// create a new block.
func (s *ContainerState) finalizeBlock() {
	s.current = nil
}

func (s *ContainerState) paragraph() *TextBlock {
	if block, ok := s.currentValue().(*TextBlock); ok {
		return block
	}

	block := newTextBlock(s)

	s.current = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *ContainerState) code() *codeBlock {
	if block, ok := s.currentValue().(*codeBlock); ok {
		return block
	}

	block := newCodeBlock(s)

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *ContainerState) quote() *quoteBlock {
	if block, ok := s.current().(*quoteBlock); ok {
		return block
	}

	block := newQuoteBlock(s)

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

func (s *ContainerState) separator() *separatorBlock {
	if block, ok := s.current().(*separatorBlock); ok {
		return block
	}

	block := newSeparatorBlock()

	s.element = s.list.PushBack(block)
	s.parent.Append(block)

	return block
}

// TODO: turn quoteBlock into a Box, and implement descend+ascend for it.
// func (s *currentBlockState) descend() {}
// func (s *currentBlockState) ascend()  {}

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

// TrimmedText is a segment of a string with its surrounding spaces trimmed. The
// number of spaces trimmed are recorded into the Left and Right fields.
type TrimmedText struct {
	Text  string
	Left  int
	Right int
}

// TrimNewLines trims new lines surrounding str into TrimmedText.
func TrimNewLines(str string) TrimmedText {
	rhs := len(str) - len(strings.TrimRight(str, "\n"))
	str = strings.TrimRight(str, "\n")

	lhs := len(str) - len(strings.TrimLeft(str, "\n"))
	str = strings.TrimLeft(str, "\n")

	return TrimmedText{str, lhs, rhs}
}

type quoteBlock struct {
	*gtk.Box
	state *ContainerState
}

var quoteBlockCSS = cssutil.Applier("mcontent-quote-block", `
	.mcontent-quote-block {
		border-left:  3px solid alpha(@theme_fg_color, 0.5);
		padding-left: 5px;
	}
	.mcontent-quote-block:not(:last-child) {
		margin-bottom: 3px;
	}
	.mcontent-quote-block > textview.mauthor-haschip {
		margin-bottom: -1em;
	}
`)

func newQuoteBlock(s *ContainerState) *quoteBlock {
	box := gtk.NewBox(gtk.OrientationVertical, 0)
	box.SetOverflow(gtk.OverflowHidden)

	quote := quoteBlock{
		Box:   box,
		state: s.clone(box),
	}
	quoteBlockCSS(quote)
	return &quote
}

type codeBlock struct {
	*gtk.Overlay
	context context.Context

	scroll *gtk.ScrolledWindow
	lang   *gtk.Label
	text   *TextBlock
}

var codeBlockCSS = cssutil.Applier("mcontent-code-block", `
	.mcontent-code-block scrollbar {
		background: none;
		border:     none;
	}
	.mcontent-code-block:active scrollbar {
		opacity: 0.2;
	}
	.mcontent-code-block:not(.mcontent-code-block-expanded) scrollbar {
		opacity: 0;
	}
	.mcontent-code-block-text {
		font-family: monospace;
		padding: 4px 6px;
		padding-bottom: 0px; /* bottom-margin */
	}
	.mcontent-code-block-actions > *:not(label) {
		background-color: @theme_bg_color;
		margin-top:    4px;
		margin-right:  4px;
		margin-bottom: 4px;
	}
	.mcontent-code-block-language {
		font-family: monospace;
		font-size: 0.9em;
		margin: 0px 6px;
		color: mix(@theme_bg_color, @theme_fg_color, 0.85);
	}
	/*
	 * ease-in-out-gradient -steps 5 -min 0.2 -max 0 
	 * ease-in-out-gradient -steps 5 -min 0 -max 75 -f $'%.2fpx\n'
	 */
	.mcontent-code-block-voverflow .mcontent-code-block-cover {
		background-image: linear-gradient(
			to top,
			alpha(@theme_bg_color, 0.25) 0.00px,
			alpha(@theme_bg_color, 0.24) 2.40px,
			alpha(@theme_bg_color, 0.19) 19.20px,
			alpha(@theme_bg_color, 0.06) 55.80px,
			alpha(@theme_bg_color, 0.01) 72.60px
		);
	}
`)

var codeLowerHeight = prefs.NewInt(200, prefs.IntMeta{
	Name:    "Collapsed Codeblock Height",
	Section: "Text",
	Description: "The height of a collapsed codeblock." +
		" Long snippets of code will appear cropped.",
	Min: 50,
	Max: 5000,
})

var codeUpperHeight = prefs.NewInt(400, prefs.IntMeta{
	Name:    "Expanded Codeblock Height",
	Section: "Text",
	Description: "The height of an expanded codeblock." +
		" Codeblocks are either shorter than this or as tall." +
		" Ignored if this is lower than the collapsed height.",
	Min: 50,
	Max: 5000,
})

func init() { prefs.Order(codeLowerHeight, codeUpperHeight) }

func newCodeBlock(s *ContainerState) *codeBlock {
	text := newTextBlock(s)
	text.AddCSSClass("mcontent-code-block-text")
	text.SetWrapMode(gtk.WrapNone)
	text.SetVScrollPolicy(gtk.ScrollMinimum)
	text.SetBottomMargin(18)

	sw := gtk.NewScrolledWindow()
	sw.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
	sw.SetPropagateNaturalHeight(true)
	sw.SetChild(text)

	language := gtk.NewLabel("")
	language.AddCSSClass("mcontent-code-block-language")
	language.SetHExpand(true)
	language.SetEllipsize(pango.EllipsizeEnd)
	language.SetSingleLineMode(true)
	language.SetXAlign(0)
	language.SetVAlign(gtk.AlignCenter)

	wrap := gtk.NewToggleButton()
	wrap.SetIconName("format-justify-left-symbolic")
	wrap.SetTooltipText("Toggle Word Wrapping")

	copy := gtk.NewButtonFromIconName("edit-copy-symbolic")
	copy.SetTooltipText("Copy All")
	copy.ConnectClicked(func() {
		popover := gtk.NewPopover()
		popover.SetCanTarget(false)
		popover.SetAutohide(false)
		popover.SetChild(gtk.NewLabel("Copied!"))
		popover.SetPosition(gtk.PosLeft)
		popover.SetParent(copy)

		start, end := text.buf.Bounds()
		text := text.buf.Text(start, end, false)

		clipboard := gdk.DisplayGetDefault().Clipboard()
		clipboard.SetText(text)

		popover.Popup()
		glib.TimeoutSecondsAdd(3, func() {
			popover.Popdown()
			popover.Unparent()
		})
	})

	expand := gtk.NewToggleButton()
	expand.SetTooltipText("Toggle Reveal Code")

	actions := gtk.NewBox(gtk.OrientationHorizontal, 0)
	actions.AddCSSClass("mcontent-code-block-actions")
	actions.SetHAlign(gtk.AlignFill)
	actions.SetVAlign(gtk.AlignStart)
	actions.Append(language)
	actions.Append(wrap)
	actions.Append(copy)
	actions.Append(expand)

	clickOverlay := gtk.NewBox(gtk.OrientationVertical, 0)
	clickOverlay.Append(sw)
	// Clicking on the codeblock will click the button for us, but only if it's
	// collapsed.
	click := gtk.NewGestureClick()
	click.SetButton(gdk.BUTTON_PRIMARY)
	click.SetExclusive(true)
	click.ConnectPressed(func(n int, x, y float64) {
		// TODO: don't handle this on a touchscreen.
		if !expand.Active() {
			expand.Activate()
		}
	})
	clickOverlay.AddController(click)

	overlay := gtk.NewOverlay()
	overlay.SetOverflow(gtk.OverflowHidden)
	overlay.SetChild(clickOverlay)
	overlay.AddOverlay(actions)
	overlay.SetMeasureOverlay(actions, true)
	overlay.AddCSSClass("frame")
	codeBlockCSS(overlay)

	// Lazily initialized in notify::upper below.
	var cover *gtk.Box
	coverSetVisible := func(visible bool) {
		if cover != nil {
			cover.SetVisible(visible)
		}
	}

	// Manually keep track of the expanded height, since SetMaxContentHeight
	// doesn't work (below issue).
	var maxHeight int
	var minHeight int

	vadj := text.VAdjustment()

	toggleExpand := func() {
		if expand.Active() {
			overlay.AddCSSClass("mcontent-code-block-expanded")
			expand.SetIconName("view-restore-symbolic")
			sw.SetCanTarget(true)
			sw.SetSizeRequest(-1, maxHeight)
			sw.SetMarginTop(actions.AllocatedHeight())
			language.SetOpacity(1)
			coverSetVisible(false)
		} else {
			overlay.RemoveCSSClass("mcontent-code-block-expanded")
			expand.SetIconName("view-fullscreen-symbolic")
			sw.SetCanTarget(false)
			sw.SetSizeRequest(-1, minHeight)
			sw.SetMarginTop(0)
			language.SetOpacity(0)
			coverSetVisible(true)
			// Restore scrolling when uncollapsed.
			vadj.SetValue(0)
		}
	}
	expand.ConnectClicked(toggleExpand)

	// Workaround for issue https://gitlab.gnome.org/GNOME/gtk/-/issues/3515.
	vadj.NotifyProperty("upper", func() {
		upperHeight := codeUpperHeight.Value()
		lowerHeight := codeLowerHeight.Value()
		if upperHeight < lowerHeight {
			upperHeight = lowerHeight
		}

		upper := int(vadj.Upper())
		maxHeight = upper
		minHeight = upper

		if maxHeight > upperHeight {
			maxHeight = upperHeight
		}

		if minHeight > lowerHeight {
			minHeight = lowerHeight
			overlay.AddCSSClass("mcontent-code-block-voverflow")

			if cover == nil {
				// Use a fading gradient to let the user know (visually) that
				// there's still more code hidden. This isn't very accessible.
				cover = gtk.NewBox(gtk.OrientationHorizontal, 0)
				cover.AddCSSClass("mcontent-code-block-cover")
				cover.SetCanTarget(false)
				cover.SetVAlign(gtk.AlignFill)
				cover.SetHAlign(gtk.AlignFill)
				overlay.AddOverlay(cover)
			}
		}

		// Quite expensive when it's put here, but it's safer.
		toggleExpand()
	})

	wrap.ConnectClicked(func() {
		if wrap.Active() {
			text.SetWrapMode(gtk.WrapWordChar)
		} else {
			// TODO: this doesn't shrink back, which is weird.
			text.SetWrapMode(gtk.WrapNone)
		}
	})

	return &codeBlock{
		Overlay: overlay,
		context: s.context,
		scroll:  sw,
		lang:    language,
		text:    text,
	}
}

func (b *codeBlock) withHighlight(lang string, f func(*TextBlock)) {
	b.lang.SetText(lang)

	start := b.text.iter.Offset()
	f(b.text)

	startIter := b.text.buf.IterAtOffset(start)

	// Don't add any hyphens.
	noHyphens := md.TextTags.FromTable(b.text.table, "_nohyphens")
	b.text.buf.ApplyTag(noHyphens, startIter, b.text.iter)

	hl.Highlight(b.context, startIter, b.text.iter, lang)
}

type separatorBlock struct {
	*gtk.Separator
}

func newSeparatorBlock() *separatorBlock {
	sep := gtk.NewSeparator(gtk.OrientationHorizontal)
	sep.AddCSSClass("mcontent-separator-block")
	return &separatorBlock{sep}
}
