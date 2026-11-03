package desktop

import (
	"testing"
	"time"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestFormatNumbers(t *testing.T) {
	cases := []struct{ got, want string }{
		{formatInt(0), "0"},
		{formatInt(1234), "1,234"},
		{formatInt(-1234567), "-1,234,567"},
		{formatCompact(9999), "9,999"},
		{formatCompact(12_900), "12.9K"},
		{formatCompact(10_000), "10.0K"},
		{formatCompact(4_200_000), "4.2M"},
		{formatCompact(1_000_000_000), "1,000.0M"},
		{formatBytes(512), "512 B"},
		{formatBytes(1024), "1.0 KB"},
		{formatBytes(1536), "1.5 KB"},
		{formatBytes(12 << 20), "12.0 MB"},
		{formatBytes(3 << 50), "3,072.0 TB"},
		{formatDuration(0), "00:00"},
		{formatDuration(134_999), "02:14"},
		{formatDuration(3_734_000), "1:02:14"},
		{formatRate(12), "12"},
		{formatRate(3.46), "3.5"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestFormatClock(t *testing.T) {
	at := time.Date(2026, 1, 2, 14, 5, 9, 0, time.Local)
	if got := formatClock(at.UnixMilli()); got != "14:05:09" {
		t.Fatalf("got %q", got)
	}
}

func TestPaths(t *testing.T) {
	cases := []struct{ got, want string }{
		{hostOf(" https://www.example.com:8080/a "), "www.example.com:8080"},
		{hostOf("ftp://example.com/"), ""},
		{hostOf("example.com"), ""},
		{hostInitial("www.example.com"), "E"},
		{hostInitial(""), ""},
		{baseName("/Users/ada/Downloads/site-sucker/"), "site-sucker"},
		{baseName(`C:\Sites\copy`), "copy"},
		{baseName("/"), "/"},
		{tildify("/Users/ada/Downloads"), "~/Downloads"},
		{tildify("/home/ada/x"), "~/x"},
		{tildify("/opt/x"), "/opt/x"},
		{shortPath("/Users/ada/dl/example.com/index.html", "/Users/ada/dl/"), "example.com/index.html"},
		{shortPath("/Users/ada/dlx/index.html", "/Users/ada/dl"), "~/dlx/index.html"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestLogLine(t *testing.T) {
	ms := time.Date(2026, 1, 2, 9, 8, 7, 0, time.Local).UnixMilli()
	cases := []struct {
		e    LogEntry
		want string
	}{
		{LogEntry{TimeMs: ms, Level: LevelSave, URL: "https://a/", Path: "/d/a/index.html"}, "09:08:07  SAVE   https://a/ -> /d/a/index.html"},
		{LogEntry{TimeMs: ms, Level: LevelError, URL: "https://a/x", Message: "404"}, "09:08:07  ERROR  https://a/x | 404"},
		{LogEntry{TimeMs: ms, Level: LevelDone, Message: "Crawl finished"}, "09:08:07  DONE   Crawl finished"},
	}
	for _, c := range cases {
		if got := logLine(c.e); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}

func TestHeaders(t *testing.T) {
	headers, err := parseHeaders("# comment\n Accept-Language: en-US \n\nX-Token:a:b\nX-Token: c")
	if err != nil {
		t.Fatal(err)
	}
	if len(headers) != 2 || headers["Accept-Language"] != "en-US" || headers["X-Token"] != "c" {
		t.Fatalf("got %v", headers)
	}
	if got := formatHeaders(map[string]string{"b": "2", "a": "1"}); got != "a: 1\nb: 2" {
		t.Fatalf("got %q", got)
	}
	if _, err := parseHeaders("ok: 1\nbroken"); err == nil || err.Error() != "headers, line 2: expected Name: Value" {
		t.Fatalf("got %v", err)
	}
}

func TestCollectConfig(t *testing.T) {
	cfg := config.Default()
	cfg.StartURL = "  https://example.com/  "
	cfg.Network.Proxy.Address = " 127.0.0.1:1080 "
	got, err := collectConfig(cfg, "X: 1", true)
	if err != nil {
		t.Fatal(err)
	}
	if got.StartURL != "https://example.com/" || got.Network.Proxy.Address != "127.0.0.1:1080" || got.Network.Headers["X"] != "1" {
		t.Fatalf("got %+v", got)
	}
	if got, _ := collectConfig(cfg, "X: 1", false); len(got.Network.Headers) != 0 {
		t.Fatalf("headers kept: %v", got.Network.Headers)
	}
	if _, err := collectConfig(cfg, "bad", true); err == nil {
		t.Fatal("expected a header error")
	}
}

func TestActivity(t *testing.T) {
	if a := activity(nil); a.bars[0] != -1 || a.bars[activityBars-1] != -1 || a.peak != 1 || a.rate != 0 {
		t.Fatalf("empty: %+v", a)
	}
	var samples []Sample
	for i := range 80 {
		samples = append(samples, Sample{TimeMs: int64(i) * 1000, Saved: int64(i * 2)})
	}
	samples[79].Saved = samples[78].Saved + 10
	a := activity(samples)
	if a.bars[0] != 2 || a.bars[activityBars-1] != 10 || a.peak != 10 {
		t.Fatalf("bars: %v peak %d", a.bars, a.peak)
	}
	if a.rate != 2.8 {
		t.Fatalf("rate %v", a.rate)
	}
	short := activity(samples[:3])
	if short.bars[activityBars-3] != -1 || short.bars[activityBars-2] != 2 || short.bars[activityBars-1] != 2 {
		t.Fatalf("short: %v", short.bars)
	}
}
