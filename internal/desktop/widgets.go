package desktop

import (
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

// controlHeight is the height of buttons, fields and pop-up buttons, as
// macOS's regular controls.
const controlHeight = 26

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
	blink        // fades to 30% and back
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
			alpha = 1 - 0.7*(0.5-0.5*float32(math.Cos(2*math.Pi*float64(phase(p.Now(), period)))))
		case beacon:
			p.AnimationFrame()
			t := phase(p.Now(), period)
			t = 1 - (1-t)*(1-t)*(1-t)
			p.Shadow(r, r.W/2, 0, 0, 0, 6*t, col.Alpha(0.45*(1-t)))
		}
		p.Fill(r, col.Alpha(alpha), r.W/2)
	})
}

// symbol is an icon on a rounded square of a color, as System Settings
// shows its panes.
func symbol(c *ui.Context, ic *ui.SVG, tint ui.Color, size float32) ui.Element {
	return ui.Box(c).Size(size, size).Shrink(0).Radius(size*0.23).Center().
		Gradient(tint.Mix(ui.RGB(255, 255, 255), 0.18), tint, 180).
		InsetShadow(0, 0, 0, 0.5, ui.RGBA(0, 0, 0, 0.08)).
		Children(func() {
			ui.Icon(c, ic).FontSize(size * 0.58).TextColor(ui.RGB(255, 255, 255))
		})
}

// buttonKind is the look of a button.
type buttonKind uint8

const (
	buttonDefault buttonKind = iota // bordered
	buttonPrimary                   // bordered, prominent
	buttonGhost                     // borderless
	buttonDanger                    // bordered, destructive
)

type buttonKey struct {
	label string
	icon  *ui.SVG
}

// buttonBase is a push button, a capsule as on macOS, whose content the
// caller builds.
func (a *app) buttonBase(c *ui.Context, key any, kind buttonKind, disabled bool) ui.Element {
	p := a.pal
	b := ui.ButtonBase(c.Key(key)).Height(controlHeight).Padding(0, 12).Gap(6).Radius(controlHeight / 2).
		Justify(ui.Center).Shrink(0).Disabled(disabled)
	hovered, pressed := b.Hovered() && !disabled, b.Pressed() && !disabled
	fg := p.label
	switch kind {
	case buttonPrimary:
		bg := p.accent
		switch {
		case pressed:
			bg = bg.Mix(ui.RGB(0, 0, 0), 0.16)
		case hovered:
			bg = bg.Mix(ui.RGB(0, 0, 0), 0.07)
		}
		b.Background(bg).FontWeight(500)
		fg = p.accentText
	case buttonGhost:
		fg = p.label2
		switch {
		case pressed:
			b.Background(p.fill)
			fg = p.label
		case hovered:
			b.Background(p.fill3)
			fg = p.label
		}
	default:
		alpha := float32(0.06)
		if p.dark {
			alpha = 0.1
		}
		switch {
		case pressed:
			alpha += 0.08
		case hovered:
			alpha += 0.03
		}
		b.Background(p.ink(alpha))
		if kind == buttonDanger {
			fg = p.red
		}
	}
	if disabled {
		b.Opacity(0.4)
	}
	b.Transition(ui.ElementTransition{Colors: true, Duration: 120 * time.Millisecond})
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
			ui.Icon(c, ic).FontSize(14)
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

// page lays out a pane of settings, as System Settings does: a header of
// its symbol, title and description, then grouped sections, scrolling
// under the toolbar.
func (a *app) page(c *ui.Context, ic *ui.SVG, tint ui.Color, title, description string, body func()) {
	p := a.pal
	ui.Scroll(c).Fill().Padding(barHeight+14, 24, 40, 24).ScrollbarInsets(barHeight, 0, 0, 0).Children(func() {
		ui.Column(c).FillWidth().MaxWidth(640).AlignSelf(ui.Center).Children(func() {
			p.groupBox(ui.Column(c)).Padding(22, 28, 20, 28).Gap(4).AlignItems(ui.Center).Margin(0, 0, 24, 0).Children(func() {
				symbol(c, ic, tint, 52).Margin(0, 0, 8, 0)
				ui.Text(c, title).FontSize(15).FontWeight(700)
				ui.Text(c, description).FontSize(12).TextColor(p.label2).TextAlign(ui.Center)
			})
			body()
		})
	})
}

// group is a section of a grouped form: its rows in a rounded box under
// a title, with a footnote below.
func (a *app) group(c *ui.Context, title, footer string, body func()) {
	p := a.pal
	ui.Column(c).Margin(0, 0, 22, 0).Children(func() {
		if title != "" {
			ui.Text(c, title).FontWeight(600).Padding(0, 10).Margin(0, 0, 7, 0)
		}
		p.groupBox(ui.Column(c)).Padding(0, 12).Children(func() {
			ui.Column(c).Gap(1).Dividers(1, p.separator).Children(body)
		})
		if footer != "" {
			ui.Text(c, footer).FontSize(11.5).LineHeight(1.4).TextColor(p.label2).Padding(0, 10).Margin(7, 0, 0, 0)
		}
	})
}

func (a *app) rowText(c *ui.Context, label, hint string, dim bool) ui.Element {
	p := a.pal
	col := ui.Column(c).Gap(2).MinWidth(0).Children(func() {
		ui.Text(c, label)
		if hint != "" {
			ui.Text(c, hint).FontSize(11).TextColor(p.label2)
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
	ui.Row(c).MinHeight(42).Padding(8, 0).Gap(16).Children(func() {
		a.rowText(c, label, hint, dim).Grow(1).Basis(0)
		ui.Row(c).Gap(8).Justify(ui.End).Shrink(0).MaxWidthPercent(62).Children(control)
	})
}

// stacked is a setting whose control takes the whole width, below its
// label.
func (a *app) stacked(c *ui.Context, label, hint string, dim bool, control func()) {
	ui.Column(c).Gap(8).Padding(10, 0, 11, 0).Children(func() {
		a.rowText(c, label, hint, dim)
		ui.Row(c).Gap(8).Children(control)
	})
}

// callout is a note below the sections, or a warning.
func (a *app) callout(c *ui.Context, warning bool, text string) {
	p := a.pal
	col, ic := p.label2, iconInfo
	if warning {
		col, ic = p.yellow, iconWarning
	}
	ui.Row(c).Gap(8).Margin(0, 0, 14, 0).Padding(0, 10).AlignItems(ui.Start).Children(func() {
		ui.Icon(c, ic).FontSize(14).TextColor(col).Margin(1, 0, 0, 0)
		ui.Text(c, text).FontSize(12).LineHeight(1.45).TextColor(p.label2).Grow(1).Basis(0)
	})
}

// fieldChrome draws the box of a text field, as AppKit's: its face, a
// hairline, and the focus ring or the red ring of an invalid value.
func (a *app) fieldChrome(box ui.Element, focused, invalid, disabled bool) {
	p := a.pal
	border := p.ink(0.12)
	switch {
	case invalid:
		border = p.red.Alpha(0.7)
		box.Shadow(0, 0, 0, 3, p.red.Alpha(0.35))
	case focused:
		border = p.accent.Alpha(0.7)
		box.Shadow(0, 0, 0, 3.5, p.accentRing)
	}
	box.Background(p.field).Border(1, border)
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
	box := ui.Row(c.Key(key)).Height(controlHeight).Padding(0, 3, 0, 8).Gap(4).Radius(7).AlignItems(ui.Center)
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
			b := a.button(c, buttonGhost, ic, "", o.disabled).Size(20, 20).Label(name).Tooltip(tip)
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

// numberField edits a whole number with a field and a stepper beside it,
// as AppKit's forms do; Up and Down step too (ten steps with Shift). What
// is typed applies while it is a number in range; anything else shows red
// until the field loses the focus.
func numberField[T ~int | ~int64](a *app, c *ui.Context, key string, value *T, o numberOptions) {
	p := a.pal
	if o.step == 0 {
		o.step = 1
	}
	if o.width == 0 {
		o.width = 72
	}
	step := func(by int64, text *string) {
		next := min(o.hi, max(o.lo, int64(*value)+by))
		*value = T(next)
		*text = strconv.FormatInt(next, 10)
	}
	ui.Row(c.Key(key)).Gap(4).Shrink(0).Children(func() {
		box := ui.Row(c).Height(controlHeight).Width(o.width).Shrink(0).Padding(0, 8).Gap(4).Radius(7).AlignItems(ui.Center)
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
				ui.Text(c, o.unit).FontSize(12).TextColor(p.label2).Shrink(0)
			}
		})
		a.fieldChrome(box, focused, invalid, o.disabled)
		if invalid {
			box.Tooltip("A whole number from " + formatInt(o.lo) + " to " + formatInt(o.hi))
		}
		a.stepper(c, o.label, o.disabled, func(up bool) {
			by := o.step
			if !up {
				by = -by
			}
			step(by, text)
		})
	})
}

// stepper is a pair of arrows beside a field, as AppKit's NSStepper.
func (a *app) stepper(c *ui.Context, label string, disabled bool, step func(up bool)) {
	p := a.pal
	arrows := ui.Column(c).Width(15).Height(controlHeight-2).Shrink(0).Radius(5).Background(p.ink(0.06)).Clip().
		Dividers(0.5, p.separator).Gap(0.5)
	if disabled {
		arrows.Opacity(0.4)
	}
	arrows.Children(func() {
		for _, up := range []bool{true, false} {
			ic, name := iconChevronUp, "Increment "+label
			if !up {
				ic, name = iconChevronDown, "Decrement "+label
			}
			b := ui.ButtonBase(c.Key(up)).Grow(1).Basis(0).Center().Label(name).Disabled(disabled)
			if b.Pressed() && !disabled {
				b.Background(p.fill)
			}
			b.Children(func() { ui.Icon(c, ic).FontSize(9).TextColor(p.label) })
			if b.Clicked() {
				step(up)
			}
		}
	})
}

// picker is a pop-up button choosing one of options from a menu of the
// system's, as SwiftUI's Picker in a form.
func (a *app) picker(c *ui.Context, key, label string, value *string, options []option, disabled bool) {
	p := a.pal
	b := a.buttonBase(c, "picker:"+key, buttonDefault, disabled).Padding(0, 7, 0, 11).Gap(10).Label(label)
	b.Children(func() {
		ui.Text(c, labelOf(options, *value)).SingleLine()
		ui.Icon(c, iconChevronsUpDown).FontSize(11).TextColor(p.label2)
	})
	b.Menu(func(m *ui.Menu) {
		for _, o := range options {
			if m.Item(o.label).Checked(o.value == *value).Chosen() {
				*value = o.value
			}
		}
	})
}

// checkbox is a check box with an icon and a label, as AppKit's.
func (a *app) checkbox(c *ui.Context, label string, ic *ui.SVG, on *bool) {
	p := a.pal
	box := ui.CheckboxBase(c.Key(label), on).Height(30).Gap(8).Label(label)
	checked := *on
	box.Children(func() {
		mark := ui.Box(c).Size(15, 15).Radius(4).Center().Shrink(0)
		if checked {
			mark.Background(p.accent).Children(func() {
				ui.Icon(c, iconCheck).FontSize(11).TextColor(p.accentText)
			})
		} else {
			mark.Background(p.field).Border(1, p.ink(0.2))
		}
		if box.Pressed() {
			mark.Opacity(0.75)
		}
		col := p.label2
		if checked {
			col = p.accent
		}
		ui.Icon(c, ic).FontSize(15).TextColor(col)
		ui.Text(c, label).Grow(1).SingleLine()
	})
}

// toggle is a switch for a setting.
func (a *app) toggle(c *ui.Context, key string, on *bool, label string, disabled bool) {
	ui.Switch(c.Key(key), on).Label(label).Disabled(disabled)
}
