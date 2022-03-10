package gmd

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

const embeddedURLPrefix = "link:"

// BindLinkHandler binds input handlers for triggering hyperlinks within the
// TextView. If BindLinkHandler is called on the same TextView again, then it
// does nothing. The function checks this by checking for the .gmd-hyperlinked
// class.
func BindLinkHandler(tview *gtk.TextView, onURL func(string)) {
	if tview.HasCSSClass("gmd-hyperlinked") {
		return
	}
	tview.AddCSSClass("gmd-hyperlinked")

	linkTags := textutil.LinkTags()

	checkURL := func(x, y float64) *EmbeddedURL {
		bx, by := tview.WindowToBufferCoords(gtk.TextWindowWidget, int(x), int(y))
		it, ok := tview.IterAtLocation(bx, by)
		if !ok {
			return nil
		}

		for _, tags := range it.Tags() {
			tagName := tags.ObjectProperty("name").(string)

			if !strings.HasPrefix(tagName, embeddedURLPrefix) {
				continue
			}

			u, ok := ParseEmbeddedURL(strings.TrimPrefix(tagName, embeddedURLPrefix))
			if ok {
				return &u
			}
		}

		return nil
	}

	var buf *gtk.TextBuffer
	var table *gtk.TextTagTable
	var iters [2]*gtk.TextIter

	needIters := func() {
		if buf == nil {
			buf = tview.Buffer()
			table = buf.TagTable()
		}

		if iters == [2]*gtk.TextIter{} {
			i1 := buf.IterAtOffset(0)
			i2 := buf.IterAtOffset(0)
			iters = [2]*gtk.TextIter{i1, i2}
		}
	}

	click := gtk.NewGestureClick()
	click.SetButton(1)
	click.SetExclusive(true)
	click.ConnectAfter("pressed", func(nPress int, x, y float64) {
		if nPress != 1 {
			return
		}

		if u := checkURL(x, y); u != nil {
			onURL(u.URL)

			needIters()
			tag := linkTags.FromBuffer(buf, "a:visited")

			iters[0].SetOffset(u.From)
			iters[1].SetOffset(u.To)

			buf.ApplyTag(tag, iters[0], iters[1])
		}
	})

	var (
		lastURL *EmbeddedURL
		lastTag *gtk.TextTag
	)

	unhover := func() {
		if lastURL != nil {
			needIters()
			iters[0].SetOffset(lastURL.From)
			iters[1].SetOffset(lastURL.To)
			buf.RemoveTag(lastTag, iters[0], iters[1])

			lastURL = nil
			lastTag = nil
		}
	}

	motion := gtk.NewEventControllerMotion()
	motion.ConnectLeave(func() {
		unhover()
		iters = [2]*gtk.TextIter{}
	})
	motion.ConnectMotion(func(x, y float64) {
		u := checkURL(x, y)
		if u == lastURL {
			return
		}

		unhover()

		if u != nil {
			needIters()
			iters[0].SetOffset(u.From)
			iters[1].SetOffset(u.To)

			hover := linkTags.FromTable(table, "a:hover")
			buf.ApplyTag(hover, iters[0], iters[1])

			lastURL = u
			lastTag = hover
		}
	})

	tview.AddController(click)
	tview.AddController(motion)
}

// EmbeddedURL is a type that describes a URL and its bounds within a text
// buffer.
type EmbeddedURL struct {
	From int    `json:"1"`
	To   int    `json:"2"`
	URL  string `json:"u"`
}

func embedURL(x, y int, url string) string {
	b, err := json.Marshal(EmbeddedURL{x, y, url})
	if err != nil {
		log.Panicln("bug: error marshaling embeddedURL:", err)
	}

	return string(b)
}

// ParseEmbeddedURL parses the inlined data into an embedded URL structure.
func ParseEmbeddedURL(data string) (EmbeddedURL, bool) {
	var d EmbeddedURL
	err := json.Unmarshal([]byte(data), &d)
	return d, err == nil
}

// Bounds returns the bound iterators from the given text buffer.
func (e *EmbeddedURL) Bounds(buf *gtk.TextBuffer) (start, end *gtk.TextIter) {
	startIter := buf.IterAtOffset(e.From)
	endIter := buf.IterAtOffset(e.To)
	return startIter, endIter
}
