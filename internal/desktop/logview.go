package desktop

import (
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
		return p.success
	case LevelError:
		return p.danger
	case LevelInfo:
		return p.accent
	case LevelDone:
		return p.text
	}
	return p.iconFaint
}

// The widths of the log's first columns.
const (
	timeColumn  = 62
	levelColumn = 70
)

// logView is the event log, as a table in a card: the filters in a
// segmented control, a search field and the lines under their columns'
// titles, which follow the end as they come.
func (a *app) logView(c *ui.Context) {
	p := a.pal
	a.filterLog()
	p.card(ui.Column(c)).Grow(1).MinHeight(0).Clip().Children(func() {
		ui.Row(c).Gap(8).Padding(10, 10, 10, 12).Shrink(0).Children(func() {
			a.logFilters(c)
			ui.Spacer(c)
			a.logSearch(c)
			following := a.logList.AtEnd()
			follow := a.iconButton(c, iconFollow, "Follow new lines", false)
			if following {
				follow.Background(p.layer2).TextColor(p.icon)
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
		ui.Row(c).Height(28).Padding(0, 16).Gap(10).Shrink(0).BorderWidth(1, 0, 1, 0).BorderColor(p.borderMuted).
			FontSize(11).FontWeight(weightMedium).TextColor(p.textFaint).Children(func() {
			ui.Text(c, "Time").Width(timeColumn).Shrink(0)
			ui.Text(c, "Status").Width(levelColumn).Shrink(0)
			ui.Text(c, "URL").Grow(1.35).Basis(0).MinWidth(0)
			ui.Text(c, "Details").Grow(1).Basis(0).MinWidth(0)
		})
		list := ui.List(c, &a.logList, len(a.logShown), func(i int) { a.logRow(c, i) }).Grow(1).MinHeight(0).Padding(4, 0)
		list.Children(func() {
			if len(a.logShown) > 0 {
				return
			}
			empty := "The log is empty"
			if len(a.log) > 0 {
				empty = "Nothing found"
			}
			ui.Box(c).Fill().Center().Children(func() {
				ui.Text(c, empty).FontWeight(weightMedium).TextColor(p.textFaint)
			})
		})
	})
}

// logFilters choose the level the log shows, each segment with its count.
func (a *app) logFilters(c *ui.Context) {
	p := a.pal
	chosen := 0
	labels := make([]string, len(logFilters))
	for i, f := range logFilters {
		labels[i] = f.label
		if f.level == a.logFilter {
			chosen = i
		}
	}
	seg := a.segmented(c, "filters", "Show", &chosen, labels, func(i int, on bool) {
		f := logFilters[i]
		count := len(a.log)
		if f.level != "" {
			count = a.logCounts[levelSlot(f.level)]
			ui.Box(c).Size(6, 6).Radius(3).Background(a.levelColor(f.level))
		}
		col := p.textMuted
		if on {
			col = p.text
		}
		ui.Text(c, f.label).FontSize(12).FontWeight(weightMedium).TextColor(col).SingleLine()
		ui.Text(c, formatInt(int64(count))).FontSize(11).FontFeatures("tnum").TextColor(p.textFaint).SingleLine()
	}).Shrink(1).MinWidth(0).ClipX()
	if seg.Changed() {
		a.logFilter = logFilters[chosen].level
		a.filterLog()
	}
}

// logSearch is a search field, as Hex's: a magnifier, and a button that
// clears it.
func (a *app) logSearch(c *ui.Context) {
	p := a.pal
	box := ui.Row(c.Key("search")).Width(220).Height(controlHeight).Padding(0, 6, 0, 8).Gap(8).Radius(6).Margin(0, 4, 0, 0)
	focused := false
	box.Children(func() {
		ui.Icon(c, iconSearch).FontSize(16).TextColor(p.iconMuted)
		in := ui.TextInputBase(c, &a.logQuery).Grow(1).MinWidth(0).FontSize(12).Placeholder("Search the log").
			Label("Search the log").FocusRing(false)
		if in.Shortcut(0, ui.KeyEscape) {
			a.logQuery = ""
		}
		focused = in.Focused()
		if a.logQuery != "" {
			if a.smallIconButton(c, iconClose, "Clear search", false).Clicked() {
				a.logQuery = ""
			}
		}
	})
	a.fieldChrome(box, focused, false, false)
	a.filterLog()
}

// logRow builds the line i of those shown.
func (a *app) logRow(c *ui.Context, i int) {
	p := a.pal
	e := a.entry(a.logShown[i])
	row := ui.Row(c).Height(24).Padding(0, 16).Gap(10).FontSize(12).Tooltip(logLine(*e))
	if row.Hovered() {
		row.Background(p.hover)
	}
	row.Children(func() {
		ui.Text(c, formatClock(e.TimeMs)).Width(timeColumn).Shrink(0).FontFeatures("tnum").TextColor(p.textFaint).SingleLine()
		ui.Row(c).Width(levelColumn).Shrink(0).Gap(6).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(a.levelColor(e.Level))
			fg := p.textMuted
			if e.Level == LevelError {
				fg = p.danger
			}
			ui.Text(c, levelLabels[e.Level]).TextColor(fg).SingleLine()
		})
		if e.URL == "" {
			msg := ui.Text(c, e.Message).TextColor(p.textMuted).SingleLine().Grow(1).MinWidth(0)
			if e.Level == LevelDone {
				msg.TextColor(p.text).FontWeight(weightMedium)
			}
			return
		}
		ui.Text(c, e.URL).Font(fontMono).FontSize(11.5).SingleLine().Grow(1.35).Basis(0).MinWidth(0)
		if e.Level == LevelSave && e.Path != "" {
			ui.RichText(c,
				ui.Span{Text: "→ ", Color: p.textFaint},
				ui.Span{Text: shortPath(e.Path, a.cfg.Destination), Color: p.textMuted},
			).Font(fontMono).FontSize(11.5).SingleLine().Grow(1).Basis(0).MinWidth(0)
		} else {
			ui.Text(c, firstNonEmpty(e.Message, shortPath(e.Path, a.cfg.Destination))).TextColor(p.textMuted).SingleLine().Grow(1).Basis(0).MinWidth(0)
		}
	})
}
