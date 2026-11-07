package desktop

// LogLevel classifies a log line.
type LogLevel string

const (
	LevelSave  LogLevel = "save"
	LevelSkip  LogLevel = "skip"
	LevelError LogLevel = "error"
	LevelInfo  LogLevel = "info"
	LevelDone  LogLevel = "done"
)

// NoticeKind selects how a notice looks.
type NoticeKind string

const (
	NoticeInfo    NoticeKind = "info"
	NoticeSuccess NoticeKind = "success"
	NoticeWarning NoticeKind = "warning"
	NoticeError   NoticeKind = "error"
)

// Stats are the crawl counters.
type Stats struct {
	Discovered int64
	Processed  int64
	Saved      int64
	Skipped    int64
	Failed     int64
	Bytes      int64
	// StartedAtMs and FinishedAtMs are Unix milliseconds, 0 when unknown.
	StartedAtMs  int64
	FinishedAtMs int64
}

// Snapshot is the state of the engine the window shows.
type Snapshot struct {
	Running  bool
	Paused   bool
	Stopping bool
	Status   string
	Stats    Stats
}

// LogEntry is one line of the event log.
type LogEntry struct {
	ID      int64
	TimeMs  int64
	Level   LogLevel
	URL     string
	Path    string
	Message string
}

// Sample is the number of files saved at a moment of a crawl, taken once a
// second for the activity chart and the rate.
type Sample struct {
	TimeMs int64
	Saved  int64
}

// Update carries the state and the log lines added since the last update
// to the window. With Reset a new crawl began: the window replaces its log
// with Log instead of appending to it.
type Update struct {
	State   Snapshot
	Log     []LogEntry
	Reset   bool
	Samples []Sample
	Notice  *Notice
}

// Notice is a message for a toast.
type Notice struct {
	Kind    NoticeKind
	Title   string
	Message string
}

// ProxyProfile describes the exit of a proxy.
type ProxyProfile struct {
	IP          string
	Country     string
	CountryCode string
	Region      string
	City        string
	Flag        string
	ISP         string
	Summary     string
}
