package desktop

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// controlHeight is the height of buttons, fields, pop-up buttons and
// segmented controls, as Hex's.
const controlHeight = 28

// controlMotion is how controls move: a spring that settles without
// overshooting, in 0.2 s.
var controlMotion = ui.Animation{Spring: ui.SpringOptions{Duration: 200 * time.Millisecond}}

// option is a choice of a setting: the value the config holds and what
// the window says of it.
type option struct {
	value, label, hint string
	icon               *ui.SVG
}

func labelOf(options []option, value string) string {
	if i := slices.IndexFunc(options, func(o option) bool { return o.value == value }); i >= 0 {
		return options[i].label
	}
	return value
}

// phase is how far now is into a repeating period, from 0 to 1.
func phase(now time.Time, period time.Duration) float32 {
	return float32(now.UnixNano()%int64(period)) / float32(period)
}

// pulse is how a status dot moves.
type pulse uint8

const (
	still  pulse = iota
	blink        // fades to 45% and back
	beacon       // sends out a ring
)

// dot draws a status dot, which animates without rebuilding the view.
func (a *app) dot(c *ui.Context, size float32, col ui.Color, mode pulse, period time.Duration) ui.Element {
	if c.Preferences().ReduceMotion {
		mode = still
	}
	return ui.Box(c).Size(size, size).Shrink(0).Draw(func(p *ui.Painter, r ui.Rect) {
		alpha := float32(1)
		switch mode {
		case blink:
			p.AnimationFrame()
			alpha = 1 - 0.55*(0.5-0.5*float32(math.Cos(2*math.Pi*float64(phase(p.Now(), period)))))
		case beacon:
			p.AnimationFrame()
			t := phase(p.Now(), period)
			t = 1 - (1-t)*(1-t)*(1-t)
			p.Shadow(r, r.W/2, 0, 0, 0, 5*t, col.Alpha(0.4*(1-t)))
		}
		p.Fill(r, col.Alpha(alpha), r.W/2)
	})
}

// buttonKind is the look of a button.
type buttonKind uint8

const (
	buttonSecondary buttonKind = iota // the neutral face with a hairline
	buttonPrimary                     // the contrast face, once a screen
	buttonGhost                       // no face until hovered
	buttonDanger                      // the neutral face, in red
)

type buttonKey struct {
	label string
	icon  *ui.SVG
}

// buttonBase is a push button whose content the caller builds. Hovering
// shows at once, without a fade, as Hex's.
func (a *app) buttonBase(c *ui.Context, key any, kind buttonKind, disabled bool) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(key)).Height(controlHeight).Padding(0, 11).Gap(6).Radius(6).
		Justify(ui.Center).Shrink(0).FontWeight(weightMedium).Disabled(disabled)
	hovered, pressed := b.Hovered() && !disabled, b.Pressed() && !disabled
	fg := p.text
	switch kind {
	case buttonPrimary:
		// The contrast face lightens under the pointer in a light window,
		// and darkens in a dark one, where it is light.
		toward := ui.RGB(255, 255, 255)
		if p.dark {
			toward = ui.RGB(0, 0, 0)
		}
		bg := p.contrast
		switch {
		case pressed:
			bg = bg.Mix(toward, 0.2)
		case hovered:
			bg = bg.Mix(toward, 0.1)
		}
		b.Background(bg)
		fg = p.contrastText
	case buttonGhost:
		fg = p.iconMuted
		switch {
		case pressed:
			b.Background(p.pressed)
			fg = p.icon
		case hovered:
			b.Background(p.hover)
			fg = p.icon
		}
	default:
		bg := p.button
		switch {
		case pressed:
			bg = p.buttonPressed
		case hovered:
			bg = p.buttonHover
		}
		p.control(b.Background(bg))
		if kind == buttonDanger {
			fg = p.danger
		}
	}
	if disabled {
		b.Opacity(0.45)
	}
	return b.TextColor(fg)
}

// button is a push button with an icon, a label or both.
func (a *app) button(c *ui.Context, kind buttonKind, ic *ui.SVG, label string, disabled bool) ui.Element {
	b := a.buttonBase(c, buttonKey{label, ic}, kind, disabled)
	if label == "" {
		b.Width(controlHeight).Padding(0)
	} else {
		b.Label(label)
	}
	return b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(16)
		}
		if label != "" {
			ui.Text(c, label).SingleLine()
		}
	})
}

// iconButton is a borderless button showing only an icon, named by tip.
func (a *app) iconButton(c *ui.Context, ic *ui.SVG, tip string, disabled bool) ui.Element {
	return a.button(c, buttonGhost, ic, "", disabled).Label(tip).Tooltip(tip)
}

// smallIconButton is the small borderless button inside fields and
// toasts.
func (a *app) smallIconButton(c *ui.Context, ic *ui.SVG, tip string, disabled bool) ui.Element {
	b := a.iconButton(c, ic, tip, disabled).Size(20, 20).Radius(4)
	return b
}

// The layout of a page of settings, as Hex's: a gutter of 32 that cards
// hang 16 past, so that their rows line up with the page's title.
const (
	pageGutter  = 16
	cardPadding = 16
)

// page lays out a pane of settings: its groups, scrolling under the
// panel's header.
func (a *app) page(c *ui.Context, body func()) {
	ui.Scroll(c).Fill().Padding(4, pageGutter, 40, pageGutter).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(880).Children(body)
	})
}

// group is a section of a page: its rows in a card under a title, with a
// footnote below.
func (a *app) group(c *ui.Context, title, footer string, body func()) {
	p := a.pal
	ui.Column(c).Margin(0, 0, 24, 0).Children(func() {
		if title != "" {
			ui.Text(c, title).FontWeight(weightMedium).LineHeight(16/13.0).TextColor(p.textMuted).
				Padding(0, cardPadding).Margin(0, 0, 8, 0)
		}
		p.card(ui.Column(c)).Padding(0, cardPadding).Children(func() {
			ui.Column(c).Gap(1).Dividers(1, p.borderMuted).Children(body)
		})
		if footer != "" {
			ui.Text(c, footer).FontSize(12).LineHeight(16/12.0).TextColor(p.textMuted).Padding(0, cardPadding).Margin(8, 0, 0, 0)
		}
	})
}

func (a *app) rowText(c *ui.Context, label, hint string, dim bool) ui.Element {
	p := a.pal
	col := ui.Column(c).Gap(2).MinWidth(0).Children(func() {
		ui.Text(c, label).LineHeight(20 / 13.0)
		if hint != "" {
			ui.Text(c, hint).FontSize(12).LineHeight(16 / 12.0).TextColor(p.textMuted)
		}
	})
	if dim {
		col.Opacity(0.45)
	}
	return col
}

// row is a setting: its label and hint on the left, its control on the
// right.
func (a *app) row(c *ui.Context, label, hint string, dim bool, control func()) {
	ui.Row(c).MinHeight(48).Padding(8, 0).Gap(24).Children(func() {
		a.rowText(c, label, hint, dim).Grow(1).Basis(0)
		ui.Row(c).Gap(8).Justify(ui.End).Shrink(0).MaxWidthPercent(62).Children(control)
	})
}

// stacked is a setting whose control takes the whole width, below its
// label.
func (a *app) stacked(c *ui.Context, label, hint string, dim bool, control func()) {
	ui.Column(c).Gap(8).Padding(12, 0, 14, 0).Children(func() {
		a.rowText(c, label, hint, dim)
		ui.Row(c).Gap(8).Children(control)
	})
}

// callout is a note below the groups, or a warning.
func (a *app) callout(c *ui.Context, warning bool, text string) {
	p := a.pal
	col, ic := p.iconMuted, iconInfo
	if warning {
		col, ic = p.warning, iconWarning
	}
	ui.Row(c).Gap(8).Margin(0, 0, 16, 0).Padding(0, cardPadding).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, ic).FontSize(14).TextColor(col).Margin(1, 0, 0, 0)
		ui.Text(c, text).FontSize(12).LineHeight(1.45).TextColor(p.textMuted).Grow(1).Basis(0)
	})
}

// fact is a row of a label and its value.
func (a *app) fact(c *ui.Context, label, value string, mono bool) ui.Element {
	p := a.pal
	if value == "" {
		value = "—"
	}
	return ui.Row(c).MinHeight(36).Padding(8, 0).Gap(16).Children(func() {
		ui.Text(c, label).TextColor(p.textMuted).Grow(1).SingleLine()
		v := ui.Text(c, value).Selectable().TextAlign(ui.End).MaxWidthPercent(70)
		if mono {
			v.Font(fontMono).FontSize(12)
		}
	})
}

// fieldChrome draws the box of a text field: the field's face on the
// controls' hairline, and a line of the focus color while it has the
// focus, or of red around an invalid value.
func (a *app) fieldChrome(box ui.Element, focused, invalid, disabled bool) {
	p := a.pal
	p.control(box.Background(p.field))
	switch {
	case invalid:
		box.Border(1, p.danger).Shadow(0, 0, 0, 3, p.danger.Alpha(0.18))
	case focused:
		box.Border(1, p.focus)
	}
	if disabled {
		box.Opacity(0.45)
	}
}

// fieldOptions say how a text field looks.
type fieldOptions struct {
	placeholder string
	label       string
	width       float32 // 0 grows in the row
	mono        bool
	disabled    bool
	reveal      *bool // a password, shown while *reveal
}

// textField is a text field. It returns the input, for Submitted.
func (a *app) textField(c *ui.Context, key string, value *string, o fieldOptions) ui.Element {
	box := ui.Row(c.Key(key)).Height(controlHeight).Padding(0, 4, 0, 9).Gap(4).Radius(6).AlignItems(ui.Center)
	if o.width > 0 {
		box.Width(o.width).Shrink(0)
	} else {
		box.Grow(1).MinWidth(0)
	}
	var in ui.Element
	focused := false
	box.Children(func() {
		in = ui.TextInputBase(c, value).Grow(1).MinWidth(0).Placeholder(o.placeholder).Disabled(o.disabled).FocusRing(false)
		if o.label != "" {
			in.Label(o.label)
		}
		if o.mono {
			in.Font(fontMono).FontSize(12)
		}
		if o.reveal != nil && !*o.reveal {
			in.Password()
		}
		focused = in.Focused()
		if o.reveal != nil {
			ic, tip, name := iconEye, "Show", "Show password"
			if *o.reveal {
				ic, tip, name = iconEyeOff, "Hide", "Hide password"
			}
			b := a.smallIconButton(c, ic, tip, o.disabled).Label(name)
			if b.Clicked() {
				*o.reveal = !*o.reveal
			}
		} else {
			ui.Box(c).Width(5).Shrink(0)
		}
	})
	a.fieldChrome(box, focused, false, o.disabled)
	return in
}

// numberOptions say what a number field takes.
type numberOptions struct {
	lo, hi, step int64
	unit         string
	label        string
	width        float32
	disabled     bool
}

// numberField edits a whole number in a field with a pair of arrows at its
// end; Up and Down step too (ten steps with Shift). What is typed applies
// while it is a number in range; anything else shows red until the field
// loses the focus.
func numberField[T ~int | ~int64](a *app, c *ui.Context, key string, value *T, o numberOptions) {
	p := a.pal
	if o.step == 0 {
		o.step = 1
	}
	if o.width == 0 {
		o.width = 84
	}
	step := func(by int64, text *string) {
		next := min(o.hi, max(o.lo, int64(*value)+by))
		*value = T(next)
		*text = strconv.FormatInt(next, 10)
	}
	box := ui.Row(c.Key(key)).Height(controlHeight).Width(o.width).Shrink(0).Padding(0, 0, 0, 9).Gap(4).Radius(6).
		AlignItems(ui.Stretch).Clip()
	text := ui.Local(box, "text", func() string { return strconv.FormatInt(int64(*value), 10) })
	focused, invalid := false, false
	box.Children(func() {
		in := ui.TextInputBase(c, text).Grow(1).MinWidth(0).FontFeatures("tnum").
			TextAlign(ui.End).Disabled(o.disabled).FocusRing(false)
		if o.label != "" {
			in.Label(o.label)
		}
		n, err := strconv.ParseInt(strings.TrimSpace(*text), 10, 64)
		valid := err == nil && n >= o.lo && n <= o.hi
		if in.Changed() && valid {
			*value = T(n)
		}
		for _, k := range [...]struct {
			mods ui.Modifiers
			key  ui.Key
			by   int64
		}{{0, ui.KeyUp, o.step}, {ui.Shift, ui.KeyUp, 10 * o.step}, {0, ui.KeyDown, -o.step}, {ui.Shift, ui.KeyDown, -10 * o.step}} {
			if in.Shortcut(k.mods, k.key) {
				if !valid {
					*value = 0
				}
				step(k.by, text)
				valid = true
			}
		}
		focused = in.Focused()
		if !focused {
			*text = strconv.FormatInt(int64(*value), 10)
			valid = true
		}
		invalid = !valid && !o.disabled
		if o.unit != "" {
			ui.Text(c, o.unit).FontSize(12).TextColor(p.textFaint).Shrink(0).AlignSelf(ui.Center)
		}
		a.stepper(c, o.label, o.disabled, func(up bool) {
			by := o.step
			if !up {
				by = -by
			}
			step(by, text)
		})
	})
	a.fieldChrome(box, focused, invalid, o.disabled)
	if invalid {
		box.Tooltip("A whole number from " + formatInt(o.lo) + " to " + formatInt(o.hi))
	}
}

// stepper is the pair of arrows at the end of a number field.
func (a *app) stepper(c *ui.Context, label string, disabled bool, step func(up bool)) {
	p := a.pal
	ui.Column(c).Width(18).Shrink(0).BorderWidth(0, 0, 0, 1).BorderColor(p.borderMuted).Children(func() {
		for _, up := range []bool{true, false} {
			ic, name := iconChevronUp, "Increment "+label
			if !up {
				ic, name = iconChevronDown, "Decrement "+label
			}
			b := ui.ButtonBase(c.Key(up)).Grow(1).Basis(0).Center().Label(name).Disabled(disabled)
			col := p.iconMuted
			switch {
			case disabled:
			case b.Pressed():
				b.Background(p.pressed)
				col = p.icon
			case b.Hovered():
				b.Background(p.hover)
				col = p.icon
			}
			b.Children(func() { ui.Icon(c, ic).FontSize(10).TextColor(col) })
			if b.Clicked() {
				step(up)
			}
		}
	})
}

// picker is a select field choosing one of options from a menu of the
// system's.
func (a *app) picker(c *ui.Context, key, label string, value *string, options []option, disabled bool) {
	p := a.pal
	b := a.buttonBase(c, "picker:"+key, buttonSecondary, disabled).Padding(0, 8, 0, 10).Gap(12).FontWeight(weightRegular).Label(label)
	b.Children(func() {
		ui.Text(c, labelOf(options, *value)).SingleLine().Grow(1)
		ui.Icon(c, iconChevronDown).FontSize(14).TextColor(p.iconMuted)
	})
	b.Menu(func(m *ui.Menu) {
		for _, o := range options {
			if m.Item(o.label).Checked(o.value == *value).Chosen() {
				*value = o.value
			}
		}
	})
}

// segmented is a segmented control of labels, of which *selected is the
// one chosen: a track of gray, and the chosen segment on a lighter face
// that slides to the next one chosen. content, if
// not nil, builds a segment's face in place of its label.
func (a *app) segmented(c *ui.Context, key, label string, selected *int, labels []string, content func(i int, chosen bool)) ui.Element {
	p := a.pal
	parts := ui.SegmentedBase(c.Key(key), selected, len(labels))
	track := parts.Track.Row().Height(controlHeight).Padding(2).Gap(2).Radius(6).Shrink(0).Background(p.segmentTrack).
		Shadow(0, 0, 0, 0.5, p.borderBase).Label(label)
	var chosenSegment ui.Element
	track.Children(func() {
		for i, name := range labels {
			chosen := i == *selected
			s := parts.Segment(i).FillHeight().Padding(0, 12).Gap(6).Radius(4).Center().Shrink(0).Label(name)
			if chosen {
				chosenSegment = s
			}
			s.Children(func() {
				if content != nil {
					content(i, chosen)
					return
				}
				col := p.textMuted
				if chosen {
					col = p.text
				}
				ui.Text(c, name).FontWeight(weightMedium).TextColor(col).SingleLine()
			})
		}
	})
	// The face lies under the segments, where the chosen one was laid out
	// last frame; it jumps there on the control's first frames and slides
	// after.
	at, tb := chosenSegment.LayoutBounds(), track.LayoutBounds()
	x, w := at.X-tb.X, at.W
	if prev := ui.Local(track, "laid-out", func() bool { return false }); !*prev || w == 0 {
		*prev = w > 0
		x, w = track.Jump("face-x", x), track.Jump("face-w", w)
	} else {
		x, w = track.AnimateTo("face-x", x, controlMotion), track.AnimateTo("face-w", w, controlMotion)
	}
	face, ring, shade := p.segmentFace, p.borderStrong, p.shadowPill
	track.Draw(func(pt *ui.Painter, r ui.Rect) {
		if w <= 0 {
			return
		}
		f := ui.Rect{X: r.X + x, Y: r.Y + 2, W: w, H: r.H - 4}
		pt.Shadow(f, 4, 0, 1, 2, 0, shade)
		pt.Fill(f, face, 4)
		pt.Stroke(f, ring, 4, 0.5)
	})
	return track
}

// checkbox is a check box with an icon and a label.
func (a *app) checkbox(c *ui.Context, label string, ic *ui.SVG, on *bool) {
	p := a.pal
	box := ui.CheckboxBase(c.Key(label), on).Height(32).Gap(10).Label(label)
	checked := *on
	box.Children(func() {
		mark := ui.Box(c).Size(16, 16).Radius(4).Center().Shrink(0)
		if checked {
			mark.Background(p.accent).Children(func() {
				ui.Icon(c, iconCheck).FontSize(12).TextColor(ui.RGB(255, 255, 255))
			})
		} else {
			p.control(mark.Background(p.field))
		}
		if box.Pressed() {
			mark.Opacity(0.75)
		}
		col := p.iconFaint
		if checked {
			col = p.iconMuted
		}
		ui.Icon(c, ic).FontSize(16).TextColor(col)
		ui.Text(c, label).Grow(1).SingleLine()
	})
}

// toggle is a switch for a setting, as Hex's: a track of 36 by 16 in the
// accent while on, and a knob that slides across it and stretches while
// pressed.
func (a *app) toggle(c *ui.Context, key string, on *bool, label string, disabled bool) {
	p := a.pal
	const w, h, inset = 36, 16, 1
	sw := ui.SwitchBase(c.Key(key), on).Size(w, h).Shrink(0).Radius(4).Label(label).Disabled(disabled)
	track := p.layer4
	if *on {
		track = p.accent
	}
	sw.Background(sw.AnimateColor("track", track, controlMotion)).InsetShadow(0, 1, 1.5, 0, ui.RGBA(0, 0, 0, 0.12))
	knob := float32(21)
	if sw.Pressed() && !disabled {
		knob = 26
	}
	knobW := sw.AnimateTo("knob-w", knob, controlMotion)
	at := float32(0)
	if *on {
		at = 1
	}
	x := sw.AnimateTo("knob-x", at, controlMotion)
	if disabled {
		sw.Opacity(0.45)
	}
	sw.Children(func() {
		ui.Box(c).Absolute().Top(inset).Left(inset+x*(w-2*inset-knobW)).Size(knobW, h-2*inset).Radius(3).
			Gradient(ui.RGB(255, 255, 255), ui.Hex("#f2f2f2"), 180).
			Shadow(0, 1, 1.5, 0, ui.RGBA(0, 0, 0, 0.2)).Shadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.08))
	})
}
