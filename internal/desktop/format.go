package desktop

import (
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/maksqi/site-sucker/internal/config"
)

// formatInt writes n with thousands separators: 1,234.
func formatInt(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	s = group(s)
	if neg {
		return "-" + s
	}
	return s
}

// group adds thousands separators to a run of digits.
func group(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var b strings.Builder
	head := len(digits) % 3
	if head > 0 {
		b.WriteString(digits[:head])
	}
	for i := head; i < len(digits); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(digits[i : i+3])
	}
	return b.String()
}

// formatDecimal writes v with one decimal and thousands separators: 1,024.5.
func formatDecimal(v float64) string {
	s := strconv.FormatFloat(math.Abs(v), 'f', 1, 64)
	whole, frac, _ := strings.Cut(s, ".")
	s = group(whole) + "." + frac
	if v < 0 && s != "0.0" {
		return "-" + s
	}
	return s
}

// formatCompact is the form for tight places: 1,284, 12.9K, 4.2M.
func formatCompact(n int64) string {
	switch abs := max(n, -n); {
	case abs < 10_000:
		return formatInt(n)
	case abs < 1_000_000:
		return formatDecimal(float64(n)/1000) + "K"
	default:
		return formatDecimal(float64(n)/1_000_000) + "M"
	}
}

var byteUnits = []string{"KB", "MB", "GB", "TB"}

// formatBytes writes a size: 512 B, 1.5 KB, 12.0 MB.
func formatBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	scaled, unit := float64(n), -1
	for {
		scaled /= 1024
		unit++
		if scaled < 1024 || unit == len(byteUnits)-1 {
			break
		}
	}
	return formatDecimal(scaled) + " " + byteUnits[unit]
}

// formatDuration writes 02:14, or 1:02:14 past an hour.
func formatDuration(ms int64) string {
	total := max(0, ms/1000)
	h, m, s := total/3600, total%3600/60, total%60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

// formatClock writes the local time of a Unix millisecond: 14:05:09.
func formatClock(ms int64) string {
	return time.UnixMilli(ms).Format("15:04:05")
}

// formatRate writes files per second with at most one decimal: 12, 3.5.
func formatRate(v float64) string {
	s := formatDecimal(v)
	return strings.TrimSuffix(s, ".0")
}

// hostOf returns the host, with its port, of an http or https URL, else "".
func hostOf(value string) string {
	u, err := url.Parse(strings.TrimSpace(value))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.Host
}

// hostInitial is the letter of a host for the badge in the URL field.
func hostInitial(host string) string {
	host = strings.TrimPrefix(host, "www.")
	for _, r := range host {
		return strings.ToUpper(string(r))
	}
	return ""
}

var trailingSeparators = regexp.MustCompile(`[\\/]+$`)

// baseName returns the last element of a path, for chips that show a folder.
func baseName(path string) string {
	trimmed := trailingSeparators.ReplaceAllString(strings.TrimSpace(path), "")
	if last := trimmed[strings.LastIndexAny(trimmed, `/\`)+1:]; last != "" {
		return last
	}
	return path
}

var homePrefix = regexp.MustCompile(`^/(?:Users|home)/[^/]+`)

// tildify abbreviates the home directory as ~ when a path shows it.
func tildify(path string) string {
	return homePrefix.ReplaceAllString(path, "~")
}

// plural picks the English form for n: plural(5, "file", "files").
func plural(n int64, one, other string) string {
	if n == 1 {
		return one
	}
	return other
}

// shortPath shows a saved file relative to the download folder, or with the
// home directory abbreviated.
func shortPath(path, destination string) string {
	root := trailingSeparators.ReplaceAllString(strings.TrimSpace(destination), "")
	if root != "" && strings.HasPrefix(path, root) && len(path) > len(root) && (path[len(root)] == '/' || path[len(root)] == '\\') {
		return path[len(root)+1:]
	}
	return tildify(path)
}

// logLine is a log entry as copied to the clipboard.
func logLine(e LogEntry) string {
	var detail string
	if e.Level == LevelSave && e.Path != "" {
		detail = e.URL + " -> " + e.Path
	} else {
		var parts []string
		if e.URL != "" {
			parts = append(parts, e.URL)
		}
		if m := firstNonEmpty(e.Message, e.Path); m != "" {
			parts = append(parts, m)
		}
		detail = strings.Join(parts, " | ")
	}
	return fmt.Sprintf("%s  %-5s  %s", formatClock(e.TimeMs), strings.ToUpper(string(e.Level)), detail)
}

// firstNonEmpty returns the first of its arguments that is not empty.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// parseHeaders reads header lines, "Name: Value", skipping empty lines and
// comments that start with #.
func parseHeaders(value string) (map[string]string, error) {
	headers := map[string]string{}
	for i, raw := range strings.Split(value, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		name, rest, ok := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if !ok || name == "" {
			return nil, fmt.Errorf("headers, line %d: expected Name: Value", i+1)
		}
		headers[name] = strings.TrimSpace(rest)
	}
	return headers, nil
}

// formatHeaders writes headers as lines that parseHeaders reads, by name.
func formatHeaders(headers map[string]string) string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	slices.Sort(names)
	lines := make([]string, len(names))
	for i, name := range names {
		lines[i] = name + ": " + headers[name]
	}
	return strings.Join(lines, "\n")
}

// collectConfig turns the settings the window edits into the config the
// engine takes: it trims the text fields and parses the header lines. For a
// proxy check (all false) the headers are left out.
func collectConfig(cfg config.Config, headersText string, all bool) (config.Config, error) {
	c := cfg
	c.StartURL = strings.TrimSpace(c.StartURL)
	c.Destination = strings.TrimSpace(c.Destination)
	c.Network.UserAgent = strings.TrimSpace(c.Network.UserAgent)
	c.Network.Proxy.Address = strings.TrimSpace(c.Network.Proxy.Address)
	c.Browser.ExecutablePath = strings.TrimSpace(c.Browser.ExecutablePath)
	c.Scope.IncludePattern = strings.TrimSpace(c.Scope.IncludePattern)
	c.Scope.ExcludePattern = strings.TrimSpace(c.Scope.ExcludePattern)
	c.Authentication.Cookie = strings.TrimSpace(c.Authentication.Cookie)
	c.Network.Headers = map[string]string{}
	if all {
		headers, err := parseHeaders(headersText)
		if err != nil {
			return c, err
		}
		c.Network.Headers = headers
	}
	return c, nil
}

// activityBars is how many seconds the activity strip shows.
const activityBars = 60

// activityData is what the dashboard shows of the samples: the files saved
// in each of the last seconds, -1 for none known, their peak, and the rate
// over the last ten seconds.
type activityData struct {
	bars [activityBars]int64
	peak int64
	rate float64
}

func activity(samples []Sample) activityData {
	var a activityData
	for i := range a.bars {
		a.bars[i] = -1
	}
	a.peak = 1
	deltas := max(0, len(samples)-1)
	from := max(0, deltas-activityBars)
	for i := from; i < deltas; i++ {
		v := max(0, samples[i+1].Saved-samples[i].Saved)
		a.bars[activityBars-(deltas-i)] = v
		a.peak = max(a.peak, v)
	}
	if len(samples) >= 2 {
		first, last := samples[max(0, len(samples)-11)], samples[len(samples)-1]
		if dt := float64(last.TimeMs-first.TimeMs) / 1000; dt > 0 {
			a.rate = float64(last.Saved-first.Saved) / dt
		}
	}
	return a
}
