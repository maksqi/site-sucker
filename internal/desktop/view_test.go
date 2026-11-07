package desktop

import (
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/maksqi/site-sucker/internal/config"
)

// allSections are the pages, each with a text only it shows.
var allSections = []struct {
	id   section
	text string
}{
	{secDashboard, "Download a website"},
	{secCrawl, "Maximum depth"},
	{secNetwork, "Parallel connections"},
	{secBrowser, "Always Chromium"},
	{secOutput, "Folder per host"},
	{secSettings, "Theme"},
}

func newTestApp(t *testing.T) (*app, *ui.Tester) {
	t.Helper()
	a := newApp(testSystem(nil, nil))
	tt := ui.NewTester(a.view, 1280, 860)
	tt.SetTitleBar(ui.TitleBar{Height: headerHeight, Left: 80})
	return a, tt
}

// testSystem is a system without dialogs, whose appearance and
// preferences go to theme and saved when they are not nil.
func testSystem(theme *[]mygo.ThemeSource, saved *[]prefs) system {
	return system{
		chooseFolder:  func(string, string) (string, error) { return "", nil },
		chooseBrowser: func() (string, error) { return "", nil },
		openProfile:   func() (string, error) { return "", nil },
		saveProfile:   func() (string, error) { return "", nil },
		openPath:      func(string) error { return nil },
		applyTheme: func(s mygo.ThemeSource) {
			if theme != nil {
				*theme = append(*theme, s)
			}
		},
		savePrefs: func(p prefs) error {
			if saved != nil {
				*saved = append(*saved, p)
			}
			return nil
		},
	}
}

// crawlUpdate is an update of a crawl under way, with a line of each level.
func crawlUpdate(now time.Time) Update {
	ms := now.UnixMilli()
	var samples []Sample
	for i := range 40 {
		samples = append(samples, Sample{TimeMs: ms - int64(40-i)*1000, Saved: int64(i*i) / 3})
	}
	return Update{
		State: Snapshot{
			Running: true, Status: "Saved: https://example.com/about/",
			Stats: Stats{Discovered: 412, Processed: 287, Saved: 263, Skipped: 17, Failed: 7, Bytes: 18_400_000, StartedAtMs: ms - 95_000},
		},
		Reset:   true,
		Samples: samples,
		Log: []LogEntry{
			{ID: 1, TimeMs: ms - 4000, Level: LevelInfo, URL: "https://example.com/", Message: "Starting"},
			{ID: 2, TimeMs: ms - 3000, Level: LevelSave, URL: "https://example.com/", Path: "/Users/ada/Downloads/site-sucker/example.com/index.html"},
			{ID: 3, TimeMs: ms - 2000, Level: LevelSkip, URL: "https://example.com/logout", Message: "disallowed by robots.txt"},
			{ID: 4, TimeMs: ms - 1000, Level: LevelError, URL: "https://example.com/missing.png", Message: "HTTP 404"},
			{ID: 5, TimeMs: ms, Level: LevelInfo, Message: "Chromium rendered https://example.com/app/"},
		},
	}
}

func TestSectionsRender(t *testing.T) {
	for _, dark := range []bool{false, true} {
		a, tt := newTestApp(t)
		tt.SetDark(dark)
		for _, s := range allSections {
			a.section = s.id
			tt.Frame()
			if !tt.HasText(s.text) {
				t.Errorf("dark %v, %s: no %q in %q", dark, s.id, s.text, tt.Texts())
			}
		}
	}
}

func TestSidebarNavigates(t *testing.T) {
	a, tt := newTestApp(t)
	for _, name := range []string{"Crawl", "Network", "Browser", "Output", "Settings", "Download"} {
		if err := tt.Click(name); err != nil {
			t.Fatal(err)
		}
	}
	if a.section != secDashboard {
		t.Fatalf("section %q", a.section)
	}
	if err := tt.Click("Crawl"); err != nil {
		t.Fatal(err)
	}
	if a.section != secCrawl || !tt.HasText("Which links to follow") {
		t.Fatalf("section %q, texts %q", a.section, tt.Texts())
	}
}

func TestStartNeedsAValidURL(t *testing.T) {
	a, tt := newTestApp(t)
	if err := tt.Click("Start"); err != nil {
		t.Fatal(err)
	}
	if a.snap.Running {
		t.Fatal("a crawl started without a URL")
	}
	if !tt.HasText("Could not start the download") || !tt.HasText("enter a valid start URL with http:// or https://") {
		t.Fatalf("no toast in %q", tt.Texts())
	}
}

func TestTypingTheURL(t *testing.T) {
	a, tt := newTestApp(t)
	if err := tt.Click("Website address"); err != nil {
		t.Fatal(err)
	}
	tt.Type("https://www.example.org/docs")
	if a.cfg.StartURL != "https://www.example.org/docs" {
		t.Fatalf("start URL %q", a.cfg.StartURL)
	}
	if !tt.HasText("www.example.org") {
		t.Fatalf("the dashboard shows no host: %q", tt.Texts())
	}
}

func TestProxyType(t *testing.T) {
	a, tt := newTestApp(t)
	a.section = secNetwork
	tt.Frame()
	// The type is a pop-up button, whose menu is the system's.
	if err := tt.Click("No proxy"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("SOCKS5"); err != nil {
		t.Fatalf("%v; the menu: %q", err, tt.Menu())
	}
	if a.cfg.Network.Proxy.Type != config.ProxySOCKS5 {
		t.Fatalf("proxy type %q", a.cfg.Network.Proxy.Type)
	}
	// The sidebar names the proxy's type.
	if n := strings.Count(strings.Join(tt.Texts(), "\n"), "SOCKS5"); n < 2 {
		t.Fatalf("SOCKS5 shows %d times", n)
	}
}

func TestAccentIsTheThemes(t *testing.T) {
	a, tt := newTestApp(t)
	// An orange accent chosen in System Settings does not show.
	tt.SetPreferences(ui.Preferences{Accent: ui.Hex("#f7821b")})
	want := ui.Hex("#3b5cf6")
	for _, dark := range []bool{false, true} {
		tt.SetDark(dark)
		tt.Frame()
		if a.pal.accent != want || a.theme.Accent != want || a.theme.InactiveAccent != want {
			t.Fatalf("dark %v: accent %v, theme's %v", dark, a.pal.accent, a.theme.Accent)
		}
	}
}

func TestThemeSwitch(t *testing.T) {
	var applied []mygo.ThemeSource
	var saved []prefs
	a := newApp(testSystem(&applied, &saved))
	tt := ui.NewTester(a.view, 1280, 860)
	a.section = secSettings
	tt.Frame()
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if a.prefs.Theme != mygo.ThemeDark || len(applied) != 1 || applied[0] != mygo.ThemeDark {
		t.Fatalf("theme %q, applied %q", a.prefs.Theme, applied)
	}
	if err := tt.Click("Auto"); err != nil {
		t.Fatal(err)
	}
	zoom := defaultPrefs().Zoom
	if want := []prefs{{mygo.ThemeDark, zoom}, {mygo.ThemeSystem, zoom}}; !slices.Equal(saved, want) {
		t.Fatalf("saved %v, want %v", saved, want)
	}
	// The same choice again changes nothing.
	if err := tt.Click("Auto"); err != nil {
		t.Fatal(err)
	}
	if len(applied) != 2 {
		t.Fatalf("applied %q", applied)
	}
}

func TestInterfaceSize(t *testing.T) {
	var saved []prefs
	a := newApp(testSystem(nil, &saved))
	tt := ui.NewTester(a.view, 1280, 860)
	a.section = secSettings
	tt.Frame()
	if a.prefs.Zoom != 1.15 {
		t.Fatalf("default zoom %v", a.prefs.Zoom)
	}
	if err := tt.Click("Large"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	if a.prefs.Zoom != 1.3 || len(saved) != 1 || saved[0].Zoom != 1.3 {
		t.Fatalf("zoom %v, saved %v", a.prefs.Zoom, saved)
	}
}

func TestPrefsRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs", "preferences.json")
	if p, err := readPrefs(path); err != nil || p != defaultPrefs() {
		t.Fatalf("no file: %v, %v", p, err)
	}
	if err := writePrefs(path, prefs{Theme: mygo.ThemeLight, Zoom: 1.3}); err != nil {
		t.Fatal(err)
	}
	if p, err := readPrefs(path); err != nil || p != (prefs{mygo.ThemeLight, 1.3}) {
		t.Fatalf("read back %v, %v", p, err)
	}
	// What is unknown, or missing as in the preferences of earlier
	// versions, reads as the default.
	if err := os.WriteFile(path, []byte(`{"theme":"sepia","zoom":7}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, _ := readPrefs(path); p != defaultPrefs() {
		t.Fatalf("unknown values read as %v", p)
	}
	if err := os.WriteFile(path, []byte(`{"theme":"dark"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, _ := readPrefs(path); p != (prefs{mygo.ThemeDark, defaultPrefs().Zoom}) {
		t.Fatalf("no zoom reads as %v", p)
	}
}

func TestNumberField(t *testing.T) {
	a := newApp(testSystem(nil, nil))
	count := 10
	tt := ui.NewTester(func(c *ui.Context) {
		ui.Column(c).Padding(20).Children(func() {
			numberField(a, c, "count", &count, numberOptions{lo: 1, hi: 100, label: "Count"})
		})
	}, 300, 100)
	if err := tt.Click("Count"); err != nil {
		t.Fatal(err)
	}
	tt.Key(0, ui.KeyUp)
	if count != 11 {
		t.Fatalf("count %d after Up", count)
	}
	tt.Key(ui.Shift, ui.KeyDown)
	if count != 1 {
		t.Fatalf("count %d after Shift+Down", count)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("42")
	if count != 42 {
		t.Fatalf("count %d after typing", count)
	}
	tt.Type("0")
	if count != 42 {
		t.Fatalf("count %d after typing past the range", count)
	}
}

func TestLogShowsFiltersAndCopies(t *testing.T) {
	a, tt := newTestApp(t)
	a.post(func() { a.apply(crawlUpdate(time.Now())) })
	tt.Frame()
	for _, want := range []string{"Files saved", "Downloading", "https://example.com/missing.png", "HTTP 404", "→ "} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
	if err := tt.Click("Info"); err != nil {
		t.Fatal(err)
	}
	if len(a.logShown) != 2 || tt.HasText("https://example.com/logout") {
		t.Fatalf("info filter shows %d lines", len(a.logShown))
	}
	a.logFilter = LevelError
	tt.Frame()
	if err := tt.Click("Copy shown lines"); err != nil {
		t.Fatal(err)
	}
	if got := tt.Clipboard(); !strings.Contains(got, "ERROR  https://example.com/missing.png | HTTP 404") {
		t.Fatalf("clipboard %q", got)
	}
	if err := tt.Click("All"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("Search the log"); err != nil {
		t.Fatal(err)
	}
	tt.Type("ROBOTS")
	if len(a.logShown) != 1 || a.entry(a.logShown[0]).ID != 3 {
		t.Fatalf("search shows %v", a.logShown)
	}
}

func TestFinishShowsAToast(t *testing.T) {
	a, tt := newTestApp(t)
	now := time.Now()
	a.post(func() { a.apply(crawlUpdate(now)) })
	a.post(func() {
		u := crawlUpdate(now)
		u.Reset, u.State.Running = false, false
		u.State.Stats.FinishedAtMs = now.UnixMilli()
		u.Log = []LogEntry{{ID: 6, TimeMs: now.UnixMilli(), Level: LevelDone, Message: "Crawl finished"}}
		a.apply(u)
	})
	tt.Frame()
	for _, want := range []string{"Download finished", "263 files · 17.5 MB", "Open folder", "Finished"} {
		if !tt.HasText(want) {
			t.Errorf("no %q in %q", want, tt.Texts())
		}
	}
}

func TestLogKeepsTheNewest(t *testing.T) {
	a := newApp(testSystem(nil, nil))
	a.logFilter = LevelError
	a.shownFilter = LevelError
	var entries []LogEntry
	for i := range logLimit + 250 {
		level := LevelSave
		if i%10 == 0 {
			level = LevelError
		}
		entries = append(entries, LogEntry{ID: int64(i + 1), Level: level})
	}
	a.appendLog(entries[:100])
	a.appendLog(entries[100:])
	if len(a.log) != logLimit || a.log[0].ID != 251 {
		t.Fatalf("kept %d lines from %d", len(a.log), a.log[0].ID)
	}
	if a.logCounts[levelSlot(LevelError)] != logLimit/10 || len(a.logShown) != logLimit/10 {
		t.Fatalf("errors: %d counted, %d shown", a.logCounts[levelSlot(LevelError)], len(a.logShown))
	}
	if first := a.entry(a.logShown[0]); first.ID != 251 || first.Level != LevelError {
		t.Fatalf("first shown %+v", first)
	}
}

// TestSnapshots draws every page into SITE_SUCKER_SNAPSHOTS, to look at.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("SITE_SUCKER_SNAPSHOTS")
	if dir == "" {
		t.Skip("set SITE_SUCKER_SNAPSHOTS to a folder")
	}
	save := func(tt *ui.Tester, name string) {
		tt.Advance(2 * time.Second)
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	for _, dark := range []bool{false, true} {
		theme := map[bool]string{false: "light", true: "dark"}[dark]
		a, tt := newTestApp(t)
		tt.SetDark(dark)
		for _, s := range allSections {
			a.section = s.id
			tt.Frame()
			save(tt, fmt.Sprintf("%s-%s", theme, s.id))
		}
		a.section = secDashboard
		a.cfg.StartURL = "https://example.com/"
		a.post(func() { a.apply(crawlUpdate(time.Now())) })
		tt.Frame()
		save(tt, theme+"-running")
	}
}

// TestScreenshots draws the README's screenshots at twice the size, into
// SITE_SUCKER_SCREENSHOTS (docs/screenshots):
//
//	SITE_SUCKER_SCREENSHOTS=$PWD/docs/screenshots go test ./internal/desktop -run Screenshots
func TestScreenshots(t *testing.T) {
	dir := os.Getenv("SITE_SUCKER_SCREENSHOTS")
	if dir == "" {
		t.Skip("set SITE_SUCKER_SCREENSHOTS to a folder")
	}
	shoot := func(name string, dark bool, setup func(a *app)) {
		a := newApp(testSystem(nil, nil))
		a.cfg.Destination = "/Users/ada/Downloads/site-sucker"
		a.version = "1.0.0"
		tt := ui.NewTester(a.view, 1280, 820)
		tt.SetScale(2)
		tt.SetDark(dark)
		tt.SetTitleBar(ui.TitleBar{Height: headerHeight, Left: 80})
		setup(a)
		tt.Frame()
		tt.Advance(2 * time.Second)
		f, err := os.Create(filepath.Join(dir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
	}
	shoot("download", false, func(a *app) { a.cfg.StartURL = "https://example.com/docs/" })
	shoot("crawl-dark", true, func(a *app) {
		a.cfg.StartURL = "https://example.com/docs/"
		a.post(func() { a.apply(screenshotUpdate(time.Now())) })
	})
	shoot("crawl-settings", false, func(a *app) { a.section = secCrawl })
}

// screenshotUpdate is a crawl well under way, with a log of the kinds of
// lines a real one shows.
func screenshotUpdate(now time.Time) Update {
	u := crawlUpdate(now)
	u.State.Status = "Saved: https://example.com/docs/guides/install/"
	u.State.Stats = Stats{Discovered: 1284, Processed: 917, Saved: 862, Skipped: 41, Failed: 3, Bytes: 48_700_000, StartedAtMs: now.UnixMilli() - 154_000}
	ms := now.UnixMilli()
	paths := []string{
		"docs/", "docs/getting-started/", "assets/app.css", "assets/app.js", "docs/guides/", "images/hero.webp",
		"docs/api/", "images/old-logo.png", "docs/guides/install/",
	}
	u.Log = u.Log[:0]
	for i, path := range paths {
		url := "https://example.com/" + path
		e := LogEntry{ID: int64(i + 1), TimeMs: ms - int64(len(paths)-i)*700, Level: LevelSave, URL: url,
			Path: "/Users/ada/Downloads/site-sucker/example.com/" + strings.TrimSuffix(path, "/") + map[bool]string{true: "/index.html", false: ""}[strings.HasSuffix(path, "/")]}
		switch i {
		case 4:
			e.Level, e.Path, e.Message = LevelSkip, "", "disallowed by robots.txt"
			e.URL = "https://example.com/admin/"
		case 7:
			e.Level, e.Path, e.Message = LevelError, "", "HTTP 404"
		}
		u.Log = append(u.Log, e)
	}
	return u
}
