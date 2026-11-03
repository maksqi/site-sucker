package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"

	"github.com/maksqi/site-sucker/internal/config"
	"github.com/maksqi/site-sucker/internal/crawler"
)

const (
	flushInterval  = 100 * time.Millisecond
	sampleInterval = time.Second
	// pendingLimit bounds the log lines of a single update; sampleLimit is
	// a minute of samples and the one before it.
	pendingLimit = 2000
	sampleLimit  = 61
)

// engine runs the crawl and the interactive browser and keeps their state.
// Its methods are safe from any goroutine; it reports every change through
// sink, at most every flushInterval while a crawl runs.
type engine struct {
	// sendMu orders updates: an update is built and handed to sink under
	// it, so the window never sees an older state after a newer one.
	sendMu sync.Mutex
	sink   func(Update)

	mu       sync.Mutex
	window   *mygo.Window
	running  bool
	paused   bool
	stopping bool
	finished bool
	control  *crawler.Control
	cancel   context.CancelFunc
	release  func()
	stats    Stats
	status   string
	nextID   int64
	pending  []LogEntry
	samples  []Sample
	notice   *Notice
	reset    bool
	dirty    bool

	browser        BrowserState
	browserSession *crawler.BrowserSession
	browserCancel  context.CancelFunc
}

func newEngine(sink func(Update)) *engine {
	return &engine{sink: sink, status: "Ready to start", browser: BrowserClosed}
}

// Start validates cfg and starts the crawl in the background.
func (e *engine) Start(cfg config.Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return errors.New("a download is already running")
	}
	ctx, cancel := context.WithCancel(context.Background())
	control := crawler.NewControl()
	e.running, e.paused, e.stopping, e.finished = true, false, false, false
	e.control, e.cancel = control, cancel
	e.stats = Stats{}
	e.status = "Preparing download..."
	e.pending, e.reset = nil, true
	e.samples = []Sample{{TimeMs: time.Now().UnixMilli()}}
	e.appendLocked(LogEntry{Level: LevelInfo, URL: cfg.StartURL, Message: "Starting"})
	window := e.window
	e.mu.Unlock()

	release := mygo.Power.KeepAwake("Downloading a website", false)
	e.mu.Lock()
	e.release = release
	e.mu.Unlock()
	if window != nil {
		window.SetProgressBar(mygo.ProgressBar{State: mygo.ProgressIndeterminate})
	}
	e.publish()

	done := make(chan struct{})
	go e.flushLoop(done)
	go func() {
		_, err := crawler.Run(ctx, cfg, control, e.onEvent)
		cancel()
		e.finishRun(err)
		close(done)
	}()
	return nil
}

// finishRun resets the state after crawler.Run returned. Errors before the
// crawl started, which no EventFinished reports, come back as a notice.
func (e *engine) finishRun(err error) {
	e.mu.Lock()
	if !e.finished {
		switch {
		case err != nil && !errors.Is(err, context.Canceled):
			e.status = "Could not start the download"
			e.appendLocked(LogEntry{Level: LevelError, Message: err.Error()})
			e.notice = &Notice{Kind: NoticeError, Title: "Could not start the download", Message: err.Error()}
		case err != nil:
			e.status = "Download stopped"
		}
	}
	e.running, e.paused, e.stopping = false, false, false
	e.control, e.cancel = nil, nil
	e.sampleLocked(time.Now())
	release, window := e.release, e.window
	e.release = nil
	e.dirty = true
	e.mu.Unlock()

	if release != nil {
		release()
	}
	if window != nil {
		window.SetProgressBar(mygo.ProgressBar{})
	}
}

func (e *engine) onEvent(event crawler.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	stats := statsFrom(event.Stats)
	if stats.StartedAtMs == 0 {
		stats.StartedAtMs = e.stats.StartedAtMs
	}
	e.stats = stats
	if status, ok := statusFor(event); ok {
		e.status = status
	}
	if entry, ok := logEntryFor(event); ok {
		e.appendLocked(entry)
	}
	if event.Type == crawler.EventFinished {
		e.finished = true
	}
	e.dirty = true
}

func (e *engine) flushLoop(done <-chan struct{}) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			e.publish()
			return
		case now := <-ticker.C:
			e.mu.Lock()
			if last := e.samples[len(e.samples)-1]; now.UnixMilli()-last.TimeMs >= sampleInterval.Milliseconds() {
				e.sampleLocked(now)
			}
			dirty := e.dirty
			e.mu.Unlock()
			if dirty {
				e.publish()
			}
		}
	}
}

// sampleLocked records the files saved so far; e.mu must be held.
func (e *engine) sampleLocked(now time.Time) {
	e.samples = append(e.samples, Sample{TimeMs: now.UnixMilli(), Saved: e.stats.Saved})
	if len(e.samples) > sampleLimit {
		e.samples = append(e.samples[:0:0], e.samples[len(e.samples)-sampleLimit:]...)
	}
	e.dirty = true
}

// SetPaused pauses or resumes the crawl. Requests already running finish.
func (e *engine) SetPaused(paused bool) {
	e.mu.Lock()
	if !e.running || e.control == nil || e.stopping {
		e.mu.Unlock()
		return
	}
	state := mygo.ProgressIndeterminate
	if paused {
		e.control.Pause()
		e.status = "Paused: active requests will finish"
		state = mygo.ProgressPaused
	} else {
		e.control.Resume()
		e.status = "Download resumed"
	}
	e.paused = paused
	e.dirty = true
	window := e.window
	e.mu.Unlock()
	if window != nil {
		window.SetProgressBar(mygo.ProgressBar{State: state})
	}
	e.publish()
}

// TogglePause pauses a running crawl, or resumes a paused one.
func (e *engine) TogglePause() {
	e.mu.Lock()
	paused := e.paused
	e.mu.Unlock()
	e.SetPaused(!paused)
}

// Stop cancels the crawl.
func (e *engine) Stop() {
	e.mu.Lock()
	cancel := e.cancel
	if cancel == nil || e.stopping {
		e.mu.Unlock()
		return
	}
	e.stopping = true
	if e.paused && e.control != nil {
		e.control.Resume()
		e.paused = false
	}
	e.status = "Stopping..."
	e.dirty = true
	e.mu.Unlock()
	cancel()
	e.publish()
}

// CheckProxy finds the exit IP, country and timezone of the proxy in cfg.
// Only the network settings are used: no site cookies or authorization.
func (e *engine) CheckProxy(ctx context.Context, cfg config.Config) (ProxyProfile, error) {
	pcfg := proxyConfig(cfg)
	timeout := time.Duration(pcfg.Network.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	e.SetStatus("Checking proxy...")
	profile, err := crawler.CheckProxy(ctx, pcfg)
	if err != nil {
		e.SetStatus("Proxy check failed")
		return ProxyProfile{}, err
	}
	e.SetStatus("Proxy works: " + strings.TrimSpace(profile.Flag+" "+profile.Country))
	return profileFrom(profile), nil
}

// OpenBrowser starts a regular Chromium, without CDP, that browses through
// the proxy in cfg. It returns once the browser runs.
func (e *engine) OpenBrowser(cfg config.Config) (BrowserInfo, error) {
	icfg, startURL, err := interactiveConfig(cfg)
	if err != nil {
		return BrowserInfo{}, err
	}
	e.mu.Lock()
	if e.browserSession != nil || e.browserCancel != nil {
		e.mu.Unlock()
		return BrowserInfo{}, errors.New("the interactive browser is already open or starting")
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.browserCancel = cancel
	e.browser = BrowserStarting
	e.status = "Starting interactive Chromium..."
	e.dirty = true
	e.mu.Unlock()
	e.publish()

	session, err := crawler.OpenInteractiveBrowser(ctx, icfg, startURL)
	if err != nil {
		cancel()
		e.mu.Lock()
		e.browserCancel = nil
		e.browser = BrowserClosed
		e.status = "Could not start Chromium"
		e.dirty = true
		e.mu.Unlock()
		e.publish()
		if errors.Is(err, context.Canceled) {
			return BrowserInfo{}, nil
		}
		return BrowserInfo{}, err
	}

	route := proxyRoute(icfg)
	e.mu.Lock()
	e.browserSession = session
	e.browser = BrowserOpen
	e.status = "Regular Chromium opened: " + route
	e.appendLocked(LogEntry{Level: LevelInfo, Message: "Regular Chromium without CDP opened: " + route})
	e.mu.Unlock()
	e.publish()
	go e.waitBrowser(session, cancel)

	info := BrowserInfo{Opened: true, Route: route}
	if profile := session.Profile(); profile.IP != "" {
		p := profileFrom(profile)
		info.Profile = &p
	}
	if warning := session.Warning(); warning != "" {
		info.Warning = warning + "\nThe browser runs with its normal system fingerprint, without geo emulation."
	}
	return info, nil
}

// CloseBrowser closes the interactive browser and its proxy bridge.
func (e *engine) CloseBrowser() {
	e.mu.Lock()
	cancel, session := e.browserCancel, e.browserSession
	if cancel == nil && session == nil {
		e.mu.Unlock()
		return
	}
	e.status = "Closing interactive Chromium..."
	e.dirty = true
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if session != nil {
		go session.Close()
	}
	e.publish()
}

func (e *engine) waitBrowser(session *crawler.BrowserSession, cancel context.CancelFunc) {
	<-session.Done()
	cancel()
	e.mu.Lock()
	if e.browserSession != session {
		e.mu.Unlock()
		return
	}
	e.browserSession, e.browserCancel = nil, nil
	e.browser = BrowserClosed
	e.status = "Interactive Chromium closed"
	e.appendLocked(LogEntry{Level: LevelInfo, Message: "Interactive Chromium closed"})
	e.mu.Unlock()
	e.publish()
}

// Shutdown stops the crawl and closes the interactive browser, before the
// app quits.
func (e *engine) Shutdown() {
	e.mu.Lock()
	cancel, browserCancel, session := e.cancel, e.browserCancel, e.browserSession
	e.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if browserCancel != nil {
		browserCancel()
	}
	if session != nil {
		session.Close()
	}
}

// SetWindow gives the engine the window whose Dock or taskbar icon shows
// the progress of a crawl.
func (e *engine) SetWindow(w *mygo.Window) {
	e.mu.Lock()
	e.window = w
	e.mu.Unlock()
}

// SetStatus sets the status line.
func (e *engine) SetStatus(status string) {
	e.mu.Lock()
	e.status = status
	e.dirty = true
	e.mu.Unlock()
	e.publish()
}

// appendLocked adds a line to the log; e.mu must be held.
func (e *engine) appendLocked(entry LogEntry) {
	e.nextID++
	entry.ID = e.nextID
	if entry.TimeMs == 0 {
		entry.TimeMs = time.Now().UnixMilli()
	}
	e.pending = appendBounded(e.pending, pendingLimit, entry)
	e.dirty = true
}

func (e *engine) snapshotLocked() Snapshot {
	return Snapshot{
		Running: e.running, Paused: e.paused, Stopping: e.stopping,
		Browser: e.browser, Status: e.status, Stats: e.stats,
	}
}

// publish sends the state and the log lines added since the last update.
func (e *engine) publish() {
	e.sendMu.Lock()
	defer e.sendMu.Unlock()
	e.mu.Lock()
	update := Update{
		State:   e.snapshotLocked(),
		Log:     e.pending,
		Reset:   e.reset,
		Samples: append([]Sample(nil), e.samples...),
		Notice:  e.notice,
	}
	e.pending, e.reset, e.notice, e.dirty = nil, false, nil, false
	e.mu.Unlock()
	if e.sink != nil {
		e.sink(update)
	}
}

// readProfile reads the JSON profile at path.
func readProfile(path string) (config.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return config.Config{}, fmt.Errorf("import profile: %w", err)
	}
	defer file.Close()
	cfg, err := config.Decode(file)
	if err != nil {
		return config.Config{}, fmt.Errorf("import profile: %w", err)
	}
	return cfg, nil
}

// writeProfile saves cfg as a JSON profile at path.
func writeProfile(path string, cfg config.Config) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("export profile: %w", err)
	}
	if err := config.Encode(file, cfg); err != nil {
		_ = file.Close()
		return fmt.Errorf("export profile: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("export profile: %w", err)
	}
	return nil
}

// checkFolder returns the download folder path, trimmed, if it is a folder
// that exists.
func checkFolder(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("destination folder is not set")
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("folder %s does not exist yet: it appears after the first download", path)
	}
	if err != nil {
		return "", fmt.Errorf("open folder: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a folder", path)
	}
	return path, nil
}
