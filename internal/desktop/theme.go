package desktop

import (
	"github.com/egoist/mygo/ui"
)

// The interface's fonts, embedded in fonts.go: Inter for the interface and
// IBM Plex Mono for URLs, paths and headers.
const (
	fontUI   = "Inter"
	fontMono = "IBM Plex Mono"
)

// The weights of Inter the interface uses: regular text and labels, and
// the medium of titles and buttons.
const (
	weightRegular = 440
	weightMedium  = 530
)

// palette holds the colors of the interface, as Hex's OpenCode theme has
// them: a deep window under a panel of the base color, layers of gray for
// what is chosen and hovered, text and icons in three strengths, and the
// accent, a blue, only on controls that are on.
type palette struct {
	dark bool

	// deep fills the window and the sidebar; base the panel, the cards
	// and the fields.
	deep, base ui.Color
	// layer1 … layer4 are the grays over base, from the faintest.
	layer1, layer2, layer3, layer4 ui.Color

	// borderWeak outlines the panel and the cards; muted divides rows;
	// base and strong ring controls.
	borderWeak, borderMuted, borderBase, borderStrong ui.Color

	text, textMuted, textFaint ui.Color
	icon, iconMuted, iconFaint ui.Color

	// hover and pressed lie over what the pointer is on.
	hover, pressed ui.Color

	accent, accentText, focus ui.Color

	danger, dangerSoft, success, successSoft, warning ui.Color

	// button is the face of a secondary button, buttonHover and
	// buttonPressed as the pointer is on it; contrast that of the primary
	// one, with contrastText on it.
	button, buttonHover, buttonPressed ui.Color
	contrast, contrastText             ui.Color
	// field is the face of text fields and check boxes.
	field ui.Color

	// tooltip is the face of tooltips and menus.
	tooltip ui.Color
	// segmentTrack is the track of a segmented control, segmentFace the
	// face of its chosen segment.
	segmentTrack, segmentFace ui.Color

	// shadowRaised is the shadow of cards, shadowControl of buttons and
	// fields, shadowFloating of toasts; shadowPill is the shade under the
	// chosen segment of a segmented control.
	shadowRaised, shadowControl, shadowFloating []shadow
	shadowPill                                  ui.Color
}

// shadow is a box shadow, as Element.Shadow takes it.
type shadow struct {
	x, y, blur, spread float32
	color              ui.Color
}

var lightPalette = palette{
	deep:   ui.Hex("#fafafa"),
	base:   ui.Hex("#ffffff"),
	layer1: ui.Hex("#fafafa"),
	layer2: ui.Hex("#f2f2f2"),
	layer3: ui.Hex("#eeeeee"),
	layer4: ui.Hex("#dbdbdb"),

	borderWeak:   ui.Hex("#dbdbdb"),
	borderMuted:  ui.RGBA(0, 0, 0, 0.08),
	borderBase:   ui.RGBA(0, 0, 0, 0.1),
	borderStrong: ui.RGBA(0, 0, 0, 0.2),

	text:      ui.Hex("#161616"),
	textMuted: ui.Hex("#5c5c5c"),
	textFaint: ui.Hex("#808080"),
	icon:      ui.Hex("#3a3a3a"),
	iconMuted: ui.Hex("#808080"),
	iconFaint: ui.Hex("#aeaeae"),

	hover:   ui.RGBA(0, 0, 0, 0.04),
	pressed: ui.RGBA(0, 0, 0, 0.08),

	accent:     ui.Hex("#3b5cf6"),
	accentText: ui.Hex("#3b5cf6"),
	focus:      ui.Hex("#7698fd"),

	danger:      ui.Hex("#b82d35"),
	dangerSoft:  ui.Hex("#fcebeb"),
	success:     ui.Hex("#198b43"),
	successSoft: ui.Hex("#e7f9ea"),
	warning:     ui.Hex("#a15c00"),

	button:        ui.Hex("#ffffff"),
	buttonHover:   ui.Hex("#fafafa"),
	buttonPressed: ui.Hex("#f2f2f2"),
	contrast:      ui.Hex("#242424"),
	contrastText:  ui.Hex("#ffffff"),
	field:         ui.Hex("#ffffff"),
	tooltip:       ui.Hex("#242424"),
	segmentTrack:  ui.Hex("#f2f2f2"),
	segmentFace:   ui.Hex("#ffffff"),

	shadowRaised: []shadow{
		{0, 2, 4, 0, ui.RGBA(0, 0, 0, 0.04)},
		{0, 1, 2, -1, ui.RGBA(0, 0, 0, 0.08)},
	},
	shadowControl: []shadow{
		{0, 1, 1.5, 0, ui.RGBA(0, 0, 0, 0.1)},
		{0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.14)},
	},
	shadowFloating: []shadow{
		{0, 8, 16, 0, ui.RGBA(0, 0, 0, 0.04)},
		{0, 4, 8, 0, ui.RGBA(0, 0, 0, 0.08)},
		{0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.12)},
	},
	shadowPill: ui.RGBA(0, 0, 0, 0.08),
}

var darkPalette = palette{
	dark:   true,
	deep:   ui.Hex("#080808"),
	base:   ui.Hex("#161616"),
	layer1: ui.Hex("#242424"),
	layer2: ui.Hex("#2e2e2e"),
	layer3: ui.Hex("#3a3a3a"),
	layer4: ui.Hex("#5c5c5c"),

	borderWeak:   ui.Hex("#282828"),
	borderMuted:  ui.RGBA(255, 255, 255, 0.08),
	borderBase:   ui.RGBA(255, 255, 255, 0.1),
	borderStrong: ui.RGBA(255, 255, 255, 0.2),

	text:      ui.Hex("#fafafa"),
	textMuted: ui.Hex("#aeaeae"),
	textFaint: ui.Hex("#808080"),
	icon:      ui.Hex("#dbdbdb"),
	iconMuted: ui.Hex("#808080"),
	iconFaint: ui.Hex("#5c5c5c"),

	hover:   ui.RGBA(255, 255, 255, 0.06),
	pressed: ui.RGBA(255, 255, 255, 0.1),

	accent:     ui.Hex("#3b5cf6"),
	accentText: ui.Hex("#a2bcff"),
	focus:      ui.Hex("#7698fd"),

	danger:      ui.Hex("#f17471"),
	dangerSoft:  ui.Hex("#461516"),
	success:     ui.Hex("#6bd586"),
	successSoft: ui.Hex("#14361d"),
	warning:     ui.Hex("#f0b44c"),

	button:        ui.RGBA(255, 255, 255, 0.06),
	buttonHover:   ui.RGBA(255, 255, 255, 0.1),
	buttonPressed: ui.RGBA(255, 255, 255, 0.14),
	contrast:      ui.Hex("#ededed"),
	contrastText:  ui.Hex("#161616"),
	field:         ui.RGBA(255, 255, 255, 0.03),
	tooltip:       ui.Hex("#2e2e2e"),
	segmentTrack:  ui.Hex("#242424"),
	segmentFace:   ui.Hex("#3a3a3a"),

	shadowRaised: []shadow{
		{0, 2, 4, 0, ui.RGBA(0, 0, 0, 0.3)},
		{0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.3)},
	},
	shadowControl: []shadow{
		{0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.4)},
		{0, 0, 0, 0.5, ui.RGBA(255, 255, 255, 0.2)},
	},
	shadowFloating: []shadow{
		{0, 8, 16, 0, ui.RGBA(0, 0, 0, 0.4)},
		{0, 4, 8, 0, ui.RGBA(0, 0, 0, 0.3)},
		{0, 0, 0, 0.5, ui.RGBA(255, 255, 255, 0.16)},
	},
	shadowPill: ui.RGBA(0, 0, 0, 0.4),
}

// paletteFor returns the palette of an appearance.
func paletteFor(dark bool) *palette {
	p := &lightPalette
	if dark {
		p = &darkPalette
	}
	return p
}

// applyPalette dresses the theme of the frame, for the widgets of package
// ui, in the palette's colors and Inter.
func applyPalette(t *ui.Theme, p *palette) {
	t.Dark = p.dark
	t.Background = p.deep
	t.Surface = p.button
	t.SurfaceHover = p.hover
	t.SurfacePressed = p.pressed
	t.Border = p.borderBase
	t.Text = p.text
	t.TextMuted = p.textMuted
	t.Accent = p.accent
	t.AccentHover = p.accent.Mix(ui.RGB(0, 0, 0), 0.08)
	t.AccentPressed = p.accent.Mix(ui.RGB(0, 0, 0), 0.16)
	t.AccentText = ui.RGB(255, 255, 255)
	// The accent stays when the window is in the background, as Hex's.
	t.InactiveAccent = p.accent
	t.Danger = p.danger
	t.Warning = p.warning
	t.Success = p.success
	t.Selection = p.accent.Alpha(0.25)
	t.Focus = p.focus
	t.FocusRingWidth = 1
	t.FocusRingOffset = 0
	t.Inverse, t.InverseText = p.tooltip, ui.RGB(255, 255, 255)
	t.Scrollbar = p.text.Alpha(0.25)
	t.Radius = 6
	t.Font = fontUI
	t.FontSize = 13
}

func shadows(e ui.Element, list []shadow) ui.Element {
	for _, s := range list {
		e.Shadow(s.x, s.y, s.blur, s.spread, s.color)
	}
	return e
}

// card gives an element the face of a card: the base color, rounded and
// outlined, on a soft shadow.
func (p *palette) card(e ui.Element) ui.Element {
	return shadows(e.Radius(8).Background(p.base).Border(1, p.borderBase), p.shadowRaised)
}

// control gives a button or a field its shadow and hairline ring.
func (p *palette) control(e ui.Element) ui.Element {
	return shadows(e, p.shadowControl)
}
