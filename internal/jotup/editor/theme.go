package editor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/diamondburned/gotk4-sourceview/pkg/gtksource/v5"
	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotkit/app/prefs"
	"github.com/diamondburned/gotkit/gtkutil/textutil"
)

func schemeIsDark(scheme *gtksource.StyleScheme) bool {
	style := scheme.Style("background-pattern")
	if style == nil {
		return false
	}

	// Create a new TextTag to scrape the background value from.
	tag := gtk.NewTextTag("")
	style.Apply(tag)

	bg, _ := tag.ObjectProperty("background-rgba").(*gdk.RGBA)
	if bg == nil {
		return false
	}

	return textutil.ColorIsDark(
		float64(bg.Red()),
		float64(bg.Green()),
		float64(bg.Blue()),
	)
}

type schemeProp struct {
	prefs.Pubsub
	prefs.PropMeta
	val *gtksource.StyleScheme
	mut sync.Mutex
}

var scheme = &schemeProp{
	Pubsub: *prefs.NewPubsub(),
	PropMeta: prefs.PropMeta{
		Name:        "Highlight Style Scheme",
		Section:     "Editor",
		Description: "The name of the style scheme for syntax highlighting.",
	},
}

func init() {
	prefs.RegisterProp(scheme)
}

// Publish publishes the (nilable) scheme.
func (s *schemeProp) Publish(scheme *gtksource.StyleScheme) {
	s.mut.Lock()
	s.val = scheme
	s.mut.Unlock()

	s.Pubsub.Publish()
}

// Value returns the StyleScheme or the classic theme.
func (s *schemeProp) Value() *gtksource.StyleScheme {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.val
}

// GuessValue is like Value, except the defaults are accounted for.
func (s *schemeProp) GuessValue(w gtk.Widgetter) *gtksource.StyleScheme {
	if val := s.Value(); val != nil {
		return val
	}

	manager := gtksource.StyleSchemeManagerGetDefault()
	if textutil.IsDarkTheme(w) {
		return manager.Scheme("Adwaita-dark")
	} else {
		return manager.Scheme("Adwaita")
	}
}

func (s *schemeProp) MarshalJSON() ([]byte, error) {
	val := s.Value()
	if val == nil {
		return []byte("null"), nil
	}
	return json.Marshal(val.ID())
}

func (s *schemeProp) UnmarshalJSON(blob []byte) error {
	if string(blob) == "null" {
		s.Publish(nil)
		return nil
	}

	var id string
	if err := json.Unmarshal(blob, &id); err != nil {
		return err
	}

	manager := gtksource.StyleSchemeManagerGetDefault()
	sscheme := manager.Scheme(id)
	if sscheme == nil {
		return fmt.Errorf("unknown scheme %q", id)
	}

	s.Publish(sscheme)
	return nil
}

// CreateWidget creates either a *gtk.Entry or a *gtk.TextView.
func (s *schemeProp) CreateWidget(ctx context.Context, save func()) gtk.Widgetter {
	chooser := gtksource.NewStyleSchemeChooserButton()
	chooser.AddCSSClass("prefui-prop")
	chooser.AddCSSClass("prefui-prop-stylescheme")

	s.SubscribeWidget(chooser, func() {
		if style := s.GuessValue(chooser); style != nil {
			chooser.SetStyleScheme(style)
		}
	})
	chooser.NotifyProperty("style-scheme", func() {
		s.Publish(chooser.StyleScheme())
		save()
	})

	return chooser
}

// WidgetIsLarge returns false.
func (s *schemeProp) WidgetIsLarge() bool { return false }
