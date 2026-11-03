package desktop

import (
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

const (
	sidebarWidth = 228
	sidebarInset = 8  // the room around the floating sidebar
	barHeight    = 52 // the toolbar's, and TitleBarHeight in run.go
	barControl   = 34 // the height of the toolbar's capsules
	statusHeight = 26
)

// modKey is the modifier of the shortcuts, as the menus show it.
var modKey = map[bool]string{true: "⌘", false: "Ctrl"}[runtime.GOOS == "darwin"]

// navItems are the sections the sidebar lists, after Download, with the
// short names it and the toolbar show.
var navItems = []struct {
	id    section
	icon  *ui.SVG
	label string
}{
	{secLimits, iconGauge, "Limits"},
	{secNetwork, iconNetwork, "Network"},
	{secChromium, iconAppWindow, "Chromium"},
	{secScope, iconScanSearch, "Scope"},
	{secProxy, iconRoute, "Proxy"},
	{secAuth, iconKey, "Authentication"},
	{secOutput, iconFolderTree, "Files"},
}

// view builds the window: a sidebar floating over it, and the section
// under a toolbar of glass, as a window of SwiftUI.
func (a *app) view(c *ui.Context) {
	a.drain()
	a.pal = paletteFor(c.Theme().Dark)
	a.theme = *c.Theme()
	applyPalette(&a.theme, a.pal)
	c.SetTheme(&a.theme)
	for _, t := range a.toasts {
		c.AddToast(t)
	}
	a.toasts = a.toasts[:0]

	p := a.pal
	ui.Row(c).Fill().AlignItems(ui.Stretch).Background(p.window).Children(func() {
		a.sidebar(c)
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			ui.Box(c).Grow(1).MinHeight(0).Clip().Children(func() {
				ui.Column(c).Key(a.section).Fill().
					Transition(ui.ElementTransition{Duration: 200 * time.Millisecond, Enter: &ui.Motion{}}).
					Children(func() { a.content(c) })
				a.toolbar(c)
			})
			a.statusBar(c)
		})
	})
	a.toastLayer(c)
}

func (a *app) content(c *ui.Context) {
	switch a.section {
	case secLimits:
		a.limitsPage(c)
	case secNetwork:
		a.networkPage(c)
	case secChromium:
		a.chromiumPage(c)
	case secScope:
		a.scopePage(c)
	case secProxy:
		a.proxyPage(c)
	case secAuth:
		a.authPage(c)
	case secOutput:
		a.outputPage(c)
	default:
		a.dashboard(c)
	}
}

// The sidebar.

func (a *app) sidebar(c *ui.Context) {
	p := a.pal
	// Glass over the window, a little gray in light mode, as the
	// sidebars of macOS 26 and later over a white window.
	tint := ui.Transparent
	if !p.dark {
		tint = p.ink(0.05)
	}
	panel := ui.Column(c).Width(sidebarWidth).Shrink(0).Margin(sidebarInset, 0, sidebarInset, sidebarInset).Radius(18).
		Material(glass.Glass{Tint: tint})
	panel.Children(func() {
		// The window controls sit in the panel's top.
		ui.Box(c).Height(barHeight - sidebarInset).Shrink(0).DragWindow()
		// A source list as AppKit's: gray while it has no focus, without
		// a face for the pointer.
		a.sideTheme = a.theme
		a.sideTheme.SurfaceHover = ui.Transparent
		a.sideTheme.SurfacePressed = p.ink(0.1)
		a.sideTheme.Radius = 8
		c.SetTheme(&a.sideTheme)
		chosen := string(a.section)
		ui.Sidebar(c, &chosen, func() {
			ui.SidebarItem(c, string(secDashboard), iconActivity, "Download").Children(func() { a.downloadBadge(c) })
			ui.SidebarSection(c, "Settings", nil, func() {
				for _, it := range navItems {
					item := ui.SidebarItem(c, string(it.id), it.icon, it.label)
					if it.id == secProxy {
						item.Children(func() { a.proxyBadge(c) })
					}
				}
			})
		}).Grow(1).Width(sidebarWidth).Padding(0, 10).Background(ui.Transparent).Label("Sections")
		c.SetTheme(&a.theme)
		a.section = section(chosen)
		if a.snap.Browser != BrowserClosed {
			ui.Column(c).Padding(10).Shrink(0).Children(func() { a.browserChip(c) })
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
			col, mode, text = p.yellow, still, "paused"
		}
		ui.Row(c).Gap(5).Shrink(0).Children(func() {
			a.dot(c, 6, col, mode, 1200*time.Millisecond)
			ui.Text(c, text).FontSize(11).FontFeatures("tnum").TextColor(p.label2).SingleLine()
		})
	case snap.Stats.Failed > 0:
		a.badge(c, formatCompact(snap.Stats.Failed), p.red, ui.RGB(255, 255, 255))
	}
}

func (a *app) proxyBadge(c *ui.Context) {
	if kind := a.cfg.Network.Proxy.Type; kind != "" && kind != "none" {
		a.badge(c, strings.ToUpper(kind), a.pal.ink(0.08), a.pal.label2)
	}
}

// badge is a count or a word in a capsule, as a sidebar's.
func (a *app) badge(c *ui.Context, text string, bg, fg ui.Color) {
	ui.Box(c).Height(17).Padding(0, 6).Radius(8.5).Background(bg).Center().Shrink(0).Children(func() {
		ui.Text(c, text).FontSize(10.5).FontWeight(600).FontFeatures("tnum").TextColor(fg).SingleLine()
	})
}

func (a *app) browserChip(c *ui.Context) {
	p := a.pal
	starting := a.snap.Browser == BrowserStarting
	chip := ui.Row(c).Key("browser").Height(30).Padding(0, 4, 0, 10).Gap(8).Radius(10).Background(p.ink(0.06)).
		Transition(ui.ElementTransition{Animation: &ui.Bouncy, Enter: &ui.Motion{Scale: 0.94}})
	chip.Children(func() {
		if starting {
			a.dot(c, 7, p.yellow, blink, time.Second)
			ui.Text(c, "Starting Chromium…").FontSize(12).Grow(1).SingleLine()
		} else {
			a.dot(c, 7, p.green, still, 0)
			ui.Text(c, "Chromium open").FontSize(12).Grow(1).SingleLine()
		}
		if a.iconButton(c, iconClose, "Close browser", false).Size(22, 22).Clicked() {
			a.closeBrowser()
		}
	})
}

// The toolbar.

// title is what the toolbar calls the section.
func (a *app) title() string {
	for _, it := range navItems {
		if it.id == a.section {
			return it.label
		}
	}
	return "Download"
}

// toolbar floats over the top of the section, which scrolls under it and
// fades there.
func (a *app) toolbar(c *ui.Context) {
	bar := c.TitleBar()
	ui.Box(c).Absolute().Top(0).Left(0).Right(0).Height(barHeight + 10).PassThrough().Material(glass.ScrollEdge{})
	ui.Row(c).Absolute().Top(0).Left(0).Right(0).Height(barHeight).Padding(0, max(14, bar.Right+8), 0, 18).Gap(10).
		DragWindow().Children(func() {
		a.titleBlock(c)
		a.urlField(c)
		a.folderChip(c)
		a.controls(c)
		a.moreMenu(c)
	})
}

// titleBlock names the section, and for Download what the crawl does, as
// a navigation title and subtitle.
func (a *app) titleBlock(c *ui.Context) {
	p := a.pal
	ui.Column(c).Shrink(0).MaxWidth(170).Gap(1).Margin(0, 6, 0, 0).Children(func() {
		ui.Text(c, a.title()).FontSize(15).FontWeight(700).SingleLine()
		if a.section != secDashboard {
			return
		}
		ui.Row(c).Gap(5).MinWidth(0).Children(func() {
			label, col, mode := a.phase()
			a.dot(c, 6, col, mode, 1600*time.Millisecond)
			ui.Text(c, label).FontSize(11).TextColor(p.label2).SingleLine().Shrink(0)
			if host := hostOf(a.cfg.StartURL); host != "" {
				ui.Text(c, "· "+host).FontSize(11).TextColor(p.label2).SingleLine().MinWidth(0)
			}
		})
	})
}

// capsule is a control of the toolbar: a capsule of glass, which casts a
// soft shadow on a light window, as the toolbars of macOS 26 and later.
func (a *app) capsule(e ui.Element) ui.Element {
	return a.floating(e.Height(barControl).Radius(barControl / 2).Shrink(0).Material(glass.Glass{}))
}

// floating gives glass over a light window its shadow.
func (a *app) floating(e ui.Element) ui.Element {
	if !a.pal.dark {
		e.Shadow(0, 2, 8, -2, ui.RGBA(0, 0, 0, 0.1)).Shadow(0, 0, 1, 0, ui.RGBA(0, 0, 0, 0.06))
	}
	return e
}

func (a *app) urlField(c *ui.Context) {
	p := a.pal
	running := a.snap.Running
	box := a.capsule(ui.Row(c.Key("url"))).Grow(1).Shrink(1).MinWidth(160).Padding(0, 14, 0, 6).Gap(8)
	focused := false
	box.Children(func() {
		host := hostOf(a.cfg.StartURL)
		badge := ui.Box(c).Size(22, 22).Shrink(0).Radius(11).Center()
		if host != "" {
			badge.Background(p.accent).Children(func() {
				ui.Text(c, hostInitial(host)).FontSize(11).FontWeight(700).TextColor(p.accentText)
			})
		} else {
			badge.Children(func() { ui.Icon(c, iconGlobe).FontSize(15).TextColor(p.label2) })
		}
		in := ui.TextInputBase(c, &a.cfg.StartURL).Grow(1).MinWidth(0).
			Placeholder("https://example.com/").Label("Website address").ReadOnly(running).FocusRing(false)
		if in.Submitted() {
			a.start()
		}
		focused = in.Focused()
	})
	if focused {
		box.Shadow(0, 0, 0, 3.5, p.accentRing)
	}
}

func (a *app) folderChip(c *ui.Context) {
	p := a.pal
	dest := strings.TrimSpace(a.cfg.Destination)
	label, tip := baseName(dest), tildify(dest)
	switch {
	case a.cfg.AskDestination:
		label, tip = "Ask for folder", "The folder will be asked for at start"
	case dest == "":
		label = "Choose folder"
	}
	a.capsule(ui.Row(c)).Clip().Children(func() {
		pick := ui.ButtonBase(c.Key("pick")).FillHeight().Padding(0, 12, 0, 14).Gap(7).MaxWidth(180).Label("Choose folder").Tooltip(tip)
		if pick.Hovered() {
			pick.Background(p.fill3)
		}
		pick.Children(func() {
			ui.Icon(c, iconFolder).FontSize(15).TextColor(p.accent)
			ui.Text(c, label).SingleLine().MinWidth(0)
		})
		if pick.Clicked() {
			a.chooseFolder()
		}
		ui.Box(c).Width(1).Height(18).Background(p.separator)
		open := ui.ButtonBase(c.Key("open")).FillHeight().Width(38).Center().
			Label("Open download folder").Tooltip("Open download folder")
		if open.Hovered() {
			open.Background(p.fill3)
		}
		open.Children(func() { ui.Icon(c, iconFolderOpen).FontSize(15).TextColor(p.label) })
		if open.Clicked() {
			a.openFolder()
		}
	})
}

func (a *app) controls(c *ui.Context) {
	p := a.pal
	snap := a.snap
	swap := ui.ElementTransition{Duration: 220 * time.Millisecond, Enter: &ui.Motion{Scale: 0.92}}
	if !snap.Running {
		start := ui.ButtonBase(c.Key("start")).Height(barControl).Radius(barControl/2).Shrink(0).Padding(0, 16, 0, 14).Gap(7).
			Material(glass.Glass{Tint: p.accent, Interactive: true}).Label("Start").Tooltip("Start (" + modKey + "↩)").Transition(swap)
		a.floating(start)
		start.Children(func() {
			ui.Icon(c, iconPlay).FontSize(12).TextColor(p.accentText)
			ui.Text(c, "Start").FontWeight(600).TextColor(p.accentText).SingleLine()
		})
		if start.Clicked() {
			a.start()
		}
		return
	}
	a.capsule(ui.Row(c.Key("running"))).Clip().Transition(swap).Children(func() {
		ic, label := iconPause, "Pause"
		if snap.Paused {
			ic, label = iconPlay, "Resume"
		}
		half := func(key, label string, disabled bool) ui.Element {
			b := ui.ButtonBase(c.Key(key)).FillHeight().Padding(0, 14).Gap(6).Label(label).Disabled(disabled)
			if b.Hovered() && !disabled {
				b.Background(p.fill3)
			}
			if disabled {
				b.Opacity(0.45)
			}
			return b
		}
		pause := half("pause", label, snap.Stopping)
		pause.Children(func() {
			ui.Icon(c, ic).FontSize(14)
			ui.Text(c, label).SingleLine()
		})
		if pause.Clicked() {
			a.togglePause()
		}
		ui.Box(c).Width(1).Height(18).Background(p.separator)
		stop := half("stop", "Stop", snap.Stopping).TextColor(p.red)
		stop.Children(func() {
			if snap.Stopping {
				ui.Spinner(c).Size(13, 13).TextColor(p.red)
				ui.Text(c, "Stopping…").SingleLine()
			} else {
				ui.Icon(c, iconStop).FontSize(11)
				ui.Text(c, "Stop").SingleLine()
			}
		})
		if stop.Clicked() {
			a.stop()
		}
	})
}

// moreMenu holds what the toolbar has no room for: the profile's import
// and export, in a menu of the system's.
func (a *app) moreMenu(c *ui.Context) {
	more := ui.ButtonBase(c.Key("more")).Size(barControl, barControl).Radius(barControl / 2).Shrink(0).Center().
		Material(glass.Glass{Interactive: true}).Label("More").Tooltip("Profile")
	a.floating(more)
	more.Children(func() { ui.Icon(c, iconMore).FontSize(17) })
	more.Menu(func(m *ui.Menu) {
		if m.Item("Import Profile…").Shortcut(ui.Cmd, ui.KeyO).Chosen() {
			a.importProfile()
		}
		if m.Item("Export Profile…").Shortcut(ui.Cmd, ui.KeyS).Chosen() {
			a.exportProfile()
		}
	})
}

// The status bar.

func (a *app) statusBar(c *ui.Context) {
	p := a.pal
	snap := a.snap
	ui.Row(c).Height(statusHeight).Shrink(0).Padding(0, 16).Gap(14).BorderWidth(1, 0, 0, 0).BorderColor(p.separator).
		FontSize(11.5).TextColor(p.label2).Children(func() {
		switch {
		case snap.Stopping || snap.Paused:
			a.dot(c, 6, p.yellow, still, 0)
		case snap.Running:
			a.dot(c, 6, p.accent, blink, 1200*time.Millisecond)
		default:
			a.dot(c, 6, p.label3, still, 0)
		}
		ui.Text(c, snap.Status).Selectable().SingleLine().Grow(1).MinWidth(0).Tooltip(snap.Status)
		stats := snap.Stats
		counter := func(label string, value string, col ui.Color) {
			ui.Row(c).Gap(4).Shrink(0).Children(func() {
				if label != "" {
					ui.Text(c, label).SingleLine()
				}
				ui.Text(c, value).FontWeight(500).FontFeatures("tnum").TextColor(col).SingleLine()
			})
		}
		counter("Discovered", formatInt(stats.Discovered), p.label)
		counter("Saved", formatInt(stats.Saved), p.label)
		errors := p.label
		if stats.Failed > 0 {
			errors = p.red
		}
		counter("Errors", formatInt(stats.Failed), errors)
		counter("", formatBytes(stats.Bytes), p.label)
	})
}

// The toasts, as macOS's notification banners: glass at the top right.

func (a *app) toastLayer(c *ui.Context) {
	p := a.pal
	ui.ToastViewportBase(c, func(viewport ui.Element, toasts []ui.Toast) {
		viewport.Padding(barHeight+4, 16, 16, 16).AlignItems(ui.End).Justify(ui.Start).Gap(8)
		for _, t := range toasts {
			toast := ui.ToastBase(c, t)
			ic, col := iconInfo, p.blue
			switch NoticeKind(t.Type) {
			case NoticeSuccess:
				ic, col = iconSuccess, p.green
			case NoticeWarning:
				ic, col = iconWarning, p.yellow
			case NoticeError:
				ic, col = iconError, p.red
			}
			toast.Root.Row().Width(340).Gap(11).Padding(12, 10, 13, 12).Radius(18).AlignItems(ui.Start).
				Material(glass.Glass{}).Shadow(0, 10, 30, -8, p.shadow).
				Transition(ui.ElementTransition{Duration: 300 * time.Millisecond, Enter: &ui.Motion{X: 30}, Exit: &ui.Motion{X: 30}})
			toast.Root.Children(func() {
				symbol(c, ic, col, 28)
				ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
					ui.Text(c, t.Title).FontWeight(600)
					if t.Description != "" {
						ui.Text(c, t.Description).FontSize(12).TextColor(p.label2).Selectable()
					}
					if t.Action != "" {
						act := toast.ActionButton().Height(22).Padding(0, 10).Radius(11).Margin(6, 0, 0, 0).AlignSelf(ui.Start).
							Background(p.ink(0.08))
						if act.Hovered() {
							act.Background(p.ink(0.12))
						}
						act.Children(func() {
							ui.Text(c, t.Action).FontSize(12).FontWeight(500)
						})
					}
				})
				closer := toast.CloseButton().Size(20, 20).Radius(10).Center().Label("Close")
				if closer.Hovered() {
					closer.Background(p.fill3)
				}
				closer.Children(func() { ui.Icon(c, iconClose).FontSize(11).TextColor(p.label2) })
			})
		}
	})
}
