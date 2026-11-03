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
	rise    = ui.ElementTransition{Duration: 350 * time.Millisecond, Ease: ui.CubicBezier(0.22, 1, 0.36, 1), Enter: &ui.Motion{Y: 6}}
)

func (a *app) dashboard(c *ui.Context) {
	if !a.hasRun() {
		a.welcome(c)
		return
	}
	ui.Column(c).Fill().Gap(12).Padding(barHeight+6, 20, 16, 20).Children(func() {
		a.meters(c)
		a.progressMeter(c)
		a.logView(c)
	})
}

// phase is what the toolbar says the crawl is doing.
func (a *app) phase() (string, ui.Color, pulse) {
	p, snap := a.pal, a.snap
	switch {
	case snap.Stopping:
		return "Stopping", p.yellow, still
	case snap.Paused:
		return "Paused", p.yellow, still
	case snap.Running:
		return "Downloading", p.accent, beacon
	case snap.Stats.FinishedAtMs != 0:
		return "Finished", p.green, still
	}
	return "Ready to start", p.label3, still
}

// welcome is the dashboard before the first crawl, as SwiftUI's
// ContentUnavailableView: what the app does, and the settings it would
// start with.
func (a *app) welcome(c *ui.Context) {
	p := a.pal
	// The content stands in the middle of the room under the toolbar, and
	// scrolls in a window too short for it.
	_, h := c.Size()
	ui.Scroll(c).Fill().Padding(barHeight+12, 24, 24, 24).ScrollbarInsets(barHeight, 0, 0, 0).Children(func() {
		ui.Column(c).Key("welcome").FillWidth().MaxWidth(460).MinHeight(max(0, h-statusHeight-barHeight-36)).AlignSelf(ui.Center).
			AlignItems(ui.Center).Justify(ui.Center).Transition(rise).Children(func() {
			ui.Icon(c, iconGlobe).FontSize(54).TextColor(p.label3)
			ui.Text(c, "A whole website, offline").FontSize(20).FontWeight(700).TextAlign(ui.Center).Margin(14, 0, 0, 0)
			ui.Text(c, "Paste an address into the bar above and press Start. Pages, styles, scripts, images and fonts are saved with local links, so the copy opens without the internet.").
				LineHeight(1.45).TextColor(p.label2).TextAlign(ui.Center).Margin(6, 0, 0, 0)
			ui.Column(c).FillWidth().Margin(26, 0, 0, 0).Children(func() {
				a.group(c, "Starts with", "", func() {
					for _, item := range a.preflight() {
						a.preflightItem(c, item.label, item.value, item.to)
					}
				})
			})
			ui.Text(c, "Press "+modKey+"↩ to start the download").FontSize(11.5).TextColor(p.label2)
		})
	})
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
		{"Mode", labelOf(browserModes, cfg.Browser.Mode), secChromium},
		{"Depth", depth, secLimits},
		{"Files", files, secLimits},
		{"Scope", labelOf(hostPolicies, cfg.Scope.HostPolicy), secScope},
		{"Proxy", labelOf(proxyTypes, cfg.Network.Proxy.Type), secProxy},
		{"Folder", folder, secOutput},
	}
}

// preflightItem is a row that goes to the setting's section, as a
// NavigationLink in a form.
func (a *app) preflightItem(c *ui.Context, label, value string, to section) {
	p := a.pal
	b := ui.ButtonBase(c.Key(label)).MinHeight(36).Padding(8, 0).Gap(8).Label(label + ": " + value)
	if b.Pressed() {
		b.Opacity(0.6)
	}
	b.Children(func() {
		ui.Text(c, label).Shrink(0)
		ui.Text(c, value).TextColor(p.label2).TextAlign(ui.End).SingleLine().Grow(1).MinWidth(0)
		ui.Icon(c, iconChevron).FontSize(13).TextColor(p.label3)
	})
	if b.Clicked() {
		a.section = to
	}
}

// meters are the counters of the crawl: the files saved with the activity
// of the last minute, and four tiles.
func (a *app) meters(c *ui.Context) {
	p := a.pal
	stats := a.snap.Stats
	ui.Row(c).Key("meters").Gap(12).AlignItems(ui.Stretch).Shrink(0).Transition(rise).Children(func() {
		hero := p.groupBox(ui.Column(c)).Grow(1.25).Basis(0).MinWidth(0).Padding(14, 16, 12, 16).Gap(12).Justify(ui.SpaceBetween)
		saved := hero.AnimateTo("saved", float32(stats.Saved), countUp)
		hero.Children(func() {
			ui.Row(c).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Gap(2).Grow(1).MinWidth(0).Children(func() {
					ui.Text(c, "Files saved").FontSize(12).TextColor(p.label2)
					size := float32(44)
					if w, _ := c.Size(); w <= 1080 {
						size = 38
					}
					ui.Text(c, formatInt(int64(math.Round(float64(saved))))).FontSize(size).FontWeight(700).
						LineHeight(1).FontFeatures("tnum").SingleLine()
				})
				ui.Column(c).Gap(5).Margin(2, 0, 0, 0).Shrink(0).Children(func() {
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
			})
			a.activityStrip(c)
		})
		ui.Grid(c).Columns(2).Gap(12).Grow(1).Basis(0).MinWidth(0).Children(func() {
			failed := p.gray
			if stats.Failed > 0 {
				failed = p.red
			}
			a.tile(c, "Discovered", stats.Discovered, p.blue)
			a.tile(c, "Processed", stats.Processed, p.indigo)
			a.tile(c, "Skipped", stats.Skipped, p.gray)
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
	ui.Row(c).Gap(10).Justify(ui.End).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(p.label2).SingleLine()
		ui.Text(c, value).FontWeight(600).FontFeatures("tnum").MinWidth(70).TextAlign(ui.End).SingleLine()
	})
}

func (a *app) tile(c *ui.Context, label string, value int64, tone ui.Color) {
	p := a.pal
	box := p.groupBox(ui.Column(c.Key(label))).Padding(12, 14, 13, 14).MinWidth(0)
	shown := box.AnimateTo("value", float32(value), ui.Animation{Duration: 450 * time.Millisecond, Ease: countUp.Ease})
	box.Children(func() {
		ui.Row(c).Gap(6).Children(func() {
			ui.Box(c).Size(7, 7).Radius(3.5).Background(tone)
			ui.Text(c, label).FontSize(12).TextColor(p.label2)
		})
		ui.Text(c, formatInt(int64(math.Round(float64(shown))))).FontSize(22).FontWeight(600).
			LineHeight(1.1).FontFeatures("tnum").SingleLine().Margin(6, 0, 0, 0)
	})
}

// activityStrip charts the files saved in each of the last 60 seconds.
func (a *app) activityStrip(c *ui.Context) {
	p := a.pal
	act := &a.activity
	live := a.snap.Running && !a.snap.Paused
	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, "Saved in the last minute").FontSize(11).TextColor(p.label2).Grow(1)
			ui.Text(c, "peak "+formatInt(act.peak)+"/s").FontSize(11).FontFeatures("tnum").TextColor(p.label2)
		})
		ui.Row(c).Height(44).AlignItems(ui.Stretch).BorderWidth(0, 0, 1, 0).BorderColor(p.separator).Children(func() {
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
					col := p.accent.Alpha(0.4)
					if hovered || (live && i == activityBars-1) {
						col = p.accent
					}
					h := max(2, float32(v)/float32(act.peak)*40)
					ui.Box(c).Height(h).FillWidth().MaxWidth(24).Margin(0, 1).Radius(min(3, h), min(3, h), 0, 0).Background(col)
				})
			}
		})
	})
}

// progressMeter shows how much of what was found is processed, as a bar
// of AppKit's.
func (a *app) progressMeter(c *ui.Context) {
	p := a.pal
	stats := a.snap.Stats
	progress := float32(0)
	if stats.Discovered > 0 {
		progress = min(1, float32(stats.Processed)/float32(stats.Discovered))
	}
	ui.Column(c).Gap(7).Shrink(0).Padding(0, 2).Children(func() {
		ui.Row(c).FontSize(12).TextColor(p.label2).Children(func() {
			ui.RichText(c,
				ui.Span{Text: "Processed "},
				ui.Span{Text: formatInt(stats.Processed), Weight: 600, Color: p.label},
				ui.Span{Text: " of " + formatInt(stats.Discovered) + " discovered"},
			).Grow(1)
			ui.Text(c, fmt.Sprintf("%d%%", int(math.Round(float64(progress*100))))).FontFeatures("tnum")
		})
		track := ui.Box(c).Height(6).Radius(3).Background(p.fill).Clip().Label("Processed").Role(ui.RoleProgress).Range(0, 100, float64(progress*100))
		shown := track.AnimateTo("progress", progress, ui.Animation{Duration: 400 * time.Millisecond, Ease: rise.Ease})
		accent := p.accent
		track.Draw(func(pt *ui.Painter, r ui.Rect) {
			pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * shown, H: r.H}, accent, 3)
		})
	})
}
