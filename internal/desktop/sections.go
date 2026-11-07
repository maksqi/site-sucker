package desktop

import (
	"runtime"
	"strings"

	"github.com/egoist/mygo/ui"

	"github.com/maksqi/site-sucker/internal/config"
)

var browserModes = []option{
	{config.ModeHTTP, "HTTP only", "The fastest mode: saves server responses as they are, without a browser.", iconZap},
	{config.ModeAuto, "Auto", "Chromium steps in for SPAs and web apps, and for every HTML page when a proxy is set.", iconSparkles},
	{config.ModeBrowser, "Always Chromium", "Every HTML page renders in Chromium; assets download over HTTP.", iconAppWindow},
}

var hostPolicies = []option{
	{value: config.ScopeHost, label: "Exact host"},
	{value: config.ScopeDomain, label: "Domain and subdomains"},
	{value: config.ScopeAnyHost, label: "Any domain"},
}

var proxyTypes = []option{
	{value: config.ProxyNone, label: "No proxy"},
	{value: config.ProxyHTTP, label: "HTTP"},
	{value: config.ProxyHTTPS, label: "HTTPS"},
	{value: config.ProxySOCKS5, label: "SOCKS5"},
}

var replacements = []option{
	{value: config.ReplaceNewer, label: "If newer"},
	{value: config.ReplaceOverwrite, label: "Always"},
	{value: config.ReplaceSkip, label: "Never"},
}

const passwordWarning = "Passwords are stored in the JSON profile as plain text. Do not share the profile with others."

// em is a width in the interface's font size.
func em(n float32) float32 { return n * 13 }

// crawlPage holds what a crawl follows and how far: its limits, the links
// it takes, the URL filters and the types of files it saves.
func (a *app) crawlPage(c *ui.Context) {
	l := &a.cfg.Limits
	s := &a.cfg.Scope
	a.page(c, func() {
		a.group(c, "Limits", "A depth of 0 means only the start page and its embedded resources.", func() {
			a.row(c, "Maximum depth", "How many links to follow away from the start page", !l.MaxDepthEnabled, func() {
				numberField(a, c, "depth", &l.MaxDepth, numberOptions{lo: 0, hi: 1000, unit: "levels", width: em(7.5), disabled: !l.MaxDepthEnabled, label: "Maximum depth"})
				a.toggle(c, "depth-on", &l.MaxDepthEnabled, "Limit depth", false)
			})
			a.row(c, "Maximum files", "The crawl stops once this many files are saved", !l.MaxFilesEnabled, func() {
				numberField(a, c, "files", &l.MaxFiles, numberOptions{lo: 1, hi: 10_000_000, unit: "files", width: em(8), disabled: !l.MaxFilesEnabled, label: "Maximum files"})
				a.toggle(c, "files-on", &l.MaxFilesEnabled, "Limit the number of files", false)
			})
			a.row(c, "Minimum size", "Smaller files are skipped", !l.MinSizeEnabled, func() {
				numberField(a, c, "min-size", &l.MinSizeKB, numberOptions{lo: 0, hi: 1_000_000_000, unit: "KB", width: em(7.5), disabled: !l.MinSizeEnabled, label: "Minimum size"})
				a.toggle(c, "min-size-on", &l.MinSizeEnabled, "Limit minimum size", false)
			})
			a.row(c, "Maximum size", "Larger files are skipped", !l.MaxSizeEnabled, func() {
				numberField(a, c, "max-size", &l.MaxSizeKB, numberOptions{lo: 1, hi: 1_000_000_000, unit: "KB", width: em(7.5), disabled: !l.MaxSizeEnabled, label: "Maximum size"})
				a.toggle(c, "max-size-on", &l.MaxSizeEnabled, "Limit maximum size", false)
			})
			a.row(c, "Minimum image size", "Icons and tracking pixels smaller than this are not saved", !l.MinImageEnabled, func() {
				numberField(a, c, "image-w", &l.MinImageWidth, numberOptions{lo: 1, hi: 100_000, width: em(5), disabled: !l.MinImageEnabled, label: "Minimum image width"})
				ui.Text(c, "×").TextColor(a.pal.textFaint)
				numberField(a, c, "image-h", &l.MinImageHeight, numberOptions{lo: 1, hi: 100_000, unit: "px", width: em(6), disabled: !l.MinImageEnabled, label: "Minimum image height"})
				a.toggle(c, "image-on", &l.MinImageEnabled, "Limit image size", false)
			})
		})
		a.group(c, "Links", "", func() {
			a.row(c, "Which links to follow", "", false, func() {
				a.picker(c, "policy", "Which links to follow", &s.HostPolicy, hostPolicies, false)
			})
			a.row(c, "External assets", "Download CSS, JS, images and fonts from other hosts", false, func() {
				a.toggle(c, "external", &s.ExternalAssets, "External assets", false)
			})
			a.row(c, "Query string", "Treat URLs with different query parameters as different pages", false, func() {
				a.toggle(c, "query", &s.KeepQuery, "Query string", false)
			})
			a.row(c, "Respect robots.txt", "Skip what the site asks crawlers not to index", false, func() {
				a.toggle(c, "robots", &s.RespectRobots, "Respect robots.txt", false)
			})
			a.row(c, "Follow rel=nofollow", "These links are skipped by default", false, func() {
				a.toggle(c, "nofollow", &s.CrawlNoFollowLinks, "Follow rel=nofollow", false)
			})
		})
		a.group(c, "URL filters", "Regular expressions in Go (RE2) syntax, checked before a URL is queued.", func() {
			a.stacked(c, "Include URLs", "", false, func() {
				a.textField(c, "include", &s.IncludePattern, fieldOptions{placeholder: "optional regular expression", label: "Include URLs", mono: true})
			})
			a.stacked(c, "Exclude URLs", "", false, func() {
				a.textField(c, "exclude", &s.ExcludePattern, fieldOptions{placeholder: "for example: /logout|/admin", label: "Exclude URLs", mono: true})
			})
		})
		a.group(c, "File types", "HTML and CSS are parsed to find nested resources.", func() {
			ui.Grid(c).Columns(2).GapX(16).Padding(8, 0).Children(func() {
				for _, kind := range fileKinds {
					a.checkbox(c, kind.label, kind.icon, kind.on(&a.cfg.Filters))
				}
			})
		})
	})
}

// networkPage holds how requests go out: the connections, how the
// downloader introduces itself, the proxy and the site's credentials.
func (a *app) networkPage(c *ui.Context) {
	n := &a.cfg.Network
	px := &n.Proxy
	none := px.Type == config.ProxyNone
	auth := &a.cfg.Authentication
	a.page(c, func() {
		a.group(c, "Connections", "", func() {
			a.row(c, "Parallel connections", "1 to 100 concurrent requests", false, func() {
				numberField(a, c, "connections", &n.Connections, numberOptions{lo: 1, hi: 100, label: "Parallel connections"})
			})
			a.row(c, "Attempts per request", "Retries on network errors, 1 to 20", false, func() {
				numberField(a, c, "attempts", &n.Attempts, numberOptions{lo: 1, hi: 20, label: "Attempts per request"})
			})
			a.row(c, "Timeout", "How long to wait for the server to respond", false, func() {
				numberField(a, c, "timeout", &n.TimeoutSeconds, numberOptions{lo: 1, hi: 86_400, unit: "s", label: "Timeout"})
			})
			a.row(c, "Delay before each request", "A pause so the site is not overloaded", false, func() {
				numberField(a, c, "delay", &n.DelayMilliseconds, numberOptions{lo: 0, hi: 3_600_000, step: 100, unit: "ms", width: em(7.5), label: "Delay before each request"})
			})
			a.row(c, "Maximum redirects", "0 to 100 redirects per request", false, func() {
				numberField(a, c, "redirects", &n.MaxRedirects, numberOptions{lo: 0, hi: 100, label: "Maximum redirects"})
			})
		})
		a.group(c, "Identity", "", func() {
			a.stacked(c, "User-Agent", "How the downloader introduces itself to the site", false, func() {
				a.textField(c, "ua", &n.UserAgent, fieldOptions{placeholder: "Mozilla/5.0 …", label: "User-Agent", mono: true})
			})
			a.row(c, "Untrusted TLS certificates", "Allow self-signed and expired certificates", false, func() {
				a.toggle(c, "tls", &n.InsecureTLS, "Untrusted TLS certificates", false)
			})
		})
		a.group(c, "Proxy", "HTTP, HTTPS and SOCKS5. The check looks up the exit IP and country with ipwho.is.", func() {
			a.row(c, "Type", "", false, func() {
				a.picker(c, "type", "Proxy type", &px.Type, proxyTypes, false)
			})
			a.row(c, "Address", "host:port or a full URL with a scheme", none, func() {
				a.textField(c, "address", &px.Address, fieldOptions{placeholder: "127.0.0.1:1080 or socks5://host:port", label: "Proxy address", mono: true, width: em(19), disabled: none})
			})
			a.row(c, "Username", "", none, func() {
				a.textField(c, "proxy-user", &px.Username, fieldOptions{label: "Proxy username", width: em(19), disabled: none})
			})
			a.row(c, "Password", "", none, func() {
				a.textField(c, "proxy-password", &px.Password, fieldOptions{label: "Proxy password", width: em(19), disabled: none, reveal: &a.showProxyPassword})
			})
			a.proxyCheck(c, none)
		})
		a.group(c, "Site authentication", "", func() {
			a.row(c, "Username", "HTTP Basic", false, func() {
				a.textField(c, "user", &auth.Username, fieldOptions{label: "Username", width: em(19)})
			})
			a.row(c, "Password", "HTTP Basic", false, func() {
				a.textField(c, "password", &auth.Password, fieldOptions{label: "Password", width: em(19), reveal: &a.showAuthPassword})
			})
			a.stacked(c, "Cookie", "Sent with every request to the site", false, func() {
				a.textField(c, "cookie", &auth.Cookie, fieldOptions{placeholder: "session=...; token=...", label: "Cookie", mono: true})
			})
			a.stacked(c, "Extra headers", "One per line: Name: Value. Lines starting with # are ignored", false, func() {
				area := ui.TextArea(c.Key("headers"), &a.headersText).Lines(5, 14).Grow(1).MinHeight(110).
					Font(fontMono).FontSize(12).LineHeight(1.55).Padding(7, 9).Radius(6).
					Placeholder("Accept-Language: en-US,en;q=0.9\nX-Custom-Header: value").Label("Extra headers").FocusRing(false)
				a.fieldChrome(area, area.Focused(), false, false)
			})
		})
		a.callout(c, true, passwordWarning)
	})
}

// proxyCheck is the row of the proxy's check: where its exit is, and the
// button that looks it up.
func (a *app) proxyCheck(c *ui.Context, none bool) {
	p := a.pal
	geo := a.geo
	stale := a.geoStale()
	title, hint := "Check the proxy", "Not checked yet"
	switch {
	case none:
		hint = "Choose a proxy type first"
	case geo != nil:
		title = strings.TrimSpace(geo.Flag + " " + firstNonEmpty(geo.Country, "Unknown country"))
		var where []string
		for _, s := range []string{geo.IP, geo.City, geo.ISP} {
			if s != "" {
				where = append(where, s)
			}
		}
		hint = strings.Join(where, " · ")
		if stale {
			hint = "The proxy settings changed; check it again"
		}
	}
	ui.Row(c).MinHeight(56).Padding(8, 0).Gap(24).Children(func() {
		text := a.rowText(c, title, hint, none).Grow(1).Basis(0)
		if stale {
			text.Opacity(0.6)
		}
		b := a.buttonBase(c, "check", buttonSecondary, a.checkingProxy || none).Label("Check connection")
		b.Children(func() {
			if a.checkingProxy {
				ui.Spinner(c).Size(13, 13).TextColor(p.iconMuted)
				ui.Text(c, "Checking…").SingleLine()
			} else {
				ui.Text(c, "Check connection").SingleLine()
			}
		})
		if b.Clicked() {
			a.checkProxy()
		}
	})
}

// browserPage holds how Chromium renders pages.
func (a *app) browserPage(c *ui.Context) {
	b := &a.cfg.Browser
	a.page(c, func() {
		a.group(c, "Rendering", "", func() { a.modeRows(c) })
		a.group(c, "Chromium", "", func() {
			a.stacked(c, "Chrome or Chromium", "Leave empty to find it automatically", false, func() {
				a.textField(c, "exe", &b.ExecutablePath, fieldOptions{placeholder: "Find Chrome/Chromium automatically", label: "Chrome or Chromium", mono: true})
				if a.button(c, buttonSecondary, nil, "Browse…", false).Clicked() {
					a.chooseBrowser()
				}
			})
			a.row(c, "Wait after load", "Time for JavaScript to run before the DOM is captured", false, func() {
				numberField(a, c, "wait", &b.WaitMilliseconds, numberOptions{lo: 0, hi: 3_600_000, step: 100, unit: "ms", width: em(7.5), label: "Wait after load"})
			})
			a.row(c, "Browser window", "Viewport size, at least 320×240", false, func() {
				numberField(a, c, "vw", &b.ViewportWidth, numberOptions{lo: 320, hi: 16_384, step: 10, width: em(5.5), label: "Window width"})
				ui.Text(c, "×").TextColor(a.pal.textFaint)
				numberField(a, c, "vh", &b.ViewportHeight, numberOptions{lo: 240, hi: 16_384, step: 10, unit: "px", width: em(6.5), label: "Window height"})
			})
			a.row(c, "Run in background", "Headless: the Chromium window is not shown", false, func() {
				a.toggle(c, "headless", &b.Headless, "Run in background", false)
			})
			a.row(c, "Scroll for lazy loading", "Scroll the page so deferred images load", false, func() {
				a.toggle(c, "scroll", &b.AutoScroll, "Scroll for lazy loading", false)
			})
		})
		a.callout(c, false, "Chromium runs JavaScript and extracts the final DOM, SPA links and the resources the page actually loaded. It never clicks arbitrary buttons, so it cannot log you out or trigger dangerous actions.")
	})
}

// modeRows chooses the browser mode from a list with a check mark by the
// mode chosen.
func (a *app) modeRows(c *ui.Context) {
	p := a.pal
	ui.RadioGroup(c, func() {
		for _, o := range browserModes {
			chosen := a.cfg.Browser.Mode == o.value
			r := ui.RadioBase(c.Key(o.value), &a.cfg.Browser.Mode, o.value).Row().Gap(12).Padding(10, 0).Label(o.label)
			r.Children(func() {
				col := p.iconFaint
				if chosen {
					col = p.icon
				}
				ui.Icon(c, o.icon).FontSize(16).TextColor(col).Width(20).Shrink(0)
				a.rowText(c, o.label, o.hint, false).Grow(1).Basis(0)
				check := ui.Icon(c, iconCheck).FontSize(16).TextColor(p.accentText)
				if !chosen {
					check.Opacity(0)
				}
			})
		}
	}).Gap(1).Dividers(1, p.borderMuted)
}

// fileKinds are the types of files a crawl saves, as Crawl shows them.
var fileKinds = []struct {
	label string
	icon  *ui.SVG
	on    func(*config.Filters) *bool
}{
	{"HTML", iconFileCode, func(f *config.Filters) *bool { return &f.HTML }},
	{"CSS", iconPalette, func(f *config.Filters) *bool { return &f.Styles }},
	{"JavaScript", iconBraces, func(f *config.Filters) *bool { return &f.Scripts }},
	{"Images", iconImage, func(f *config.Filters) *bool { return &f.Images }},
	{"Fonts", iconType, func(f *config.Filters) *bool { return &f.Fonts }},
	{"Audio and video", iconFilm, func(f *config.Filters) *bool { return &f.Media }},
	{"Documents and archives", iconArchive, func(f *config.Filters) *bool { return &f.Documents }},
	{"Everything else", iconFileOther, func(f *config.Filters) *bool { return &f.Other }},
}

// outputPage holds where and how the local copy is saved.
func (a *app) outputPage(c *ui.Context) {
	o := &a.cfg.Output
	a.page(c, func() {
		a.group(c, "Destination", "", func() {
			a.stacked(c, "Folder", "", false, func() {
				a.textField(c, "dest", &a.cfg.Destination, fieldOptions{placeholder: "~/Downloads/site-sucker", label: "Folder", mono: true})
				if a.button(c, buttonSecondary, nil, "Choose…", false).Clicked() {
					a.chooseFolder()
				}
				open := a.button(c, buttonSecondary, iconFolderOpen, "", false).Label("Open folder").Tooltip(openFolderTip)
				if open.Clicked() {
					a.openFolder()
				}
			})
			a.row(c, "Ask for a folder before starting", "Show a folder picker on every start", false, func() {
				a.toggle(c, "ask", &a.cfg.AskDestination, "Ask for a folder before starting", false)
			})
		})
		a.group(c, "Local copy", "Query variants get a short, stable hash in their names.", func() {
			a.row(c, "Folder per host", "Save the site's files in a folder named after the host", false, func() {
				a.toggle(c, "host-folder", &o.CreateHostFolder, "Folder per host", false)
			})
			a.row(c, "Offline links", "Rewrite links in HTML and CSS for offline browsing", false, func() {
				a.toggle(c, "localize", &o.LocalizeLinks, "Offline links", false)
			})
			a.row(c, "Replace existing files", "", false, func() {
				a.picker(c, "replace", "Replace existing files", &o.Replacement, replacements, false)
			})
			a.row(c, "Keep Last-Modified", "Use the server's modification date for files", false, func() {
				a.toggle(c, "times", &o.PreserveTimes, "Keep Last-Modified", false)
			})
			a.row(c, "Error pages", "Save HTTP 4xx/5xx pages", false, func() {
				a.toggle(c, "errors", &o.SaveErrorPages, "Error pages", false)
			})
		})
	})
}

// openFolderTip names the system's file manager.
var openFolderTip = func() string {
	switch runtime.GOOS {
	case "darwin":
		return "Open in Finder"
	case "windows":
		return "Open in Explorer"
	}
	return "Open in the file manager"
}()
