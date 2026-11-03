package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestRunDownloadsAndLocalizesSite(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><link rel="stylesheet" href="/style.css"></head><body><a href="/page">Page</a><img src="/logo.png"></body></html>`))
	})
	mux.HandleFunc("/page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><a href="/deep">Too deep</a></body></html>`))
	})
	mux.HandleFunc("/deep", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("deep"))
	})
	mux.HandleFunc("/style.css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		_, _ = w.Write([]byte(`@import "/more.css"; body { background: url('/bg.png') }`))
	})
	for _, asset := range []string{"/logo.png", "/bg.png", "/more.css"} {
		asset := asset
		mux.HandleFunc(asset, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(asset, ".css") {
				w.Header().Set("Content-Type", "text/css")
			} else {
				w.Header().Set("Content-Type", "image/png")
			}
			_, _ = w.Write([]byte("asset"))
		})
	}
	server := httptest.NewServer(mux)
	defer server.Close()

	destination := t.TempDir()
	cfg := config.Default()
	cfg.StartURL = server.URL + "/"
	cfg.Destination = destination
	cfg.Browser.Mode = config.ModeHTTP
	cfg.Scope.RespectRobots = false
	cfg.Limits.MaxDepth = 1
	cfg.Limits.MaxFiles = 20
	cfg.Output.Replacement = config.ReplaceOverwrite
	cfg.Network.Connections = 4

	stats, err := Run(context.Background(), cfg, NewControl(), nil)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if stats.Saved != 6 || stats.Failed != 0 {
		t.Fatalf("stats = %#v, want 6 saved and 0 failed", stats)
	}
	root := filepath.Join(destination, "127.0.0.1")
	if _, err := os.Stat(filepath.Join(root, "deep", "index.html")); !os.IsNotExist(err) {
		t.Fatalf("depth-limited page was downloaded, stat error = %v", err)
	}
	index, err := os.ReadFile(filepath.Join(root, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), `href="page/index.html"`) || !strings.Contains(string(index), `href="style.css"`) {
		t.Fatalf("root links were not localized: %s", index)
	}
	css, err := os.ReadFile(filepath.Join(root, "style.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(css), `url('/bg.png')`) && !strings.Contains(string(css), `url('bg.png')`) {
		t.Fatalf("CSS resource was not preserved or localized: %s", css)
	}
}

func TestAutoModeUsesBrowserForHTMLWhenProxyConfigured(t *testing.T) {
	cfg := config.Default()
	cfg.Browser.Mode = config.ModeAuto
	plainHTML := fetchResult{ContentType: "text/html", Body: []byte(`<html><body>Static page</body></html>`)}
	if shouldUseBrowser(cfg, plainHTML) {
		t.Fatal("auto mode without proxy rendered a static page")
	}
	cfg.Network.Proxy = config.Proxy{Type: config.ProxySOCKS5, Address: "127.0.0.1:1080"}
	if !shouldUseBrowser(cfg, plainHTML) {
		t.Fatal("auto mode with proxy did not select Chromium for HTML")
	}
	nonHTML := fetchResult{ContentType: "application/pdf", Body: []byte("pdf")}
	if shouldUseBrowser(cfg, nonHTML) {
		t.Fatal("proxy forced Chromium for a non-HTML resource")
	}
}
