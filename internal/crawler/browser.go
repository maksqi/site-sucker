package crawler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"

	"github.com/maksqi/site-sucker/internal/config"
)

type browserFetcher struct {
	cfg         config.Config
	allocator   context.Context
	cancelAlloc context.CancelFunc
	root        context.Context
	cancelRoot  context.CancelFunc
	bridge      *proxyBridge
}

func newBrowserFetcher(parent context.Context, cfg config.Config) (*browserFetcher, error) {
	bridge, err := newProxyBridge(cfg)
	if err != nil {
		return nil, fmt.Errorf("local proxy bridge: %w", err)
	}
	allocator, cancelAlloc := chromedp.NewExecAllocator(parent, browserAllocatorOptions(cfg, bridge)...)
	root, cancelRoot := chromedp.NewContext(allocator)
	browser := &browserFetcher{
		cfg: cfg, allocator: allocator, cancelAlloc: cancelAlloc,
		root: root, cancelRoot: cancelRoot, bridge: bridge,
	}
	// Start Chromium now so browser-only mode can fail before work is queued.
	if err := chromedp.Do(root); err != nil {
		browser.Close()
		if os.Getuid() == 0 {
			return nil, fmt.Errorf("start Chromium: %w (Chromium always runs with its sandbox, which does not start as root; run Site Sucker as a regular user)", err)
		}
		return nil, fmt.Errorf("start Chromium: %w", err)
	}
	return browser, nil
}

func browserAllocatorOptions(cfg config.Config, bridge *proxyBridge) []chromedp.ExecAllocatorOption {
	opts := append([]chromedp.ExecAllocatorOption{}, chromedp.DefaultExecAllocatorOptions[:]...)
	opts = append(opts,
		chromedp.Flag("ignore-certificate-errors", cfg.Network.InsecureTLS),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("no-first-run", true),
		chromedp.UserAgent(cfg.Network.UserAgent),
		chromedp.WindowSize(cfg.Browser.ViewportWidth, cfg.Browser.ViewportHeight),
	)
	if cfg.Browser.ExecutablePath != "" {
		opts = append(opts, chromedp.ExecPath(cfg.Browser.ExecutablePath))
	}
	if bridge != nil {
		opts = append(opts,
			chromedp.ProxyServer(bridge.URL()),
			chromedp.Flag("proxy-bypass-list", "<-loopback>"),
			chromedp.Flag("force-webrtc-ip-handling-policy", "disable_non_proxied_udp"),
		)
	}
	if !cfg.Browser.Headless {
		// VisibleWindow removes the headless flags above and maximizes the window.
		opts = append(opts, chromedp.VisibleWindow)
	}
	// Chromium always keeps its sandbox. chromedp adds --no-sandbox when the
	// program runs as root unless this flag is set to false, and a flag that is
	// set again takes the last value, so it stays last.
	return append(opts, chromedp.Flag("no-sandbox", false))
}

func (b *browserFetcher) Close() {
	if b == nil {
		return
	}
	if b.cancelRoot != nil {
		b.cancelRoot()
	}
	if b.cancelAlloc != nil {
		b.cancelAlloc()
	}
	if b.bridge != nil {
		b.bridge.Close()
	}
}

func (b *browserFetcher) fetch(ctx context.Context, target *url.URL, seedCookies []*http.Cookie) (fetchResult, error) {
	tab, cancelTab := chromedp.NewContext(b.root)
	defer cancelTab()
	timeout := time.Duration(b.cfg.Network.TimeoutSeconds)*time.Second + time.Duration(b.cfg.Browser.WaitMilliseconds)*time.Millisecond
	if b.cfg.Browser.AutoScroll {
		timeout += 2 * time.Second
	}
	tab, cancelTimeout := context.WithTimeout(tab, timeout)
	defer cancelTimeout()

	var html string
	var finalURL string
	var resourceURLs []string
	var browserCookies []*network.Cookie
	var redirectCount atomic.Int64
	intercept := len(b.cfg.Network.Headers) > 0 || b.cfg.Authentication.Username != ""
	// The subscriptions start before the navigation, so no event is lost.
	requests := chromedp.Events(tab, network.RequestWillBeSent)
	go func() {
		for event, err := range requests {
			if err != nil {
				return
			}
			if event.Type == network.ResourceTypeDocument && event.RedirectResponse != nil {
				redirectCount.Add(1)
			}
		}
	}()
	if intercept {
		paused := chromedp.Events(tab, fetch.RequestPaused)
		authRequired := chromedp.Events(tab, fetch.AuthRequired)
		go func() {
			for event, err := range paused {
				if err != nil {
					return
				}
				go func() {
					params := fetch.ContinueRequestParams{RequestID: event.RequestID}
					requestURL, _ := url.Parse(event.Request.URL)
					if sameOrigin(requestURL, target) && len(b.cfg.Network.Headers) > 0 {
						params.Headers = requestHeaders(event.Request.Headers, b.cfg.Network.Headers)
					}
					_, _ = chromedp.Call(tab, fetch.ContinueRequest, params)
				}()
			}
		}()
		go func() {
			for event, err := range authRequired {
				if err != nil {
					return
				}
				go func() {
					response := fetch.AuthChallengeResponseResponseDefault
					requestURL, _ := url.Parse(event.Request.URL)
					auth := &fetch.AuthChallengeResponse{Response: response}
					if event.AuthChallenge.Source == fetch.AuthChallengeSourceServer && sameOrigin(requestURL, target) && b.cfg.Authentication.Username != "" {
						auth.Response = fetch.AuthChallengeResponseResponseProvideCredentials
						auth.Username = b.cfg.Authentication.Username
						auth.Password = b.cfg.Authentication.Password
					}
					_, _ = chromedp.Call(tab, fetch.ContinueWithAuth, fetch.ContinueWithAuthParams{RequestID: event.RequestID, AuthChallengeResponse: auth})
				}()
			}
		}()
	}

	actions := []chromedp.Action[chromedp.Void]{
		command(network.Enable, network.EnableParams{}),
	}
	if intercept {
		actions = append(actions, command(fetch.Enable, fetch.EnableParams{HandleAuthRequests: new(b.cfg.Authentication.Username != "")}))
	}
	for _, cookie := range mergeBrowserCookies(parseCookieHeader(b.cfg.Authentication.Cookie), seedCookies) {
		actions = append(actions, command(network.SetCookie, network.SetCookieParams{
			Name: cookie.Name, Value: cookie.Value,
			URL: target.Scheme + "://" + target.Host + "/", Path: cookie.Path,
		}))
	}
	actions = append(actions,
		chromedp.EmulateViewport(int64(b.cfg.Browser.ViewportWidth), int64(b.cfg.Browser.ViewportHeight)),
		chromedp.Navigate(target.String()),
	)
	if b.cfg.Browser.WaitMilliseconds > 0 {
		actions = append(actions, chromedp.Sleep(time.Duration(b.cfg.Browser.WaitMilliseconds)*time.Millisecond))
	}
	if b.cfg.Browser.AutoScroll {
		for i := 0; i < 3; i++ {
			actions = append(actions,
				chromedp.Evaluate[chromedp.Void](`window.scrollTo(0, Math.max(document.body.scrollHeight, document.documentElement.scrollHeight))`),
				chromedp.Sleep(350*time.Millisecond),
			)
		}
		actions = append(actions, chromedp.Evaluate[chromedp.Void](`window.scrollTo(0, 0)`))
	}
	actions = append(actions,
		store(chromedp.OuterHTML(chromedp.CSS("html")), &html),
		store(chromedp.Location(), &finalURL),
		store(chromedp.Evaluate[[]string](`performance.getEntriesByType('resource').map(function(e) { return e.name; })`), &resourceURLs),
		chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
			result, err := cdp.Call(ctx, t, network.GetCookies, network.GetCookiesParams{URLs: []string{target.String()}})
			browserCookies = result.Cookies
			return err
		}),
	)

	response, err := chromedp.RunResponse(tab, actions...)
	if err != nil {
		return fetchResult{}, err
	}
	if html == "" {
		return fetchResult{}, fmt.Errorf("Chromium returned empty HTML")
	}
	if redirectCount.Load() > int64(b.cfg.Network.MaxRedirects) {
		return fetchResult{}, fmt.Errorf("Chromium exceeded the redirect limit: %d", redirectCount.Load())
	}
	if b.cfg.Limits.MaxSizeEnabled && int64(len(html)) > b.cfg.Limits.MaxSizeKB*1024 {
		return fetchResult{}, fmt.Errorf("rendered HTML exceeds the size limit")
	}
	resolved, err := url.Parse(finalURL)
	if err != nil || resolved.Host == "" {
		resolved = target
	}
	parsedResources := make([]*url.URL, 0, len(resourceURLs))
	for _, raw := range resourceURLs {
		u, err := url.Parse(raw)
		if err == nil && (u.Scheme == "http" || u.Scheme == "https") {
			parsedResources = append(parsedResources, u)
		}
	}
	status := http.StatusOK
	contentType := "text/html; charset=utf-8"
	if response != nil {
		status = int(response.Status)
		if response.MimeType != "" {
			contentType = response.MimeType
		}
	}
	return fetchResult{
		Body:        []byte("<!doctype html>\n" + html),
		ContentType: contentType,
		StatusCode:  status,
		FinalURL:    resolved,
		BrowserURLs: parsedResources,
		Cookies:     toHTTPCookies(browserCookies),
	}, nil
}

func requestHeaders(original network.Headers, additional map[string]string) []*fetch.HeaderEntry {
	values := make(map[string]string, len(original)+len(additional))
	actualNames := make(map[string]string, len(original)+len(additional))
	for name, value := range original {
		lower := strings.ToLower(name)
		actualNames[lower] = name
		values[lower] = fmt.Sprint(value)
	}
	for name, value := range additional {
		if strings.TrimSpace(name) == "" || strings.EqualFold(name, "Host") || strings.EqualFold(name, "Proxy-Authorization") {
			continue
		}
		lower := strings.ToLower(name)
		actualNames[lower] = name
		values[lower] = value
	}
	headers := make([]*fetch.HeaderEntry, 0, len(values))
	for lower, value := range values {
		headers = append(headers, &fetch.HeaderEntry{Name: actualNames[lower], Value: value})
	}
	return headers
}

func parseCookieHeader(value string) []*http.Cookie {
	var cookies []*http.Cookie
	for _, part := range strings.Split(value, ";") {
		nameValue := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(nameValue) == 2 && nameValue[0] != "" {
			cookies = append(cookies, &http.Cookie{Name: nameValue[0], Value: nameValue[1], Path: "/"})
		}
	}
	return cookies
}

func mergeBrowserCookies(groups ...[]*http.Cookie) []*http.Cookie {
	merged := make(map[string]*http.Cookie)
	for _, cookies := range groups {
		for _, cookie := range cookies {
			merged[cookie.Name+"\x00"+cookie.Path] = cookie
		}
	}
	result := make([]*http.Cookie, 0, len(merged))
	for _, cookie := range merged {
		result = append(result, cookie)
	}
	return result
}

func toHTTPCookies(cookies []*network.Cookie) []*http.Cookie {
	result := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		converted := &http.Cookie{
			Name: cookie.Name, Value: cookie.Value, Domain: cookie.Domain, Path: cookie.Path,
			Secure: cookie.Secure, HttpOnly: cookie.HTTPOnly,
		}
		if !cookie.Session && cookie.Expires > 0 {
			converted.Expires = time.Unix(int64(cookie.Expires), 0)
		}
		result = append(result, converted)
	}
	return result
}

// command makes an action that sends a protocol command with no result.
func command[P any](cmd cdp.Command[P, cdp.Empty], params P) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		_, err := cdp.Call(ctx, t, cmd, params)
		return err
	})
}

// store makes an action that runs action and keeps its value in dst.
func store[T any](action chromedp.Action[T], dst *T) chromedp.Action[chromedp.Void] {
	return chromedp.Func(func(ctx context.Context, t *chromedp.Target) error {
		value, err := action(ctx, t)
		*dst = value
		return err
	})
}

func shouldRender(body []byte, contentType string) bool {
	if !isHTML(contentType, body) {
		return false
	}
	lower := strings.ToLower(string(body))
	markers := []string{
		"__next_data__", "id=\"root\"", "id=\"app\"", "ng-version=", "data-reactroot",
		"window.__nuxt__", "type=\"module\"", "javascript is required", "enable javascript",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return strings.Count(lower, "<script") >= 4 && strings.Count(lower, "<a ") < 2
}
