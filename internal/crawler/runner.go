package crawler

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
	"golang.org/x/net/publicsuffix"

	"github.com/maksqi/site-sucker/internal/config"
)

type Runner struct {
	cfg        config.Config
	http       *httpFetcher
	browser    *browserFetcher
	browserMu  sync.Mutex
	robots     *robotsCache
	control    *Control
	emit       func(Event)
	counters   counters
	start      *url.URL
	include    *regexp.Regexp
	exclude    *regexp.Regexp
	startupLog string
}

type processResult struct {
	links []link
}

func Run(ctx context.Context, cfg config.Config, control *Control, emit func(Event)) (Stats, error) {
	if err := cfg.Validate(); err != nil {
		return Stats{}, err
	}
	if strings.TrimSpace(cfg.Destination) == "" {
		return Stats{}, errors.New("choose a destination folder before starting")
	}
	startupLog := ""
	if cfg.Browser.Mode != config.ModeHTTP {
		profile, geoErr := applyAutomaticProxyGeo(ctx, &cfg)
		if geoErr != nil {
			startupLog = "Could not detect the proxy geo profile: " + geoErr.Error()
		} else if profile.IP != "" {
			startupLog = fmt.Sprintf("Proxy geo profile: %s %s, locale=%s, timezone=%s", profile.Flag, profile.Country, cfg.Browser.Locale, cfg.Browser.Timezone)
		}
		if err := ctx.Err(); err != nil {
			return Stats{}, err
		}
	}
	start, _ := url.Parse(strings.TrimSpace(cfg.StartURL))
	start = canonicalURL(start, cfg.Scope.KeepQuery)
	httpClient, err := newHTTPFetcher(cfg)
	if err != nil {
		return Stats{}, err
	}
	if control == nil {
		control = NewControl()
	}
	runner := &Runner{
		cfg: cfg, http: httpClient, control: control, emit: emit, start: start,
		robots:     newRobotsCache(httpClient, "SiteSucker", cfg.Network.UserAgent),
		startupLog: startupLog,
	}
	defer runner.close()
	if cfg.Scope.IncludePattern != "" {
		runner.include = regexp.MustCompile(cfg.Scope.IncludePattern)
	}
	if cfg.Scope.ExcludePattern != "" {
		runner.exclude = regexp.MustCompile(cfg.Scope.ExcludePattern)
	}
	if cfg.Browser.Mode == config.ModeBrowser {
		if _, err := runner.ensureBrowser(ctx); err != nil {
			return Stats{}, err
		}
	}
	return runner.run(ctx)
}

func hasHeader(headers map[string]string, expected string) bool {
	for name := range headers {
		if strings.EqualFold(strings.TrimSpace(name), expected) {
			return true
		}
	}
	return false
}

func (r *Runner) run(ctx context.Context) (Stats, error) {
	r.counters.started = time.Now()
	r.counters.discovered.Store(1)
	r.send(Event{Type: EventStarted, URL: r.start.String(), Message: "Crawl started"})
	if r.startupLog != "" {
		r.send(Event{Type: EventLog, Message: r.startupLog})
	}

	visited := map[string]struct{}{r.start.String(): {}}
	queue := []job{{URL: r.start, Depth: 0, Kind: KindDocument}}
	for len(queue) > 0 {
		if err := ctx.Err(); err != nil {
			stats := r.finish(err)
			return stats, err
		}
		batchSize := min(len(queue), r.cfg.Network.Connections)
		batch := append([]job(nil), queue[:batchSize]...)
		queue = queue[batchSize:]
		completedBatch := r.processBatch(ctx, batch)
		if err := ctx.Err(); err != nil {
			stats := r.finish(err)
			return stats, err
		}
		for _, completed := range completedBatch {
			for _, discovered := range completed.result.links {
				if discovered.NoFollow && !r.cfg.Scope.CrawlNoFollowLinks {
					continue
				}
				if !r.inScope(discovered.URL, discovered.Kind) {
					continue
				}
				nextDepth := completed.source.Depth
				if discovered.Kind == KindDocument {
					nextDepth++
					if r.cfg.Limits.MaxDepthEnabled && nextDepth > r.cfg.Limits.MaxDepth {
						continue
					}
				}
				canonical := canonicalURL(discovered.URL, r.cfg.Scope.KeepQuery)
				key := canonical.String()
				if _, exists := visited[key]; exists {
					continue
				}
				if r.cfg.Limits.MaxFilesEnabled && int64(len(visited)) >= int64(r.cfg.Limits.MaxFiles) {
					continue
				}
				visited[key] = struct{}{}
				queue = append(queue, job{URL: canonical, Depth: nextDepth, Kind: discovered.Kind})
				r.counters.discovered.Add(1)
				r.send(Event{Type: EventDiscovered, URL: canonical.String(), Message: string(discovered.Kind)})
			}
		}
	}
	stats := r.finish(nil)
	return stats, nil
}

type completedJob struct {
	source job
	result processResult
	index  int
}

// processBatch keeps network work concurrent and returns discoveries in queue order.
func (r *Runner) processBatch(ctx context.Context, batch []job) []completedJob {
	results := make(chan completedJob, len(batch))
	var wg sync.WaitGroup
	for index, current := range batch {
		wg.Add(1)
		go func(index int, current job) {
			defer wg.Done()
			results <- completedJob{source: current, result: r.process(ctx, current), index: index}
		}(index, current)
	}
	wg.Wait()
	close(results)
	collected := make([]completedJob, len(batch))
	for result := range results {
		collected[result.index] = result
	}
	return collected
}

func (r *Runner) process(ctx context.Context, current job) processResult {
	if err := r.control.Wait(ctx); err != nil {
		return processResult{}
	}
	if r.cfg.Network.DelayMilliseconds > 0 {
		select {
		case <-ctx.Done():
			return processResult{}
		case <-time.After(time.Duration(r.cfg.Network.DelayMilliseconds) * time.Millisecond):
		}
	}
	if r.cfg.Scope.RespectRobots && !r.robots.allowed(ctx, current.URL) {
		r.counters.processed.Add(1)
		r.counters.skipped.Add(1)
		r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: "disallowed by robots.txt"})
		return processResult{}
	}

	result, err := r.fetch(ctx, current)
	if err != nil {
		r.counters.processed.Add(1)
		var skipped *skipError
		if errors.As(err, &skipped) {
			r.counters.skipped.Add(1)
			r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: skipped.Error()})
		} else if !errors.Is(err, context.Canceled) {
			r.counters.failed.Add(1)
			r.send(Event{Type: EventFailed, URL: current.URL.String(), Message: err.Error()})
		}
		return processResult{}
	}
	if err := ctx.Err(); err != nil {
		return processResult{}
	}
	if result.FinalURL != nil && !r.inScope(result.FinalURL, current.Kind) {
		r.counters.processed.Add(1)
		r.counters.skipped.Add(1)
		r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: "redirect outside the crawl scope: " + result.FinalURL.String()})
		return processResult{}
	}
	if result.StatusCode >= 400 && !r.cfg.Output.SaveErrorPages {
		r.counters.processed.Add(1)
		r.counters.failed.Add(1)
		r.send(Event{Type: EventFailed, URL: current.URL.String(), Message: fmt.Sprintf("HTTP %d", result.StatusCode)})
		return processResult{}
	}
	if r.cfg.Limits.MinSizeEnabled && int64(len(result.Body)) < r.cfg.Limits.MinSizeKB*1024 {
		r.counters.processed.Add(1)
		r.counters.skipped.Add(1)
		r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: fmt.Sprintf("file below the size limit: %d bytes", len(result.Body))})
		return processResult{}
	}

	base := result.FinalURL
	if base == nil {
		base = current.URL
	}
	filename := localPath(r.cfg, r.start, current.URL, current.Kind)
	rewrite := func(target *url.URL, kind ResourceKind) (string, bool) {
		if !r.cfg.Output.LocalizeLinks {
			return "", false
		}
		if !r.inScope(target, kind) || (kind == KindDocument && r.cfg.Limits.MaxDepthEnabled && current.Depth+1 > r.cfg.Limits.MaxDepth) {
			return target.String(), true
		}
		canonical := canonicalURL(target, r.cfg.Scope.KeepQuery)
		targetPath := localPath(r.cfg, r.start, canonical, kind)
		return relativeLink(filename, targetPath, target.Fragment), true
	}

	body := result.Body
	var links []link
	if isHTML(result.ContentType, body) {
		if rewritten, extracted, err := processHTML(body, result.ContentType, base, r.cfg.Output.LocalizeLinks, r.cfg.Scope.CrawlNoFollowLinks, rewrite); err == nil {
			body, links = rewritten, extracted
		} else {
			r.send(Event{Type: EventLog, URL: current.URL.String(), Message: "HTML parse: " + err.Error()})
		}
	} else if isCSS(result.ContentType) {
		body, links = processCSS(body, base, rewrite)
	}
	for _, browserURL := range result.BrowserURLs {
		links = append(links, link{URL: browserURL, Kind: KindAsset})
	}

	if !r.filterAllows(result.ContentType, body) {
		r.counters.processed.Add(1)
		r.counters.skipped.Add(1)
		r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: "excluded by the file type filter"})
		return processResult{links: links}
	}
	if r.cfg.Limits.MinImageEnabled && strings.HasPrefix(mediaType(result.ContentType), "image/") {
		imageConfig, _, err := image.DecodeConfig(bytes.NewReader(body))
		if err == nil && (imageConfig.Width < r.cfg.Limits.MinImageWidth || imageConfig.Height < r.cfg.Limits.MinImageHeight) {
			r.counters.processed.Add(1)
			r.counters.skipped.Add(1)
			r.send(Event{Type: EventSkipped, URL: current.URL.String(), Message: fmt.Sprintf("image too small: %dx%d", imageConfig.Width, imageConfig.Height)})
			return processResult{links: links}
		}
	}
	if err := ctx.Err(); err != nil {
		return processResult{}
	}

	written, err := writeFile(r.cfg, filename, body, result.LastModified)
	r.counters.processed.Add(1)
	if err != nil {
		r.counters.failed.Add(1)
		r.send(Event{Type: EventFailed, URL: current.URL.String(), Path: filename, Message: err.Error()})
		return processResult{links: links}
	}
	if !written {
		r.counters.skipped.Add(1)
		r.send(Event{Type: EventSkipped, URL: current.URL.String(), Path: filename, Message: "local file kept"})
		return processResult{links: links}
	}
	r.counters.saved.Add(1)
	r.counters.bytes.Add(int64(len(body)))
	r.send(Event{Type: EventSaved, URL: current.URL.String(), Path: filename, Message: fmt.Sprintf("saved %d bytes", len(body))})
	return processResult{links: links}
}

func (r *Runner) fetch(ctx context.Context, current job) (fetchResult, error) {
	result, err := r.http.fetch(ctx, current.URL)
	if err != nil || current.Kind != KindDocument || r.cfg.Browser.Mode == config.ModeHTTP {
		return result, err
	}
	if !shouldUseBrowser(r.cfg, result) {
		return result, nil
	}
	if current.Kind == KindDocument {
		browser, err := r.ensureBrowser(ctx)
		if err != nil {
			if r.cfg.Browser.Mode == config.ModeAuto {
				r.send(Event{Type: EventLog, URL: current.URL.String(), Message: "Chromium unavailable, using HTTP: " + err.Error()})
				return result, nil
			}
			return fetchResult{}, err
		}
		var rendered fetchResult
		var browserErr error
		for attempt := 1; attempt <= r.cfg.Network.Attempts; attempt++ {
			rendered, browserErr = browser.fetch(ctx, current.URL, r.http.cookies(current.URL))
			if browserErr == nil || ctx.Err() != nil {
				break
			}
			if attempt < r.cfg.Network.Attempts {
				select {
				case <-ctx.Done():
					return fetchResult{}, ctx.Err()
				case <-time.After(time.Duration(attempt) * 300 * time.Millisecond):
				}
			}
		}
		if browserErr != nil {
			if r.cfg.Browser.Mode == config.ModeAuto {
				r.send(Event{Type: EventLog, URL: current.URL.String(), Message: "JS rendering failed, using HTTP: " + browserErr.Error()})
				return result, nil
			}
			return fetchResult{}, browserErr
		}
		r.http.setCookies(rendered.FinalURL, rendered.Cookies)
		rendered.LastModified = result.LastModified
		return rendered, nil
	}
	return result, nil
}

func (r *Runner) ensureBrowser(ctx context.Context) (*browserFetcher, error) {
	r.browserMu.Lock()
	defer r.browserMu.Unlock()
	if r.browser != nil {
		return r.browser, nil
	}
	browser, err := newBrowserFetcher(ctx, r.cfg)
	if err != nil {
		return nil, err
	}
	r.browser = browser
	if r.cfg.Network.Proxy.Type == config.ProxyNone {
		r.send(Event{Type: EventLog, Message: "Chromium started with a direct connection"})
	} else {
		r.send(Event{Type: EventLog, Message: fmt.Sprintf("Chromium started through the %s proxy and the local proxy bridge", strings.ToUpper(r.cfg.Network.Proxy.Type))})
	}
	return browser, nil
}

func shouldUseBrowser(cfg config.Config, result fetchResult) bool {
	if !isHTML(result.ContentType, result.Body) {
		return false
	}
	switch cfg.Browser.Mode {
	case config.ModeBrowser:
		return true
	case config.ModeAuto:
		return cfg.Network.Proxy.Type != config.ProxyNone || shouldRender(result.Body, result.ContentType)
	default:
		return false
	}
}

func (r *Runner) close() {
	r.browserMu.Lock()
	defer r.browserMu.Unlock()
	if r.browser != nil {
		r.browser.Close()
		r.browser = nil
	}
	r.http.close()
}

func (r *Runner) inScope(target *url.URL, kind ResourceKind) bool {
	if target == nil || (target.Scheme != "http" && target.Scheme != "https") {
		return false
	}
	value := target.String()
	if r.include != nil && !r.include.MatchString(value) {
		return false
	}
	if r.exclude != nil && r.exclude.MatchString(value) {
		return false
	}
	if kind == KindAsset && r.cfg.Scope.ExternalAssets {
		return true
	}
	switch r.cfg.Scope.HostPolicy {
	case config.ScopeAnyHost:
		return true
	case config.ScopeDomain:
		startDomain, startErr := publicsuffix.EffectiveTLDPlusOne(r.start.Hostname())
		targetDomain, targetErr := publicsuffix.EffectiveTLDPlusOne(target.Hostname())
		if startErr == nil && targetErr == nil {
			return strings.EqualFold(startDomain, targetDomain)
		}
		fallthrough
	default:
		return strings.EqualFold(r.start.Host, target.Host)
	}
}

func (r *Runner) filterAllows(contentType string, body []byte) bool {
	media := mediaType(contentType)
	switch {
	case isHTML(contentType, body):
		return r.cfg.Filters.HTML
	case media == "text/css":
		return r.cfg.Filters.Styles
	case strings.Contains(media, "javascript") || media == "application/ecmascript":
		return r.cfg.Filters.Scripts
	case strings.HasPrefix(media, "image/"):
		return r.cfg.Filters.Images
	case strings.HasPrefix(media, "font/") || strings.Contains(media, "font") || strings.Contains(media, "woff"):
		return r.cfg.Filters.Fonts
	case strings.HasPrefix(media, "audio/") || strings.HasPrefix(media, "video/"):
		return r.cfg.Filters.Media
	case strings.Contains(media, "pdf") || strings.Contains(media, "zip") || strings.Contains(media, "officedocument") || strings.Contains(media, "msword"):
		return r.cfg.Filters.Documents
	default:
		return r.cfg.Filters.Other
	}
}

func (r *Runner) send(event Event) {
	event.Stats = r.counters.snapshot()
	if r.emit != nil {
		r.emit(event)
	}
}

func (r *Runner) finish(runErr error) Stats {
	stats := r.counters.snapshot()
	stats.FinishedAt = time.Now()
	message := "Crawl finished"
	if runErr != nil {
		message = "Crawl stopped: " + runErr.Error()
	}
	if r.emit != nil {
		r.emit(Event{Type: EventFinished, Message: message, Stats: stats})
	}
	return stats
}

func canonicalURL(input *url.URL, keepQuery bool) *url.URL {
	copyURL := *input
	copyURL.Fragment = ""
	copyURL.Host = strings.ToLower(copyURL.Host)
	if (copyURL.Scheme == "http" && copyURL.Port() == "80") || (copyURL.Scheme == "https" && copyURL.Port() == "443") {
		copyURL.Host = copyURL.Hostname()
	}
	if copyURL.Path == "" {
		copyURL.Path = "/"
	}
	if !keepQuery {
		copyURL.RawQuery = ""
		copyURL.ForceQuery = false
	}
	return &copyURL
}

func isHTML(contentType string, body []byte) bool {
	media := mediaType(contentType)
	if media == "text/html" || media == "application/xhtml+xml" {
		return true
	}
	trimmed := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 256)])))
	return strings.HasPrefix(trimmed, "<!doctype html") || strings.HasPrefix(trimmed, "<html")
}

func isCSS(contentType string) bool {
	return mediaType(contentType) == "text/css"
}

func mediaType(contentType string) string {
	value, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		value = strings.TrimSpace(strings.Split(contentType, ";")[0])
	}
	return strings.ToLower(value)
}

type robotsCache struct {
	client     *httpFetcher
	agent      string
	requestUA  string
	mu         sync.Mutex
	robotsData map[string]*robotstxt.RobotsData
}

func newRobotsCache(client *httpFetcher, agent, requestUA string) *robotsCache {
	return &robotsCache{client: client, agent: agent, requestUA: requestUA, robotsData: make(map[string]*robotstxt.RobotsData)}
}

func (r *robotsCache) allowed(ctx context.Context, target *url.URL) bool {
	key := target.Scheme + "://" + target.Host
	r.mu.Lock()
	data, ok := r.robotsData[key]
	r.mu.Unlock()
	if !ok {
		robotsURL, _ := url.Parse(key + "/robots.txt")
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, robotsURL.String(), nil)
		if err != nil {
			return true
		}
		req.Header.Set("User-Agent", r.requestUA)
		resp, err := r.client.client.Do(req)
		if err != nil {
			return true
		}
		defer resp.Body.Close()
		data, err = robotstxt.FromResponse(resp)
		if err != nil {
			return true
		}
		r.mu.Lock()
		r.robotsData[key] = data
		r.mu.Unlock()
	}
	if data == nil {
		return true
	}
	requestURI := target.EscapedPath()
	if target.RawQuery != "" {
		requestURI += "?" + target.RawQuery
	}
	return data.TestAgent(requestURI, r.agent)
}
