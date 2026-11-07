package desktop

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
)

var (
	countUp = ui.Animation{Duration: 500 * time.Millisecond, Ease: ui.CubicBezier(0.33, 1, 0.68, 1)}
	rise    = ui.ElementTransition{Duration: 300 * time.Millisecond, Ease: ui.CubicBezier(0.22, 1, 0.36, 1), Enter: &ui.Motion{Y: 6}}
)

func (a *app) dashboard(c *ui.Context) {
	if !a.hasRun() {
		a.welcome(c)
		return
	}
	ui.Column(c).Fill().Gap(16).Padding(4, 16, 16, 16).Children(func() {
		a.addressBar(c, false)
		a.meters(c)
		a.progressMeter(c)
		a.logView(c)
	})
}

// phase is what the header says the crawl is doing.
func (a *app) phase() (string, ui.Color, pulse) {
	p, snap := a.pal, a.snap
	switch {
	case snap.Stopping:
		return "Stopping", p.warning, still
	case snap.Paused:
		return "Paused", p.warning, still
	case snap.Running:
		return "Downloading", p.accent, beacon
	case snap.Stats.FinishedAtMs != 0:
		return "Finished", p.success, still
	}
	return "Ready to start", p.iconFaint, still
}

// welcome is Download before the first crawl: the address in a large bar
// in the middle of the panel, and the settings the crawl would start with
// in tiles below it.
func (a *app) welcome(c *ui.Context) {
	p := a.pal
	// The content stands in the middle of the panel, and scrolls in a
	// window too short for it.
	_, h := c.Size()
	ui.Scroll(c).Fill().Padding(0, 32, 32, 32).Children(func() {
		ui.Column(c).Key("welcome").FillWidth().MaxWidth(620).MinHeight(max(0, h-2*panelInset-headerHeight-40)).AlignSelf(ui.Center).
			AlignItems(ui.Center).Justify(ui.Center).Transition(rise).Children(func() {
			ui.Box(c).Size(48, 48).Radius(12).Background(p.layer1).Shadow(0, 0, 0, 0.5, p.borderBase).Center().Children(func() {
				ui.Icon(c, iconDownload).FontSize(22).TextColor(p.iconMuted)
			})
			ui.Text(c, "Download a website").FontSize(20).FontWeight(600).LetterSpacing(-0.2).TextAlign(ui.Center).Margin(18, 0, 0, 0)
			ui.Text(c, "Pages, styles, scripts, images and fonts are saved with local links, so the copy opens without the internet.").
				MaxWidth(420).LineHeight(1.45).TextColor(p.textMuted).TextAlign(ui.Center).Margin(6, 0, 0, 0)
			ui.Column(c).FillWidth().Margin(28, 0, 0, 0).Children(func() { a.addressBar(c, true) })
			ui.Text(c, "Press Enter or "+modKey+"↩ to start").FontSize(11).TextColor(p.textFaint).Margin(10, 0, 0, 0)
			ui.Row(c).FillWidth().Margin(36, 0, 10, 0).Padding(0, 2).Children(func() {
				ui.Text(c, "Starts with").FontWeight(weightMedium).TextColor(p.textMuted).Grow(1)
				ui.Text(c, "Click a tile to change it").FontSize(11).TextColor(p.textFaint)
			})
			ui.Grid(c).FillWidth().Columns(3).Gap(10).Children(func() {
				for _, item := range a.preflight() {
					a.preflightTile(c, item.label, item.value, item.to)
				}
			})
		})
	})
}

// addressBar is the address of the site with the controls of the crawl:
// in the middle of the panel before the first crawl (hero), at the top of
// Download after it. While a crawl runs the address stays as it is.
func (a *app) addressBar(c *ui.Context, hero bool) {
	p := a.pal
	height, button, text := float32(48), float32(32), float32(14)
	if hero {
		height, button, text = 56, 38, 15
	}
	inset := (height - button) / 2
	box := ui.Row(c.Key("address")).FillWidth().Height(height).Shrink(0).Padding(0, inset, 0, inset).Gap(10).Radius(10)
	focused := false
	box.Children(func() {
		host := hostOf(a.cfg.StartURL)
		badge := ui.Box(c).Size(button, button).Shrink(0).Radius(7).Center()
		if host != "" {
			badge.Background(p.contrast).Children(func() {
				ui.Text(c, hostInitial(host)).FontSize(text).FontWeight(600).TextColor(p.contrastText)
			})
		} else {
			badge.Background(p.layer2).Children(func() {
				ui.Icon(c, iconGlobe).FontSize(18).TextColor(p.iconMuted)
			})
		}
		in := ui.TextInputBase(c, &a.cfg.StartURL).Grow(1).MinWidth(0).FontSize(text).
			Placeholder("https://example.com/").PlaceholderColor(p.textFaint).Label("Website address").ReadOnly(a.snap.Running).FocusRing(false)
		if in.Submitted() {
			a.start()
		}
		focused = in.Focused()
		a.controls(c, button)
	})
	p.card(box)
	if focused {
		box.Border(1, p.focus).Shadow(0, 0, 0, 3, p.focus.Alpha(0.22))
	}
}

// controls are the buttons of the crawl, button tall: the folder, and
// Start, or Pause and Stop while it runs.
func (a *app) controls(c *ui.Context, button float32) {
	p := a.pal
	snap := a.snap
	open := a.button(c, buttonGhost, iconFolderOpen, "", false).Size(button, button).Label("Open download folder").Tooltip(openFolderTip)
	if open.Clicked() {
		a.openFolder()
	}
	if !snap.Running {
		start := a.buttonBase(c, "start", buttonPrimary, false).Height(button).Padding(0, 18, 0, 15).Gap(7).Label("Start").
			Tooltip("Start (" + modKey + "↩)")
		start.Children(func() {
			ui.Icon(c, iconPlay).FontSize(12)
			ui.Text(c, "Start").SingleLine()
		})
		if start.Clicked() {
			a.start()
		}
		return
	}
	ic, label := iconPause, "Pause"
	if snap.Paused {
		ic, label = iconPlay, "Resume"
	}
	pause := a.buttonBase(c, "pause", buttonSecondary, snap.Stopping).Height(button).Padding(0, 14).Label(label).
		Tooltip(label + " (" + modKey + "P)")
	pause.Children(func() {
		ui.Icon(c, ic).FontSize(14)
		ui.Text(c, label).SingleLine()
	})
	if pause.Clicked() {
		a.togglePause()
	}
	stop := a.buttonBase(c, "stop", buttonDanger, snap.Stopping).Height(button).Padding(0, 14).Label("Stop").
		Tooltip("Stop (" + modKey + ".)")
	stop.Children(func() {
		if snap.Stopping {
			ui.Spinner(c).Size(13, 13).TextColor(p.danger)
			ui.Text(c, "Stopping…").SingleLine()
		} else {
			ui.Icon(c, iconStop).FontSize(10)
			ui.Text(c, "Stop").SingleLine()
		}
	})
	if stop.Clicked() {
		a.stop()
	}
}

type preflightItem struct {
	label, value string
	to           section
}

// preflight sums up the settings a crawl starts with.
func (a *app) preflight() []preflightItem {
	cfg := &a.cfg
	depth := "unlimited"
	if cfg.Limits.MaxDepthEnabled {
		depth = fmt.Sprintf("%d %s", cfg.Limits.MaxDepth, plural(int64(cfg.Limits.MaxDepth), "level", "levels"))
	}
	files := "unlimited"
	if cfg.Limits.MaxFilesEnabled {
		files = "up to " + formatInt(int64(cfg.Limits.MaxFiles))
	}
	folder := "not chosen"
	switch dest := strings.TrimSpace(cfg.Destination); {
	case cfg.AskDestination:
		folder = "ask on start"
	case dest != "":
		folder = tildify(dest)
	}
	return []preflightItem{
		{"Mode", labelOf(browserModes, cfg.Browser.Mode), secBrowser},
		{"Depth", depth, secCrawl},
		{"Files", files, secCrawl},
		{"Scope", labelOf(hostPolicies, cfg.Scope.HostPolicy), secCrawl},
		{"Proxy", labelOf(proxyTypes, cfg.Network.Proxy.Type), secNetwork},
		{"Folder", folder, secOutput},
	}
}

// preflightTile is a setting the crawl starts with, as a tile that goes
// to its page; it lifts under the pointer.
func (a *app) preflightTile(c *ui.Context, label, value string, to section) {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).Padding(12, 14, 13, 14).Gap(4).Column().MinWidth(0).AlignItems(ui.Stretch).
		Label(label + ": " + value).Tooltip(value)
	p.card(b)
	chevron := p.iconFaint
	if b.Hovered() {
		b.Background(p.layer1).Border(1, p.borderStrong)
		chevron = p.icon
	}
	b.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(11).TextColor(p.textFaint).Grow(1).SingleLine()
			ui.Icon(c, iconChevron).FontSize(13).TextColor(chevron)
		})
		ui.Text(c, value).FontWeight(weightMedium).SingleLine().MinWidth(0)
	})
	if b.Clicked() {
		a.section = to
	}
}

// meters are the counters of the crawl in a card: the files saved with
// the activity of the last minute, and four counters beside them.
func (a *app) meters(c *ui.Context) {
	p := a.pal
	stats := a.snap.Stats
	p.card(ui.Row(c.Key("meters"))).AlignItems(ui.Stretch).Shrink(0).Clip().Transition(rise).Children(func() {
		hero := ui.Column(c).Grow(1.3).Basis(0).MinWidth(0).Padding(16, 20, 14, 20).Gap(14).Justify(ui.SpaceBetween)
		saved := hero.AnimateTo("saved", float32(stats.Saved), countUp)
		hero.Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.End).Children(func() {
				ui.Column(c).Gap(4).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, "Files saved").FontSize(12).TextColor(p.textMuted)
					size := float32(36)
					if w, _ := c.Size(); w <= 1040 {
						size = 30
					}
					ui.Text(c, formatInt(int64(math.Round(float64(saved))))).FontSize(size).FontWeight(600).
						LineHeight(1).LetterSpacing(-0.5).FontFeatures("tnum").SingleLine()
				})
				elapsed, rate := "—", "—"
				if ms := a.elapsed(c); ms > 0 {
					elapsed = formatDuration(ms)
				}
				if a.snap.Running {
					rate = formatRate(a.activity.rate) + "/s"
				}
				a.heroFact(c, "Downloaded", formatBytes(stats.Bytes))
				a.heroFact(c, "Elapsed", elapsed)
				a.heroFact(c, "Rate", rate)
			})
			a.activityStrip(c)
		})
		ui.Box(c).Width(1).Shrink(0).Background(p.borderMuted)
		// The counters, with hairlines between them: the gaps of the grid
		// over the dividers' color.
		ui.Grid(c).Columns(2).Gap(1).Grow(1).Basis(0).MinWidth(0).Background(p.borderMuted).Children(func() {
			failed := p.iconFaint
			if stats.Failed > 0 {
				failed = p.danger
			}
			a.tile(c, "Discovered", stats.Discovered, p.iconMuted)
			a.tile(c, "Processed", stats.Processed, p.accent)
			a.tile(c, "Skipped", stats.Skipped, p.iconFaint)
			a.tile(c, "Errors", stats.Failed, failed)
		})
	})
}

// elapsed is how long the crawl has run, in milliseconds, which counts on
// while it runs.
func (a *app) elapsed(c *ui.Context) int64 {
	stats := a.snap.Stats
	if stats.StartedAtMs == 0 {
		return 0
	}
	end := stats.FinishedAtMs
	if end == 0 {
		now := c.Now()
		end = now.UnixMilli()
		if a.snap.Running {
			c.After(time.Until(now.Truncate(time.Second).Add(time.Second)))
		}
	}
	return max(0, end-stats.StartedAtMs)
}

func (a *app) heroFact(c *ui.Context, label, value string) {
	p := a.pal
	ui.Column(c).Gap(2).Shrink(0).MinWidth(64).Children(func() {
		ui.Text(c, label).FontSize(11).TextColor(p.textFaint).SingleLine()
		ui.Text(c, value).FontWeight(weightMedium).FontFeatures("tnum").SingleLine()
	})
}

func (a *app) tile(c *ui.Context, label string, value int64, tone ui.Color) {
	p := a.pal
	box := ui.Column(c.Key(label)).Padding(14, 18).MinWidth(0).Background(p.base).Justify(ui.Center)
	shown := box.AnimateTo("value", float32(value), ui.Animation{Duration: 450 * time.Millisecond, Ease: countUp.Ease})
	box.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(tone)
			ui.Text(c, label).FontSize(12).TextColor(p.textMuted)
		})
		ui.Text(c, formatInt(int64(math.Round(float64(shown))))).FontSize(20).FontWeight(weightMedium).
			LineHeight(1.1).FontFeatures("tnum").SingleLine().Margin(6, 0, 0, 0)
	})
}

// activityStrip charts the files saved in each of the last 60 seconds, in
// gray, with the second under way in the accent.
func (a *app) activityStrip(c *ui.Context) {
	p := a.pal
	act := &a.activity
	live := a.snap.Running && !a.snap.Paused
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "Saved in the last minute").FontSize(11).TextColor(p.textFaint).Grow(1)
			ui.Text(c, "peak "+formatInt(act.peak)+"/s").FontSize(11).FontFeatures("tnum").TextColor(p.textFaint)
		})
		ui.Row(c).Height(40).AlignItems(ui.Stretch).BorderWidth(0, 0, 1, 0).BorderColor(p.borderMuted).Children(func() {
			for i, v := range act.bars {
				slot := ui.Column(c.Key(i)).Grow(1).Basis(0).MinWidth(0).Justify(ui.End).AlignItems(ui.Center)
				if v >= 0 {
					when := "now"
					if i < activityBars-1 {
						when = fmt.Sprintf("%ds ago", activityBars-1-i)
					}
					slot.Tooltip(fmt.Sprintf("%s %s · %s", formatInt(v), plural(v, "file", "files"), when))
				}
				hovered := slot.Hovered()
				slot.Children(func() {
					if v <= 0 {
						return
					}
					col := p.layer4
					switch {
					case live && i == activityBars-1:
						col = p.accent
					case hovered:
						col = p.iconMuted
					}
					h := max(2, float32(v)/float32(act.peak)*36)
					ui.Box(c).Height(h).FillWidth().MaxWidth(20).Margin(0, 1).Radius(min(2, h), min(2, h), 0, 0).Background(col)
				})
			}
		})
	})
}

// progressMeter shows how much of what was found is processed, and what
// the crawl did last.
func (a *app) progressMeter(c *ui.Context) {
	p := a.pal
	stats := a.snap.Stats
	progress := float32(0)
	if stats.Discovered > 0 {
		progress = min(1, float32(stats.Processed)/float32(stats.Discovered))
	}
	ui.Column(c).Gap(8).Shrink(0).Padding(0, 4).Children(func() {
		ui.Row(c).FontSize(12).TextColor(p.textMuted).Children(func() {
			ui.RichText(c,
				ui.Span{Text: "Processed "},
				ui.Span{Text: formatInt(stats.Processed), Weight: weightMedium, Color: p.text},
				ui.Span{Text: " of " + formatInt(stats.Discovered) + " discovered"},
			).Grow(1)
			ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(float64(progress*100))))).FontFeatures("tnum")
		})
		track := ui.Box(c).Height(8).Radius(4).Background(p.layer2).Clip().Label("Processed").Role(ui.RoleProgress).Range(0, 100, float64(progress*100))
		shown := track.AnimateTo("progress", progress, ui.Animation{Duration: 400 * time.Millisecond, Ease: rise.Ease})
		accent := p.accent
		track.Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * shown, H: r.H}, accent, 4)
		})
		ui.Text(c, a.snap.Status).FontSize(12).TextColor(p.textFaint).Selectable().SingleLine().Tooltip(a.snap.Status)
	})
}
