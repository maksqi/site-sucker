package desktop

import (
	"time"

	"github.com/egoist/mygo/ui"
)

var logFilters = []struct {
	level LogLevel
	label string
}{
	{"", "All"},
	{LevelSave, "Saved"},
	{LevelSkip, "Skipped"},
	{LevelError, "Errors"},
	{LevelInfo, "Info"},
}

var levelLabels = map[LogLevel]string{
	LevelSave: "saved", LevelSkip: "skipped", LevelError: "error", LevelInfo: "info", LevelDone: "done",
}

func (a *app) levelColor(l LogLevel) ui.Color {
	p := a.pal
	switch l {
	case LevelSave:
		return p.green
	case LevelError:
		return p.red
	case LevelInfo:
		return p.blue
	case LevelDone:
		return p.accent
	}
	return p.gray
}

// The widths of the log's first columns.
const (
	timeColumn  = 62
	levelColumn = 70
)

// logView is the event log, as a table: a segmented control of filters, a
// search field and the lines under their columns' titles, which follow the
// end as they come.
func (a *app) logView(c *ui.Context) {
	p := a.pal
	a.filterLog()
	p.groupBox(ui.Column(c)).Grow(1).MinHeight(0).Clip().Children(func() {
		ui.Row(c).Gap(10).Padding(8, 8, 8, 10).Shrink(0).Children(func() {
			ui.Row(c).Gap(2).Padding(2).Radius(15).Background(p.fill3).Shrink(1).MinWidth(0).ClipX().Children(func() {
				for _, f := range logFilters {
					a.filterChip(c, f.level, f.label)
				}
			})
			ui.Spacer(c)
			ui.Row(c).Gap(2).Shrink(0).Children(func() {
				a.logSearch(c)
				following := a.logList.AtEnd()
				follow := a.iconButton(c, iconFollow, "Follow new lines", false)
				if following {
					follow.Background(p.accentSoft).TextColor(p.accent)
				}
				if follow.Clicked() {
					a.logList.ScrollToEnd()
				}
				if a.iconButton(c, iconCopy, "Copy shown lines", len(a.logShown) == 0).Clicked() {
					n := int64(len(a.logShown))
					c.WriteClipboard(a.copyText())
					a.toast(NoticeSuccess, "Log copied", formatInt(n)+" "+plural(n, "line", "lines"))
				}
				if a.iconButton(c, iconTrash, "Clear log", len(a.log) == 0).Clicked() {
					a.clearLog()
				}
			})
		})
		ui.Row(c).Height(24).Padding(0, 14).Gap(10).Shrink(0).BorderWidth(1, 0, 1, 0).BorderColor(p.separator).
			FontSize(11).FontWeight(600).TextColor(p.label2).Children(func() {
			ui.Text(c, "Time").Width(timeColumn).Shrink(0)
			ui.Text(c, "Status").Width(levelColumn).Shrink(0)
			ui.Text(c, "URL").Grow(1.35).Basis(0).MinWidth(0)
			ui.Text(c, "Details").Grow(1).Basis(0).MinWidth(0)
		})
		list := ui.List(c, &a.logList, len(a.logShown), func(i int) { a.logRow(c, i) }).Grow(1).MinHeight(0)
		list.Children(func() {
			if len(a.logShown) > 0 {
				return
			}
			empty := "The log is empty"
			if len(a.log) > 0 {
				empty = "Nothing found"
			}
			ui.Box(c).Fill().Center().Children(func() {
				ui.Text(c, empty).FontSize(15).FontWeight(600).TextColor(p.label3)
			})
		})
	})
}

// filterChip is a segment of the filters, with its count.
func (a *app) filterChip(c *ui.Context, level LogLevel, label string) {
	p := a.pal
	count := len(a.log)
	if level != "" {
		count = a.logCounts[levelSlot(level)]
	}
	chosen := a.logFilter == level
	chip := ui.ButtonBase(c.Key(label)).Height(24).Padding(0, 10).Gap(5).Radius(12).Shrink(0).Role(ui.RoleTab).Selected(chosen).Label(label).
		TextColor(p.label)
	if chosen {
		chip.Background(p.raised).Shadow(0, 1, 2, 0, p.shadow).Shadow(0, 0, 0, 0.5, p.ink(0.06))
	}
	chip.Transition(ui.ElementTransition{Colors: true, Duration: 150 * time.Millisecond})
	chip.Children(func() {
		if level != "" {
			ui.Box(c).Size(6, 6).Radius(3).Background(a.levelColor(level))
		}
		ui.Text(c, label).FontSize(12).FontWeight(500).SingleLine()
		ui.Text(c, formatInt(int64(count))).FontSize(11).FontFeatures("tnum").TextColor(p.label2).SingleLine()
	})
	if chip.Clicked() {
		a.logFilter = level
		a.filterLog()
	}
}

// logSearch is a search field, a capsule with a magnifier as AppKit's.
func (a *app) logSearch(c *ui.Context) {
	p := a.pal
	box := ui.Row(c.Key("search")).Width(210).Height(26).Padding(0, 10).Gap(6).Radius(13).Margin(0, 4, 0, 0).
		Background(p.field).Border(1, p.ink(0.1))
	focused := false
	box.Children(func() {
		ui.Icon(c, iconSearch).FontSize(13).TextColor(p.label2)
		in := ui.TextInputBase(c, &a.logQuery).Grow(1).MinWidth(0).FontSize(12).Placeholder("Search").
			Label("Search the log").FocusRing(false)
		if in.Shortcut(0, ui.KeyEscape) {
			a.logQuery = ""
		}
		focused = in.Focused()
		if a.logQuery != "" {
			clear := ui.ButtonBase(c.Key("clear")).Size(16, 16).Radius(8).Center().Background(p.label3).Label("Clear search")
			clear.Children(func() { ui.Icon(c, iconClose).FontSize(10).TextColor(p.window) })
			if clear.Clicked() {
				a.logQuery = ""
			}
		}
	})
	if focused {
		box.Border(1, p.accent.Alpha(0.7)).Shadow(0, 0, 0, 3.5, p.accentRing)
	}
	a.filterLog()
}

// logRow builds the line i of those shown.
func (a *app) logRow(c *ui.Context, i int) {
	p := a.pal
	e := a.entry(a.logShown[i])
	row := ui.Row(c).Height(22).Padding(0, 14).Gap(10).FontSize(12).Tooltip(logLine(*e))
	switch {
	case row.Hovered():
		row.Background(p.fill3)
	case i%2 == 1:
		row.Background(p.fill4)
	}
	row.Children(func() {
		ui.Text(c, formatClock(e.TimeMs)).Width(timeColumn).Shrink(0).FontFeatures("tnum").TextColor(p.label2).SingleLine()
		ui.Row(c).Width(levelColumn).Shrink(0).Gap(6).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(a.levelColor(e.Level))
			fg := p.label2
			if e.Level == LevelError {
				fg = p.red
			}
			ui.Text(c, levelLabels[e.Level]).FontSize(11.5).TextColor(fg).SingleLine()
		})
		if e.URL == "" {
			msg := ui.Text(c, e.Message).TextColor(p.label2).SingleLine().Grow(1).MinWidth(0)
			if e.Level == LevelDone {
				msg.TextColor(p.label).FontWeight(600)
			}
			return
		}
		ui.Text(c, e.URL).Font(fontMono).FontSize(11.5).SingleLine().Grow(1.35).Basis(0).MinWidth(0)
		if e.Level == LevelSave && e.Path != "" {
			ui.RichText(c,
				ui.Span{Text: "→ ", Color: p.label3},
				ui.Span{Text: shortPath(e.Path, a.cfg.Destination), Color: p.label2},
			).Font(fontMono).FontSize(11.5).SingleLine().Grow(1).Basis(0).MinWidth(0)
		} else {
			ui.Text(c, firstNonEmpty(e.Message, shortPath(e.Path, a.cfg.Destination))).TextColor(p.label2).SingleLine().Grow(1).Basis(0).MinWidth(0)
		}
	})
}
