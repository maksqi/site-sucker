package desktop

import (
	"github.com/egoist/mygo/ui"
)

// fontMono is the system's monospaced font, SF Mono on macOS, for URLs,
// paths and headers. The rest of the interface is in the system's font.
const fontMono = "monospace"

// palette holds macOS's semantic colors, as AppKit gives them on macOS 27:
// the window, the fills of grouped forms and controls, the labels in four
// strengths, and the system colors. The accent is always the system's
// blue, whatever the accent chosen in System Settings, and no color is
// orange: warnings are yellow.
type palette struct {
	dark bool

	window, field, raised ui.Color
	separator             ui.Color
	shadow                ui.Color
	// tooltip is the face of tooltips, light in light mode as AppKit's.
	tooltip ui.Color

	// label, label2, label3 and label4 are AppKit's labelColor, then the
	// secondary, tertiary and quaternary ones.
	label, label2, label3, label4 ui.Color
	// fill … fill4 are systemFill and the secondary, tertiary and
	// quaternary ones; group is the face of a grouped form's section.
	fill, fill2, fill3, fill4, group ui.Color

	accent, accentText, accentSoft, accentRing ui.Color

	blue, green, red, yellow, pink, indigo, purple, cyan, gray ui.Color
}

var lightPalette = palette{
	window:    ui.Hex("#ffffff"),
	field:     ui.Hex("#ffffff"),
	raised:    ui.Hex("#ffffff"),
	separator: ui.RGBA(0, 0, 0, 0.1),
	shadow:    ui.RGBA(0, 0, 0, 0.12),
	tooltip:   ui.Hex("#f4f4f4"),

	label:  ui.RGBA(0, 0, 0, 0.85),
	label2: ui.RGBA(0, 0, 0, 0.5),
	label3: ui.RGBA(0, 0, 0, 0.26),
	label4: ui.RGBA(0, 0, 0, 0.1),

	fill:  ui.RGBA(0, 0, 0, 0.098),
	fill2: ui.RGBA(0, 0, 0, 0.078),
	fill3: ui.RGBA(0, 0, 0, 0.047),
	fill4: ui.RGBA(0, 0, 0, 0.027),
	group: ui.RGBA(0, 0, 0, 0.04),

	blue:   ui.Hex("#0088ff"),
	green:  ui.Hex("#34c759"),
	red:    ui.Hex("#ff383c"),
	yellow: ui.Hex("#ffcc00"),
	pink:   ui.Hex("#ff2d55"),
	indigo: ui.Hex("#6155f5"),
	purple: ui.Hex("#cb30e0"),
	cyan:   ui.Hex("#00c0e8"),
	gray:   ui.Hex("#8e8e93"),
}

var darkPalette = palette{
	dark:      true,
	window:    ui.Hex("#1e1e1e"),
	field:     ui.RGBA(255, 255, 255, 0.05),
	raised:    ui.RGBA(255, 255, 255, 0.2),
	separator: ui.RGBA(255, 255, 255, 0.1),
	shadow:    ui.RGBA(0, 0, 0, 0.4),
	tooltip:   ui.Hex("#323232"),

	label:  ui.RGBA(255, 255, 255, 0.85),
	label2: ui.RGBA(255, 255, 255, 0.55),
	label3: ui.RGBA(255, 255, 255, 0.25),
	label4: ui.RGBA(255, 255, 255, 0.1),

	fill:  ui.RGBA(255, 255, 255, 0.098),
	fill2: ui.RGBA(255, 255, 255, 0.078),
	fill3: ui.RGBA(255, 255, 255, 0.047),
	fill4: ui.RGBA(255, 255, 255, 0.027),
	group: ui.RGBA(255, 255, 255, 0.05),

	blue:   ui.Hex("#0091ff"),
	green:  ui.Hex("#30d158"),
	red:    ui.Hex("#ff4245"),
	yellow: ui.Hex("#ffd600"),
	pink:   ui.Hex("#ff375f"),
	indigo: ui.Hex("#6d7cff"),
	purple: ui.Hex("#db34f2"),
	cyan:   ui.Hex("#3cd3fe"),
	gray:   ui.Hex("#98989d"),
}

// paletteFor returns the palette of an appearance.
func paletteFor(dark bool) *palette {
	p := &lightPalette
	if dark {
		p = &darkPalette
	}
	return p
}

func init() {
	for _, p := range []*palette{&lightPalette, &darkPalette} {
		p.accent = p.blue
		p.accentText = ui.RGB(255, 255, 255)
		p.accentSoft = p.blue.Alpha(0.15)
		p.accentRing = p.blue.Alpha(0.5)
	}
}

// applyPalette dresses the theme of the frame, for the widgets of package
// ui, in the palette's colors and the system's font.
func applyPalette(t *ui.Theme, p *palette) {
	t.Dark = p.dark
	t.Background = p.window
	t.Surface = p.fill3
	t.SurfaceHover = p.fill2
	t.SurfacePressed = p.fill
	t.Border = p.fill
	t.Text = p.label
	t.TextMuted = p.label2
	t.Accent = p.accent
	t.AccentHover = p.accent.Mix(ui.RGB(0, 0, 0), 0.08)
	t.AccentPressed = p.accent.Mix(ui.RGB(0, 0, 0), 0.18)
	t.AccentText = p.accentText
	t.Danger = p.red
	t.Warning = p.yellow
	t.Success = p.green
	t.Selection = p.accent.Alpha(0.3)
	t.Focus = p.accentRing
	t.Inverse, t.InverseText = p.tooltip, p.label.Alpha(1)
	t.Scrollbar = p.label.Alpha(0.3)
	t.Radius = 7
	t.Font = ""
	t.FontSize = 13
}

// groupBox gives an element the face of a section of a grouped form.
func (p *palette) groupBox(e ui.Element) ui.Element {
	return e.Radius(12).Background(p.group)
}

// ink is the label's color at alpha: black in light mode, white in dark,
// as AppKit's fills.
func (p *palette) ink(alpha float32) ui.Color {
	if p.dark {
		return ui.RGBA(255, 255, 255, alpha)
	}
	return ui.RGBA(0, 0, 0, alpha)
}
