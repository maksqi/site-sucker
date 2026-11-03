package desktop

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/maksqi/site-sucker/internal/config"
)

// logLimit is how many log lines the window keeps.
const logLimit = 5000

// section is a page of the window.
type section string

const (
	secDashboard section = "dashboard"
	secLimits    section = "limits"
	secNetwork   section = "network"
	secChromium  section = "chromium"
	secScope     section = "scope"
	secProxy     section = "proxy"
	secAuth      section = "auth"
	secOutput    section = "output"
)

// system is what the app asks of the operating system: dialogs and the file
// manager. The dialogs block, so they are called on goroutines; each
// returns "" when the user cancels.
type system struct {
	chooseFolder  func(title, current string) (string, error)
	chooseBrowser func() (string, error)
	openProfile   func() (string, error)
	saveProfile   func() (string, error)
	openPath      func(path string) error
}

// app is the state of the window, which its view shows. It lives on the
// main thread: goroutines hand it changes through post.
type app struct {
	win    *mygo.Window // nil in tests
	engine *engine
	sys    system

	mu    sync.Mutex
	queue []func()

	cfg         config.Config
	headersText string
	section     section
	snap        Snapshot
	activity    activityData

	// The log: its entries, oldest first, with logBase older ones dropped;
	// the count of each level; and the indices of the entries the filter
	// and the search show, counted from the first entry ever added.
	log         []LogEntry
	logBase     int
	logCounts   [levelCount]int
	logShown    []int
	logFilter   LogLevel
	logQuery    string
	shownFilter LogLevel
	shownQuery  string
	logList     ui.ListState

	geo           *ProxyProfile
	geoKey        config.Proxy
	checkingProxy bool

	toasts   []ui.Toast
	toastSeq int

	showProxyPassword bool
	showAuthPassword  bool

	pal       *palette
	theme     ui.Theme
	sideTheme ui.Theme // the sidebar's
}

func newApp(sys system) *app {
	cfg := config.Default()
	a := &app{
		sys:         sys,
		cfg:         cfg,
		headersText: formatHeaders(cfg.Network.Headers),
		section:     secDashboard,
		snap:        Snapshot{Status: "Ready to start", Browser: BrowserClosed},
		activity:    activity(nil),
		pal:         paletteFor(false),
	}
	a.engine = newEngine(func(u Update) { a.post(func() { a.apply(u) }) })
	a.logList.FollowEnd = true
	a.logList.Key = func(i int) any { return a.entry(a.logShown[i]).ID }
	return a
}

// post runs fn on the main thread, before the next frame is built.
func (a *app) post(fn func()) {
	a.mu.Lock()
	a.queue = append(a.queue, fn)
	a.mu.Unlock()
	if a.win != nil {
		a.win.Invalidate()
	}
}

// drain runs what goroutines posted.
func (a *app) drain() {
	a.mu.Lock()
	queue := a.queue
	a.queue = nil
	a.mu.Unlock()
	for _, fn := range queue {
		fn()
	}
}

// apply takes an update of the engine.
func (a *app) apply(u Update) {
	a.snap = u.State
	if u.Samples != nil {
		a.activity = activity(u.Samples)
	}
	if u.Reset {
		a.clearLog()
	}
	a.appendLog(u.Log)
	for _, e := range u.Log {
		if e.Level != LevelDone {
			continue
		}
		stats := a.snap.Stats
		message := fmt.Sprintf("%s %s · %s", formatInt(stats.Saved), plural(stats.Saved, "file", "files"), formatBytes(stats.Bytes))
		if e.Message == "Crawl finished" {
			a.toastAction(NoticeSuccess, "Download finished", message, "Open folder", a.openFolder)
		} else {
			a.toastAction(NoticeInfo, "Download stopped", message, "Open folder", a.openFolder)
		}
	}
	if n := u.Notice; n != nil {
		a.toast(n.Kind, n.Title, n.Message)
	}
}

// hasRun reports whether the dashboard shows a crawl rather than its
// welcome.
func (a *app) hasRun() bool {
	return a.snap.Running || a.snap.Stats.StartedAtMs > 0 || len(a.log) > 0
}

// The log.

const levelCount = 4

// levelSlot is where a level counts: done lines count as info.
func levelSlot(l LogLevel) int {
	switch l {
	case LevelSave:
		return 0
	case LevelSkip:
		return 1
	case LevelError:
		return 2
	}
	return 3
}

func (a *app) entry(i int) *LogEntry { return &a.log[i-a.logBase] }

func (a *app) appendLog(entries []LogEntry) {
	query := strings.ToLower(strings.TrimSpace(a.logQuery))
	for _, e := range entries {
		a.log = append(a.log, e)
		a.logCounts[levelSlot(e.Level)]++
		if matchesLog(e, a.logFilter, query) {
			a.logShown = append(a.logShown, a.logBase+len(a.log)-1)
		}
	}
	if over := len(a.log) - logLimit; over > 0 {
		for _, e := range a.log[:over] {
			a.logCounts[levelSlot(e.Level)]--
		}
		a.log = a.log[over:]
		a.logBase += over
		first, _ := slices.BinarySearch(a.logShown, a.logBase)
		a.logShown = a.logShown[first:]
	}
}

func (a *app) clearLog() {
	a.logBase += len(a.log)
	a.log, a.logShown = nil, nil
	a.logCounts = [levelCount]int{}
}

// filterLog finds the lines to show again once the filter or the search
// changed.
func (a *app) filterLog() {
	if a.logFilter == a.shownFilter && a.logQuery == a.shownQuery {
		return
	}
	a.shownFilter, a.shownQuery = a.logFilter, a.logQuery
	query := strings.ToLower(strings.TrimSpace(a.logQuery))
	a.logShown = a.logShown[:0]
	for i, e := range a.log {
		if matchesLog(e, a.logFilter, query) {
			a.logShown = append(a.logShown, a.logBase+i)
		}
	}
}

func matchesLog(e LogEntry, filter LogLevel, query string) bool {
	if filter != "" && levelSlot(filter) != levelSlot(e.Level) {
		return false
	}
	if query == "" {
		return true
	}
	return strings.Contains(strings.ToLower(e.URL), query) ||
		strings.Contains(strings.ToLower(e.Path), query) ||
		strings.Contains(strings.ToLower(e.Message), query)
}

// copyText is the lines shown, as copied to the clipboard.
func (a *app) copyText() string {
	lines := make([]string, len(a.logShown))
	for i, abs := range a.logShown {
		lines[i] = logLine(*a.entry(abs))
	}
	return strings.Join(lines, "\n")
}

// Toasts.

func (a *app) toast(kind NoticeKind, title, message string) {
	a.toastAction(kind, title, message, "", nil)
}

func (a *app) toastAction(kind NoticeKind, title, message, action string, fn func()) {
	a.toastSeq++
	timeout := 5500 * time.Millisecond
	if kind == NoticeError {
		timeout = 9 * time.Second
	}
	a.toasts = append(a.toasts, ui.Toast{
		ID:          fmt.Sprintf("toast-%d", a.toastSeq),
		Title:       title,
		Description: message,
		Type:        string(kind),
		Timeout:     timeout,
		Action:      action,
		OnAction:    fn,
	})
}

func (a *app) fail(title string, err error) {
	if err != nil {
		a.toast(NoticeError, title, err.Error())
	}
}

// Actions. They run on the main thread, from the view and the menus, and
// hand blocking work to goroutines.

func (a *app) start() {
	if a.snap.Running {
		return
	}
	const title = "Could not start the download"
	cfg, err := collectConfig(a.cfg, a.headersText, true)
	if err == nil {
		err = cfg.Validate()
	}
	if err != nil {
		a.fail(title, err)
		return
	}
	if !cfg.AskDestination {
		a.launch(cfg)
		return
	}
	go func() {
		path, err := a.sys.chooseFolder("Folder for the Site Copy", cfg.Destination)
		a.post(func() {
			if err != nil || path == "" {
				a.fail(title, err)
				return
			}
			cfg.Destination = path
			a.cfg.Destination = path
			a.launch(cfg)
		})
	}()
}

func (a *app) launch(cfg config.Config) {
	if err := a.engine.Start(cfg); err != nil {
		a.fail("Could not start the download", err)
		return
	}
	a.cfg.Destination = cfg.Destination
	a.section = secDashboard
}

func (a *app) togglePause() { a.engine.TogglePause() }

func (a *app) stop() { a.engine.Stop() }

func (a *app) chooseFolder() {
	current := a.cfg.Destination
	go func() {
		path, err := a.sys.chooseFolder("Folder for Site Copies", current)
		a.post(func() {
			if err != nil {
				a.fail("Could not choose the folder", err)
			} else if path != "" {
				a.cfg.Destination = path
			}
		})
	}()
}

func (a *app) chooseBrowser() {
	go func() {
		path, err := a.sys.chooseBrowser()
		a.post(func() {
			if err != nil {
				a.fail("Could not choose the browser", err)
			} else if path != "" {
				a.cfg.Browser.ExecutablePath = resolveAppBundle(path)
			}
		})
	}()
}

func (a *app) openFolder() {
	path, err := checkFolder(a.cfg.Destination)
	if err != nil {
		a.fail("Could not open the folder", err)
		return
	}
	go func() {
		if err := a.sys.openPath(path); err != nil {
			a.post(func() { a.fail("Could not open the folder", fmt.Errorf("open folder: %w", err)) })
		}
	}()
}

func (a *app) importProfile() {
	go func() {
		path, err := a.sys.openProfile()
		if err != nil || path == "" {
			a.post(func() { a.fail("Import failed", err) })
			return
		}
		cfg, err := readProfile(path)
		a.post(func() {
			if err != nil {
				a.fail("Import failed", err)
				return
			}
			a.cfg = cfg
			a.headersText = formatHeaders(cfg.Network.Headers)
			a.geo = nil
			a.engine.SetStatus("Profile imported")
			a.toast(NoticeSuccess, "Profile imported", cfg.StartURL)
		})
	}()
}

func (a *app) exportProfile() {
	const title = "Export failed"
	cfg, err := collectConfig(a.cfg, a.headersText, true)
	if err == nil {
		err = cfg.Validate()
	}
	if err != nil {
		a.fail(title, err)
		return
	}
	go func() {
		path, err := a.sys.saveProfile()
		if err == nil && path != "" {
			err = writeProfile(path, cfg)
		}
		a.post(func() {
			switch {
			case err != nil:
				a.fail(title, err)
			case path != "":
				a.engine.SetStatus("Profile exported")
				a.toast(NoticeSuccess, "Profile exported", "Passwords in the profile are stored as plain text.")
			}
		})
	}()
}

func (a *app) checkProxy() {
	if a.checkingProxy {
		return
	}
	cfg, _ := collectConfig(a.cfg, a.headersText, false)
	key := a.cfg.Network.Proxy
	a.checkingProxy = true
	go func() {
		profile, err := a.engine.CheckProxy(context.Background(), cfg)
		a.post(func() {
			a.checkingProxy = false
			if err != nil {
				a.fail("Proxy check failed", err)
				return
			}
			a.rememberGeo(profile, key, true)
			a.toast(NoticeSuccess, "Proxy works", strings.TrimSpace(fmt.Sprintf("%s %s · %s", profile.Flag, profile.Country, profile.IP)))
		})
	}()
}

func (a *app) openBrowser() {
	cfg, _ := collectConfig(a.cfg, a.headersText, false)
	key := a.cfg.Network.Proxy
	go func() {
		info, err := a.engine.OpenBrowser(cfg)
		a.post(func() {
			if err != nil {
				a.fail("Could not start Chromium", err)
				return
			}
			if !info.Opened {
				return
			}
			if info.Profile != nil {
				a.rememberGeo(*info.Profile, key, false)
			}
			if info.Warning != "" {
				a.toast(NoticeWarning, "Chromium", info.Warning)
			}
		})
	}()
}

func (a *app) closeBrowser() { a.engine.CloseBrowser() }

// rememberGeo keeps the geo profile of a proxy, and with applyLocale fills
// in Chromium's locale and time zone from it.
func (a *app) rememberGeo(profile ProxyProfile, key config.Proxy, applyLocale bool) {
	a.geo, a.geoKey = &profile, key
	if applyLocale && a.cfg.Browser.AutoGeo {
		a.cfg.Browser.Locale = profile.Locale
		a.cfg.Browser.Timezone = profile.Timezone
	}
}

// geoStale reports whether the proxy settings changed since the geo
// profile was looked up.
func (a *app) geoStale() bool {
	return a.geo != nil && a.cfg.Network.Proxy != a.geoKey
}
