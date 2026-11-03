package desktop

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/maksqi/site-sucker/internal/config"
	"github.com/maksqi/site-sucker/internal/crawler"
)

func TestProxyConfigKeepsOnlyNetwork(t *testing.T) {
	cfg := config.Default()
	cfg.StartURL = "https://example.com/"
	cfg.Authentication = config.Authentication{Username: "u", Password: "p", Cookie: "session=1"}
	cfg.Network.Headers = map[string]string{"Authorization": "Bearer x"}
	cfg.Network.TimeoutSeconds = 7
	cfg.Network.Proxy = config.Proxy{Type: config.ProxySOCKS5, Address: " 127.0.0.1:1080 ", Username: "pu", Password: "pp"}

	got := proxyConfig(cfg)
	if got.Authentication != (config.Authentication{}) {
		t.Fatalf("authentication leaked: %+v", got.Authentication)
	}
	if len(got.Network.Headers) != 0 {
		t.Fatalf("headers leaked: %v", got.Network.Headers)
	}
	if got.Network.TimeoutSeconds != 7 || got.Network.Proxy.Address != "127.0.0.1:1080" || got.Network.Proxy.Password != "pp" {
		t.Fatalf("network settings lost: %+v", got.Network)
	}
}

func TestInteractiveConfigStartURL(t *testing.T) {
	cases := map[string]string{
		"https://example.com/path": "https://example.com/path",
		"":                         "about:blank",
		"example.com":              "about:blank",
		"ftp://example.com/":       "about:blank",
	}
	for input, want := range cases {
		cfg := config.Default()
		cfg.StartURL = input
		cfg.Browser.ExecutablePath = " /usr/bin/chromium "
		got, startURL, err := interactiveConfig(cfg)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		if startURL != want {
			t.Fatalf("%q: start URL %q, want %q", input, startURL, want)
		}
		if got.Browser.ExecutablePath != "/usr/bin/chromium" {
			t.Fatalf("%q: executable %q", input, got.Browser.ExecutablePath)
		}
	}
}

func TestInteractiveConfigRejectsBadProxy(t *testing.T) {
	cfg := config.Default()
	cfg.Network.Proxy = config.Proxy{Type: config.ProxyHTTP}
	if _, _, err := interactiveConfig(cfg); err == nil {
		t.Fatal("expected an error for a proxy without an address")
	}
}

func TestLogEntryFor(t *testing.T) {
	levels := map[crawler.EventType]LogLevel{
		crawler.EventSaved:    LevelSave,
		crawler.EventSkipped:  LevelSkip,
		crawler.EventFailed:   LevelError,
		crawler.EventLog:      LevelInfo,
		crawler.EventFinished: LevelDone,
	}
	for eventType, want := range levels {
		entry, ok := logEntryFor(crawler.Event{Type: eventType, URL: "https://a/", Path: "a/index.html", Message: "m"})
		if !ok || entry.Level != want || entry.URL != "https://a/" || entry.Path != "a/index.html" || entry.Message != "m" {
			t.Fatalf("%s: got %+v, %v", eventType, entry, ok)
		}
	}
	for _, eventType := range []crawler.EventType{crawler.EventDiscovered, crawler.EventStarted} {
		if _, ok := logEntryFor(crawler.Event{Type: eventType}); ok {
			t.Fatalf("%s should not be logged", eventType)
		}
	}
}

func TestStatusFor(t *testing.T) {
	if status, ok := statusFor(crawler.Event{Type: crawler.EventFinished, Message: "Crawl finished"}); !ok || status != "Crawl finished" {
		t.Fatalf("finished: %q %v", status, ok)
	}
	if _, ok := statusFor(crawler.Event{Type: crawler.EventDiscovered}); ok {
		t.Fatal("discovered should not change the status")
	}
}

func TestAppendBounded(t *testing.T) {
	var log []LogEntry
	for i := 1; i <= 7; i++ {
		log = appendBounded(log, 5, LogEntry{ID: int64(i)})
	}
	if len(log) != 5 || log[0].ID != 3 || log[4].ID != 7 {
		t.Fatalf("got %+v", log)
	}
}

func TestShortTextCountsRunes(t *testing.T) {
	if got := shortText("Ünïcödé", 20); got != "Ünïcödé" {
		t.Fatalf("short text changed: %q", got)
	}
	if got := shortText("Ünïcödé", 5); got != "Ünïc…" {
		t.Fatalf("got %q", got)
	}
}

func TestResolveAppBundle(t *testing.T) {
	dir := t.TempDir()
	bundle := filepath.Join(dir, "Google Chrome.app")
	macOS := filepath.Join(bundle, "Contents", "MacOS")
	if err := os.MkdirAll(macOS, 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(macOS, "Google Chrome")
	if err := os.WriteFile(executable, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveAppBundle(bundle); got != executable {
		t.Fatalf("got %q, want %q", got, executable)
	}
	if got := resolveAppBundle("/usr/bin/chromium"); got != "/usr/bin/chromium" {
		t.Fatalf("plain path changed: %q", got)
	}

	other := filepath.Join(dir, "Chromium Beta.app")
	if err := os.MkdirAll(filepath.Join(other, "Contents", "MacOS"), 0o755); err != nil {
		t.Fatal(err)
	}
	inner := filepath.Join(other, "Contents", "MacOS", "Chromium")
	if err := os.WriteFile(inner, nil, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := resolveAppBundle(other); got != inner {
		t.Fatalf("got %q, want %q", got, inner)
	}
}
