package desktop

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/maksqi/site-sucker/internal/config"
	"github.com/maksqi/site-sucker/internal/crawler"
)

// proxyConfig keeps only the network settings of cfg, the ones a proxy
// check needs, so it never sends site cookies or authorization.
func proxyConfig(cfg config.Config) config.Config {
	out := config.Default()
	out.Network.Connections = cfg.Network.Connections
	out.Network.Attempts = cfg.Network.Attempts
	out.Network.TimeoutSeconds = cfg.Network.TimeoutSeconds
	out.Network.DelayMilliseconds = cfg.Network.DelayMilliseconds
	out.Network.MaxRedirects = cfg.Network.MaxRedirects
	out.Network.UserAgent = strings.TrimSpace(cfg.Network.UserAgent)
	out.Network.InsecureTLS = cfg.Network.InsecureTLS
	out.Network.Proxy = config.Proxy{
		Type:     cfg.Network.Proxy.Type,
		Address:  strings.TrimSpace(cfg.Network.Proxy.Address),
		Username: cfg.Network.Proxy.Username,
		Password: cfg.Network.Proxy.Password,
	}
	return out
}

// logEntryFor turns a crawler event into a log line; events that only move
// the counters have none.
func logEntryFor(event crawler.Event) (LogEntry, bool) {
	entry := LogEntry{URL: event.URL, Path: event.Path, Message: event.Message}
	switch event.Type {
	case crawler.EventSaved:
		entry.Level = LevelSave
	case crawler.EventSkipped:
		entry.Level = LevelSkip
	case crawler.EventFailed:
		entry.Level = LevelError
	case crawler.EventLog:
		entry.Level = LevelInfo
	case crawler.EventFinished:
		entry.Level = LevelDone
	default:
		return LogEntry{}, false
	}
	return entry, true
}

// statusFor returns the status line an event sets, if any.
func statusFor(event crawler.Event) (string, bool) {
	switch event.Type {
	case crawler.EventStarted:
		return "Download started", true
	case crawler.EventSaved:
		return "Saved: " + shortText(event.URL, 80), true
	case crawler.EventFailed:
		return "Error: " + shortText(event.Message, 80), true
	case crawler.EventFinished:
		return event.Message, true
	}
	return "", false
}

func statsFrom(s crawler.Stats) Stats {
	return Stats{
		Discovered: s.Discovered, Processed: s.Processed, Saved: s.Saved,
		Skipped: s.Skipped, Failed: s.Failed, Bytes: s.Bytes,
		StartedAtMs: unixMilli(s.StartedAt), FinishedAtMs: unixMilli(s.FinishedAt),
	}
}

func profileFrom(p crawler.ProxyProfile) ProxyProfile {
	return ProxyProfile{
		IP: p.IP, Country: p.Country, CountryCode: p.CountryCode, Region: p.Region,
		City: p.City, Flag: p.Flag, ISP: p.ISP,
		Summary: p.Summary(),
	}
}

func unixMilli(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UnixMilli()
}

// resolveAppBundle turns a macOS application bundle, which the file dialog
// returns as a whole, into the executable inside it.
func resolveAppBundle(path string) string {
	if !strings.HasSuffix(strings.TrimRight(path, "/"), ".app") {
		return path
	}
	bundle := strings.TrimRight(path, "/")
	macOS := filepath.Join(bundle, "Contents", "MacOS")
	named := filepath.Join(macOS, strings.TrimSuffix(filepath.Base(bundle), ".app"))
	if info, err := os.Stat(named); err == nil && !info.IsDir() {
		return named
	}
	entries, err := os.ReadDir(macOS)
	if err != nil {
		return path
	}
	for _, entry := range entries {
		if entry.Type().IsRegular() {
			return filepath.Join(macOS, entry.Name())
		}
	}
	return path
}

// appendBounded appends entries to log, keeping at most limit of the newest.
func appendBounded(log []LogEntry, limit int, entries ...LogEntry) []LogEntry {
	log = append(log, entries...)
	if len(log) > limit {
		log = append([]LogEntry(nil), log[len(log)-limit:]...)
	}
	return log
}

func shortText(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit-1]) + "…"
}
