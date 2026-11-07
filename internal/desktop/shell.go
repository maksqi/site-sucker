package desktop

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

const (
	sidebarWidth = 220
	panelInset   = 8  // the room around the panel, but on the sidebar's side
	headerHeight = 52 // the panel's header, and TitleBarHeight in run.go
	headerX      = 32 // the header's padding, the page's gutter
)

// sidebarTop is the room above the sidebar's items: on macOS that of the
// window's buttons.
var sidebarTop = map[bool]float32{true: 38, false: 10}[runtime.GOOS == "darwin"]

// modKey is the modifier of the shortcuts, as the menus show it.
var modKey = map[bool]string{true: "⌘", false: "Ctrl"}[runtime.GOOS == "darwin"]

// navItems are the pages the sidebar lists, with the names it and the
// panel's header show.
var navItems = []struct {
	id    section
	icon  *ui.SVG
	label string
}{
	{secDashboard, iconDownload, "Download"},
	{secCrawl, iconScanSearch, "Crawl"},
	{secNetwork, iconNetwork, "Network"},
	{secBrowser, iconAppWindow, "Browser"},
	{secOutput, iconFolderTree, "Output"},
	{secSettings, iconSettings, "Settings"},
}

// view builds the window, as Hex's: a sidebar on the window's deep gray,
// and the page in a panel beside it.
func (a *app) view(c *ui.Context) {
	a.drain()
	c.SetZoom(a.prefs.Zoom)
	a.pal = paletteFor(c.Theme().Dark)
	a.theme = *c.Theme()
	applyPalette(&a.theme, a.pal)
	c.SetTheme(&a.theme)
	for _, t := range a.toasts {
		c.AddToast(t)
	}
	a.toasts = a.toasts[:0]

	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(a.pal.deep).Children(func() {
		a.sidebar(c)
		a.panel(c)
	})
	a.toastLayer(c)
}

func (a *app) content(c *ui.Context) {
	switch a.section {
	case secCrawl:
		a.crawlPage(c)
	case secNetwork:
		a.networkPage(c)
	case secBrowser:
		a.browserPage(c)
	case secOutput:
		a.outputPage(c)
	case secSettings:
		a.settingsPage(c)
	default:
		a.dashboard(c)
	}
}

// The sidebar.

func (a *app) sidebar(c *ui.Context) {
	p := a.pal
	ui.Column(c).Width(sidebarWidth).Shrink(0).Padding(0, 6, 12, 10).Children(func() {
		// The window's buttons sit in the top, which moves the window.
		ui.Box(c).Height(sidebarTop).Shrink(0).DragWindow()
		ui.RadioGroup(c, func() {
			for _, it := range navItems {
				a.navItem(c, it.id, it.icon, it.label)
			}
		}).Gap(4).Label("Pages")
		ui.Box(c).Grow(1).DragWindow()
		ui.Text(c, "Version "+a.version).FontSize(11).TextColor(p.textFaint).Padding(0, 6).SingleLine()
	})
}

// navItem is a page of the sidebar: the one shown on the second layer,
// the others in muted text.
func (a *app) navItem(c *ui.Context, id section, ic *ui.SVG, label string) {
	p := a.pal
	chosen := a.section == id
	item := ui.RadioBase(c.Key(id), &a.section, id).Row().Height(28).Padding(0, 6).Gap(8).Radius(4).Label(label)
	fg, icon := p.textMuted, p.iconMuted
	switch {
	case chosen:
		item.Background(p.layer2)
		fg, icon = p.text, p.icon
	case item.Hovered():
		item.Background(p.hover)
	}
	item.Children(func() {
		ui.Icon(c, ic).FontSize(16).TextColor(icon)
		ui.Text(c, label).TextColor(fg).Grow(1).MinWidth(0).SingleLine()
		switch id {
		case secDashboard:
			a.downloadBadge(c)
		case secNetwork:
			a.proxyBadge(c)
		}
	})
}

func (a *app) downloadBadge(c *ui.Context) {
	p := a.pal
	snap := a.snap
	switch {
	case snap.Running:
		col, mode, text := p.accent, blink, formatCompact(snap.Stats.Saved)
		if snap.Paused {
			col, mode, text = p.warning, still, "paused"
		}
		ui.Row(c).Gap(6).Shrink(0).Children(func() {
			a.dot(c, 6, col, mode, 2*time.Second)
			ui.Text(c, text).FontSize(11).FontFeatures("tnum").TextColor(p.textMuted).SingleLine()
		})
	case snap.Stats.Failed > 0:
		a.badge(c, formatCompact(snap.Stats.Failed), p.dangerSoft, p.danger)
	}
}

func (a *app) proxyBadge(c *ui.Context) {
	if kind := a.cfg.Network.Proxy.Type; kind != "" && kind != "none" {
		a.badge(c, strings.ToUpper(kind), a.pal.layer2, a.pal.textMuted)
	}
}

// badge is a count or a word in a tag, as Hex's status tags.
func (a *app) badge(c *ui.Context, text string, bg, fg ui.Color) {
	ui.Box(c).Height(18).Padding(0, 6).Radius(4).Background(bg).Border(0.5, a.pal.borderBase).Center().Shrink(0).Children(func() {
		ui.Text(c, text).FontSize(11).FontWeight(weightMedium).FontFeatures("tnum").TextColor(fg).SingleLine()
	})
}

// The panel.

// panel holds the page under its header, in the base color, rounded and
// outlined, inset from the window.
func (a *app) panel(c *ui.Context) {
	p := a.pal
	ui.Column(c).Grow(1).MinWidth(0).Margin(panelInset, panelInset, panelInset, 0).Radius(8).
		Background(p.base).Border(1, p.borderWeak).Clip().Children(func() {
		a.header(c)
		ui.Box(c).Grow(1).MinHeight(0).Children(func() {
			ui.Column(c).Key(a.section).Fill().
				Transition(ui.ElementTransition{Duration: 160 * time.Millisecond, Enter: &ui.Motion{}}).
				Children(func() { a.content(c) })
		})
	})
}

// title is what the header calls the page.
func (a *app) title() string {
	for _, it := range navItems {
		if it.id == a.section {
			return it.label
		}
	}
	return "Download"
}

// header names the page; on Download it tells what the crawl does too. It
// moves the window.
func (a *app) header(c *ui.Context) {
	bar := c.TitleBar()
	ui.Row(c).Height(headerHeight).Shrink(0).Padding(0, max(headerX-8, bar.Right), 0, headerX).Gap(8).
		DragWindow().Children(func() {
		a.titleBlock(c)
	})
}

// titleBlock names the page, and for Download what the crawl does.
func (a *app) titleBlock(c *ui.Context) {
	p := a.pal
	ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
		ui.Text(c, a.title()).FontSize(14).FontWeight(500).SingleLine()
		if a.section != secDashboard {
			return
		}
		ui.Row(c).Gap(6).MinWidth(0).Children(func() {
			label, col, mode := a.phase()
			a.dot(c, 6, col, mode, 2*time.Second)
			ui.Text(c, label).FontSize(11).TextColor(p.textMuted).SingleLine().Shrink(0)
			if host := hostOf(a.cfg.StartURL); host != "" {
				ui.Text(c, "· "+host).FontSize(11).TextColor(p.textFaint).SingleLine().MinWidth(0)
			}
		})
	})
}

// The toasts: cards that float at the top right of the panel.

func (a *app) toastLayer(c *ui.Context) {
	p := a.pal
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(panelInset+headerHeight+4, panelInset+16, 16, 16).AlignItems(ui.End).Justify(ui.Start).Gap(8)
		for _, t := range toasts {
			toast := ui.ToastBase(c, t)
			ic, col := iconInfo, p.accentText
			switch NoticeKind(t.Type) {
			case NoticeSuccess:
				ic, col = iconSuccess, p.success
			case NoticeWarning:
				ic, col = iconWarning, p.warning
			case NoticeError:
				ic, col = iconError, p.danger
			}
			toast.Root.Row().Width(340).Gap(10).Padding(12, 10, 12, 14).Radius(8).AlignItems(ui.Start).Background(p.base).
				Transition(ui.ElementTransition{Duration: 240 * time.Millisecond, Enter: &ui.Motion{Y: -6, Scale: 0.96}, Exit: &ui.Motion{Scale: 0.96}})
			shadows(toast.Root, p.shadowFloating)
			toast.Root.Children(func() {
				ui.Icon(c, ic).FontSize(16).TextColor(col).Margin(2, 0, 0, 0)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, t.Title).FontWeight(weightMedium)
					if t.Description != "" {
						ui.Text(c, t.Description).FontSize(12).LineHeight(1.4).TextColor(p.textMuted).Selectable()
					}
					if t.Action != "" {
						act := toast.ActionButton().Height(24).Padding(0, 9).Radius(4).Margin(8, 0, 0, 0).AlignSelf(ui.Start).
							Background(p.button)
						if act.Hovered() {
							act.Background(p.buttonHover)
						}
						p.control(act).Children(func() {
							ui.Text(c, t.Action).FontSize(12).FontWeight(weightMedium)
						})
					}
				})
				closer := toast.CloseButton().Size(20, 20).Radius(4).Center().Label("Close")
				if closer.Hovered() {
					closer.Background(p.hover)
				}
				closer.Children(func() { ui.Icon(c, iconClose).FontSize(12).TextColor(p.iconMuted) })
			})
		}
	})
}
