package crawler

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chromedp/chromedp"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestBrowserExecutesJavaScript(t *testing.T) {
	if os.Getenv("SITE_SUCKER_BROWSER_TEST") == "" {
		t.Skip("set SITE_SUCKER_BROWSER_TEST=1 to run the installed-Chromium test")
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="app"></div><script>document.getElementById('app').innerHTML='<a href="/dynamic">Rendered link</a>'; fetch('/api')</script></body></html>`))
	})
	mux.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	cfg := config.Default()
	cfg.StartURL = server.URL + "/"
	cfg.Browser.WaitMilliseconds = 300
	cfg.Browser.AutoScroll = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	browser, err := newBrowserFetcher(ctx, cfg)
	if err != nil {
		t.Fatalf("newBrowserFetcher() error = %v", err)
	}
	defer browser.Close()
	target, _ := url.Parse(cfg.StartURL)
	result, err := browser.fetch(ctx, target, nil)
	if err != nil {
		t.Fatalf("browser.fetch() error = %v", err)
	}
	if !strings.Contains(string(result.Body), "Rendered link") || !strings.Contains(string(result.Body), `/dynamic`) {
		t.Fatalf("JavaScript output missing: %s", result.Body)
	}
	var apiSeen bool
	for _, resource := range result.BrowserURLs {
		if resource.Path == "/api" {
			apiSeen = true
			break
		}
	}
	if !apiSeen {
		t.Fatalf("browser resource list does not contain /api: %v", result.BrowserURLs)
	}
}

func TestBrowserKeepsSiteHeadersOnConfiguredOrigin(t *testing.T) {
	if os.Getenv("SITE_SUCKER_BROWSER_TEST") == "" {
		t.Skip("set SITE_SUCKER_BROWSER_TEST=1 to run the installed-Chromium test")
	}
	targetHeaders := make(chan http.Header, 1)
	externalHeaders := make(chan http.Header, 1)
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		externalHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("image"))
	}))
	defer external.Close()
	externalURL := strings.Replace(external.URL, "127.0.0.1", "localhost", 1)
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHeaders <- r.Header.Clone()
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><img src="` + externalURL + `/pixel.png"></body></html>`))
	}))
	defer target.Close()

	cfg := config.Default()
	cfg.StartURL = target.URL + "/"
	cfg.Network.Headers = map[string]string{"X-Site-Token": "header-secret"}
	cfg.Authentication.Cookie = "session=cookie-secret"
	cfg.Browser.WaitMilliseconds = 300
	cfg.Browser.AutoScroll = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	browser, err := newBrowserFetcher(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	targetURL, _ := url.Parse(cfg.StartURL)
	if _, err := browser.fetch(ctx, targetURL, nil); err != nil {
		t.Fatal(err)
	}

	select {
	case headers := <-targetHeaders:
		if headers.Get("X-Site-Token") != "header-secret" || !strings.Contains(headers.Get("Cookie"), "session=cookie-secret") {
			t.Fatalf("configured origin did not receive credentials: %v", headers)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("configured origin request was not observed")
	}
	select {
	case headers := <-externalHeaders:
		if headers.Get("X-Site-Token") != "" || strings.Contains(headers.Get("Cookie"), "cookie-secret") {
			t.Fatalf("site credentials leaked cross-origin: %v", headers)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("external resource request was not observed")
	}
}

func TestBrowserUsesConfiguredHTTPProxy(t *testing.T) {
	if os.Getenv("SITE_SUCKER_BROWSER_TEST") == "" {
		t.Skip("set SITE_SUCKER_BROWSER_TEST=1 to run the installed-Chromium test")
	}
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><div id="proxied">Proxy works</div></body></html>`))
	}))
	defer target.Close()
	targetURL, _ := url.Parse(target.URL)
	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("proxy-user:proxy-password"))
	var targetSeen atomic.Bool
	var authSeen atomic.Bool
	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Proxy-Authorization") == expectedAuth {
			authSeen.Store(true)
		} else {
			w.Header().Set("Proxy-Authenticate", `Basic realm="test"`)
			http.Error(w, "proxy auth required", http.StatusProxyAuthRequired)
			return
		}
		if r.URL.Host == targetURL.Host {
			targetSeen.Store(true)
		}
		request := r.Clone(r.Context())
		request.RequestURI = ""
		request.Header.Del("Proxy-Authorization")
		response, err := http.DefaultTransport.RoundTrip(request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer response.Body.Close()
		copyHeaders(w.Header(), response.Header)
		w.WriteHeader(response.StatusCode)
		_, _ = io.Copy(w, response.Body)
	}))
	defer proxyServer.Close()

	cfg := config.Default()
	cfg.StartURL = target.URL + "/"
	cfg.Network.Proxy = config.Proxy{
		Type: config.ProxyHTTP, Address: proxyServer.URL,
		Username: "proxy-user", Password: "proxy-password",
	}
	cfg.Browser.WaitMilliseconds = 100
	cfg.Browser.AutoScroll = false
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	browser, err := newBrowserFetcher(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer browser.Close()
	pageURL, _ := url.Parse(cfg.StartURL)
	result, err := browser.fetch(ctx, pageURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(result.Body), "Proxy works") {
		t.Fatalf("unexpected browser result: %s", result.Body)
	}
	if !targetSeen.Load() || !authSeen.Load() {
		t.Fatalf("Chromium request did not pass through authenticated proxy: target=%t auth=%t", targetSeen.Load(), authSeen.Load())
	}
}

// sandboxOffSwitches turn off all or part of the Chromium sandbox.
var sandboxOffSwitches = []string{
	"no-sandbox", "disable-setuid-sandbox", "disable-gpu-sandbox",
	"disable-seccomp-filter-sandbox", "disable-namespace-sandbox",
}

func assertSandboxOn(t *testing.T, args []string) {
	t.Helper()
	for _, arg := range args {
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		if slices.Contains(sandboxOffSwitches, name) {
			t.Fatalf("Chromium arguments turn off the sandbox with %q: %s", arg, strings.Join(args, " "))
		}
	}
}

func TestBrowserAllocatorKeepsSandbox(t *testing.T) {
	for _, headless := range []bool{true, false} {
		t.Run(map[bool]string{true: "headless", false: "visible"}[headless], func(t *testing.T) {
			cfg := config.Default()
			cfg.Browser.Headless = headless
			cfg.Network.InsecureTLS = true
			// The program does not exist, so the allocator fails after it built the command.
			var args []string
			opts := append(browserAllocatorOptions(cfg, nil),
				chromedp.ExecPath("/do-not-run-chrome"),
				chromedp.ModifyCmdFunc(func(cmd *exec.Cmd) { args = slices.Clone(cmd.Args[1:]) }),
			)
			allocator, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
			defer cancelAlloc()
			ctx, cancel := chromedp.NewContext(allocator)
			defer cancel()
			err := chromedp.Do(ctx)
			if errors.Is(err, chromedp.ErrNoDisplay) {
				t.Skip("a visible window needs a display")
			}
			if err == nil {
				t.Fatal("expected an error for a program that does not exist")
			}
			if args == nil {
				t.Fatal("ModifyCmdFunc did not run")
			}
			assertSandboxOn(t, args)
		})
	}
}
