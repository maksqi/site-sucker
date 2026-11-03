package crawler

import (
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestLocalPathForDocumentAndQuery(t *testing.T) {
	cfg := config.Default()
	cfg.Destination = t.TempDir()
	start, _ := url.Parse("https://example.com/")
	target, _ := url.Parse("https://example.com/catalog?page=2")
	filename := localPath(cfg, start, target, KindDocument)
	if !strings.HasSuffix(filepath.ToSlash(filename), "/example.com/catalog/index_q_bc7c7eb0.html") {
		t.Fatalf("localPath() = %q", filename)
	}
}

func TestLocalPathSeparatesExternalHost(t *testing.T) {
	cfg := config.Default()
	cfg.Destination = t.TempDir()
	start, _ := url.Parse("https://example.com/")
	target, _ := url.Parse("https://cdn.example.net/assets/app.js")
	filename := filepath.ToSlash(localPath(cfg, start, target, KindAsset))
	if !strings.Contains(filename, "/example.com/_external/https_cdn.example.net/assets/app.js") {
		t.Fatalf("localPath() = %q", filename)
	}
}

func TestLocalPathDoesNotCollapseUnicodeOrEscapedSlash(t *testing.T) {
	cfg := config.Default()
	cfg.Destination = t.TempDir()
	start, _ := url.Parse("https://example.com/")
	first, _ := url.Parse("https://example.com/日本語")
	second, _ := url.Parse("https://example.com/中文字")
	escaped, _ := url.Parse("https://example.com/a%2Fb")
	nested, _ := url.Parse("https://example.com/a/b")
	if localPath(cfg, start, first, KindDocument) == localPath(cfg, start, second, KindDocument) {
		t.Fatal("different Unicode paths collided")
	}
	if localPath(cfg, start, escaped, KindAsset) == localPath(cfg, start, nested, KindAsset) {
		t.Fatal("escaped slash collided with a path separator")
	}
}
