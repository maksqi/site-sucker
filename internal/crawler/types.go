package crawler

import (
	"net/http"
	"net/url"
	"sync/atomic"
	"time"
)

type ResourceKind string

const (
	KindDocument ResourceKind = "document"
	KindAsset    ResourceKind = "asset"
)

type EventType string

const (
	EventStarted    EventType = "started"
	EventDiscovered EventType = "discovered"
	EventSaved      EventType = "saved"
	EventSkipped    EventType = "skipped"
	EventFailed     EventType = "failed"
	EventLog        EventType = "log"
	EventFinished   EventType = "finished"
)

type Event struct {
	Type    EventType
	URL     string
	Path    string
	Message string
	Stats   Stats
}

type Stats struct {
	Discovered int64
	Processed  int64
	Saved      int64
	Skipped    int64
	Failed     int64
	Bytes      int64
	StartedAt  time.Time
	FinishedAt time.Time
}

type counters struct {
	discovered atomic.Int64
	processed  atomic.Int64
	saved      atomic.Int64
	skipped    atomic.Int64
	failed     atomic.Int64
	bytes      atomic.Int64
	started    time.Time
}

func (c *counters) snapshot() Stats {
	return Stats{
		Discovered: c.discovered.Load(),
		Processed:  c.processed.Load(),
		Saved:      c.saved.Load(),
		Skipped:    c.skipped.Load(),
		Failed:     c.failed.Load(),
		Bytes:      c.bytes.Load(),
		StartedAt:  c.started,
	}
}

type job struct {
	URL   *url.URL
	Depth int
	Kind  ResourceKind
}

type link struct {
	URL      *url.URL
	Kind     ResourceKind
	NoFollow bool
}

type fetchResult struct {
	Body         []byte
	ContentType  string
	StatusCode   int
	FinalURL     *url.URL
	LastModified time.Time
	BrowserURLs  []*url.URL
	Cookies      []*http.Cookie
}
