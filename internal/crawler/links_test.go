package crawler

import (
	"net/url"
	"strings"
	"testing"
)

func TestProcessHTMLUsesBaseAndLocalizesIt(t *testing.T) {
	base, _ := url.Parse("https://example.com/start/index.html")
	body := []byte(`<html><head><base href="https://cdn.example.com/app/"></head><body><a href="page">Page</a><img src="img/logo.png"></body></html>`)
	rewrite := func(target *url.URL, kind ResourceKind) (string, bool) {
		return "local/" + target.Host + target.Path, true
	}
	rewritten, links, err := processHTML(body, "text/html; charset=utf-8", base, true, false, rewrite)
	if err != nil {
		t.Fatalf("processHTML() error = %v", err)
	}
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2", len(links))
	}
	if got := links[0].URL.String(); got != "https://cdn.example.com/app/page" {
		t.Fatalf("first link = %q", got)
	}
	output := string(rewritten)
	if !strings.Contains(output, `<base href="."`) {
		t.Fatalf("localized base missing in %s", output)
	}
	if !strings.Contains(output, `href="local/cdn.example.com/app/page"`) {
		t.Fatalf("localized href missing in %s", output)
	}
}

func TestProcessCSSRewritesURLAndImport(t *testing.T) {
	base, _ := url.Parse("https://example.com/css/main.css")
	rewrite := func(target *url.URL, _ ResourceKind) (string, bool) {
		return "local-" + strings.TrimPrefix(target.Path, "/"), true
	}
	rewritten, links := processCSS([]byte(`@import "theme.css"; body { background: url('../img/bg.png') }`), base, rewrite)
	if len(links) != 2 {
		t.Fatalf("len(links) = %d, want 2", len(links))
	}
	output := string(rewritten)
	if !strings.Contains(output, `@import "local-css/theme.css"`) || !strings.Contains(output, `url('local-img/bg.png')`) {
		t.Fatalf("unexpected CSS: %s", output)
	}
}
