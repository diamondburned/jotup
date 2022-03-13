// Package greet implements the first page that the user sees when Jotup is
// first opened. Its job is to prompt the user to open a folder.
package greet

import (
	"context"
	"fmt"
	"html"
	"sort"

	"github.com/diamondburned/gotk4/pkg/gdk/v4"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"
	"github.com/diamondburned/gotk4/pkg/pango"
	"github.com/diamondburned/gotkit/app"
	"github.com/diamondburned/gotkit/app/prefs/kvstate"
	"github.com/diamondburned/gotkit/gtkutil/cssutil"
)

var css = cssutil.Applier("greet", `
	.greet-icon {
	    margin-right: 24px;
	}
	.greet-title {
		font-size: 4em;
	    margin: calc(128px / 5) 0;
	}
	.greet-subtitle {
		font-size: 1.25em;
		margin: 0;
	    margin-bottom: 6px;
	}
	.greet-button {
		padding: 0;
	}
	.greet-button-title {
		/* font-size: 1.1em; */
		/* margin-right: 10px; */
	}
	.greet-button-subtitle {
		font-size: 0.9em;
	}
	.greet-button image {
		margin: 6px 10px;
	}
	.greet-blur {
		opacity: 0.75;
		font-size: 0.9em;
	}
	.greet-right > label {
		margin-left:  10px;
		margin-right: 10px;
	}
	.greet-subtitle:not(:nth-child(2)) {
		margin-top:    12px;
		margin-bottom: 10px;
	}
`)

// Button describes a big button.
type Button struct {
	Title string
	Icon  string
	Func  func()
}

func (b Button) widget() *gtk.Button {
	icon := gtk.NewImageFromIconName(b.Icon)
	icon.SetIconSize(gtk.IconSizeLarge)

	title := gtk.NewLabel(b.Title)
	title.SetXAlign(0)
	title.SetWrap(true)
	title.SetWrapMode(pango.WrapWordChar)
	title.AddCSSClass("greet-button-title")

	box := gtk.NewBox(gtk.OrientationHorizontal, 0)
	box.Append(icon)
	box.Append(title)

	btn := gtk.NewButton()
	btn.AddCSSClass("greet-button")
	btn.SetChild(box)
	btn.ConnectClicked(b.Func)

	return btn
}

const keepRecentPaths = 5

// View is the main greeting view.
type View struct {
	*gtk.CenterBox
	Inner *gtk.Box

	Icon  *gtk.Image
	Right struct {
		*gtk.Box // vertical
		Title    *gtk.Label
		Subtitle *gtk.Label
	}

	ctx       context.Context
	open      func(string)
	pathFreqs []pathFreq
}

type pathFreq struct {
	Path string `json:"path"`
	Freq int    `json:"freq"`
}

// NewView creates a new View.
func NewView(ctx context.Context, open func(string)) *View {
	v := View{
		ctx:  ctx,
		open: open,
	}

	v.Right.Title = gtk.NewLabel("Jotup")
	v.Right.Title.SetXAlign(0)
	v.Right.Title.AddCSSClass("greet-title")

	v.Right.Subtitle = gtk.NewLabel("Getting Started")
	v.Right.Subtitle.SetXAlign(0)
	v.Right.Subtitle.AddCSSClass("greet-subtitle")

	v.Right.Box = gtk.NewBox(gtk.OrientationVertical, 0)
	v.Right.Box.AddCSSClass("greet-right")
	v.Right.Box.SetHExpand(true)
	v.Right.Box.SetSizeRequest(250, -1)
	v.Right.Box.Append(v.Right.Title)

	button := Button{
		Title: "Open Folder...",
		Icon:  "folder-new-symbolic",
		Func:  v.promptOpenFolder,
	}
	v.Right.Box.Append(v.Right.Subtitle)
	v.Right.Box.Append(button.widget())

	recentHeader := gtk.NewLabel("Recents")
	recentHeader.SetXAlign(0)
	recentHeader.AddCSSClass("greet-subtitle")

	v.Right.Box.Append(recentHeader)
	if recentPaths := v.RecentPaths(); len(recentPaths) > 0 {
		for _, path := range v.RecentPaths() {
			path := path // copy for closure

			label := gtk.NewLabel("")
			label.SetMarkup(fmt.Sprintf(`<a href="#">%s</a>`, html.EscapeString(path)))
			label.SetXAlign(0)
			label.SetEllipsize(pango.EllipsizeMiddle)
			label.SetTooltipText(path)
			label.ConnectActivateLink(func(string) bool {
				v.markAccessPath(path)
				open(path)
				return true
			})
			v.Right.Box.Append(label)
		}
	} else {
		label := gtk.NewLabel("No recent paths.")
		label.AddCSSClass("greet-blur")
		label.SetXAlign(0)
		v.Right.Box.Append(label)
	}

	v.Icon = gtk.NewImageFromIconName("accessories-text-editor")
	v.Icon.SetPixelSize(128)
	v.Icon.SetVAlign(gtk.AlignStart)
	v.Icon.SetHAlign(gtk.AlignCenter)
	v.Icon.AddCSSClass("greet-icon")

	v.Inner = gtk.NewBox(gtk.OrientationHorizontal, 0)
	v.Inner.SetHAlign(gtk.AlignCenter)
	v.Inner.SetVAlign(gtk.AlignCenter)
	v.Inner.Append(v.Icon)
	v.Inner.Append(v.Right)

	v.CenterBox = gtk.NewCenterBox()
	v.CenterBox.SetCenterWidget(v.Inner)
	v.CenterBox.SetHExpand(true)
	v.CenterBox.SetVExpand(true)

	// Hack because GTK4's new APIs fucking suck, especially when you don't have
	// subclassing. Also, it's 2022 and there's still no way to clamp the
	// dimensions of a widget without importing a library that does terrible
	// things.
	hookToggler(v, 500, widthToggler{
		squeeze: func() {
			v.Inner.SetOrientation(gtk.OrientationVertical)
		},
		expand: func() {
			v.Inner.SetOrientation(gtk.OrientationHorizontal)
		},
	})

	css(v)
	return &v
}

func (v *View) promptOpenFolder() {
	chooser := gtk.NewFileChooserNative(
		"Open Folder", &app.WindowFromContext(v.ctx).Window,
		gtk.FileChooserActionSelectFolder, "Open", "Cancel",
	)

	chooser.SetSelectMultiple(false)
	chooser.ConnectResponse(func(resp int) {
		chooser.Destroy()
		if resp == int(gtk.ResponseAccept) {
			path := chooser.File().Path()
			v.markAccessPath(path)
			v.open(path)
		}
	})
	chooser.Show()
}

// RecentPaths returns the most recent paths.
func (v *View) RecentPaths() []string {
	if v.pathFreqs == nil {
		cfg := kvstate.AcquireConfig(v.ctx, "greet")
		cfg.Get("path_frequencies", &v.pathFreqs)
	}

	v.sortPathFreqs()

	paths := make([]string, 0, keepRecentPaths)
	for i := 0; i < len(v.pathFreqs) && i < keepRecentPaths; i++ {
		paths = append(paths, v.pathFreqs[i].Path)
	}

	return paths
}

// markAccessPath increments the access frequency of the given path.
func (v *View) markAccessPath(path string) {
	for i, pathFreq := range v.pathFreqs {
		if pathFreq.Path == path {
			v.pathFreqs[i].Freq = pathFreq.Freq + 1
			goto cleanup
		}
	}

	v.pathFreqs = append(v.pathFreqs, pathFreq{
		Path: path,
		Freq: 1,
	})

cleanup:
	v.sortPathFreqs()

	// This doesn't really leak much memory, so we're fine.
	if len(v.pathFreqs) > keepRecentPaths {
		v.pathFreqs = v.pathFreqs[:keepRecentPaths]
	}

	cfg := kvstate.AcquireConfig(v.ctx, "greet")
	cfg.Set("path_frequencies", v.pathFreqs)
}

func (v *View) sortPathFreqs() {
	sort.Slice(v.pathFreqs, func(i, j int) bool {
		pfi := v.pathFreqs[i]
		pfj := v.pathFreqs[j]
		if pfi.Freq == pfj.Freq {
			return pfi.Path < pfi.Path // A-Z
		} else {
			return pfi.Freq > pfj.Freq // highest first
		}
	})
}

type widthToggler struct {
	squeeze func()
	expand  func()
}

type widthState = uint8

const (
	widthSqueezed widthState = iota
	widthExpanded
)

func hookToggler(widget gtk.Widgetter, width int, toggler widthToggler) {
	var currState widthState

	base := gtk.BaseWidget(widget)
	base.AddTickCallback(func(gtk.Widgetter, gdk.FrameClocker) bool {
		state := widthSqueezed
		if base.AllocatedWidth() > width {
			state = widthExpanded
		}

		if currState != state {
			currState = state
			switch state {
			case widthExpanded:
				toggler.expand()
			case widthSqueezed:
				toggler.squeeze()
			}
		}

		return true
	})
}
