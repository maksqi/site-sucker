# Site Sucker

[![CI](https://github.com/maksqi/site-sucker/actions/workflows/ci.yml/badge.svg)](https://github.com/maksqi/site-sucker/actions/workflows/ci.yml)

Site Sucker is a desktop website downloader written in Go. It creates a browsable local copy of HTML pages and their CSS, JavaScript, images, fonts, media, and other linked files. The desktop app is built with [mygo](https://github.com/egoist/mygo): its interface is native UI written in Go with package `mygo/ui` and drawn on the GPU (Metal, Direct3D 11 or OpenGL), with no webview, HTML or JavaScript. It looks like a SwiftUI app on macOS: the system font and accent color, a floating sidebar and toolbar of Liquid Glass (mygo's `plugins/glass`), grouped forms as in System Settings, and the system's own menus for pop-up buttons.

## Features

- Fast concurrent HTTP crawler with pause, resume, cancellation, retries, delay, timeouts, redirects, and file/depth limits.
- JavaScript rendering through local Chrome or Chromium using `chromedp`.
- Auto mode that switches likely SPA pages to Chromium, plus an always-browser mode.
- Extraction of rendered DOM links and resources reported by the browser Performance API.
- HTTP, HTTPS, and SOCKS5 proxies with username/password authentication.
- A local HTTP proxy bridge lets Chromium use authenticated SOCKS5 proxies.
- Exact-host, registrable-domain, or unrestricted crawl scope.
- Optional external assets, query variants, `robots.txt`, `nofollow`, and include/exclude regular expressions.
- HTML and CSS link localization, including `srcset`, inline styles, CSS `url()`, `@import`, meta refresh, and HTML `<base>`.
- File replacement modes, remote modification times, MIME filters, and minimum image dimensions.
- Basic Auth, Cookie, custom headers, User-Agent, and optional insecure TLS.
- JSON profile import/export and a live, filterable event log with a per-second activity chart.
- Proxy geolocation with country, flag, city, ISP, IANA timezone, and automatic Chromium locale emulation.
- A standalone regular Chromium process without CDP automation for manual browsing through the configured proxy.

## Requirements

- Go 1.27.3 or newer (required by mygo). With the default `GOTOOLCHAIN=auto`, an older Go downloads it. No cgo, C toolchain, Node or Bun is needed; the mygo CLI is a Go tool of the module.
- macOS 12+, Windows 10/11, or Linux with GTK 3.
- Chrome or Chromium for `auto` and `browser` modes. HTTP-only mode does not need a browser.

## Install

```bash
go install github.com/maksqi/site-sucker/cmd/site-sucker@latest
```

This installs a plain binary. For the packaged app, clone the repository and build it as described below.

## Run

```bash
git clone https://github.com/maksqi/site-sucker.git
cd site-sucker
go tool mygo dev   # or: make dev
```

`mygo dev` builds the app and restarts it when Go files change.

Build the packaged app (`build/<os>-<arch>/`, on macOS `Site Sucker.app` and a disk image):

```bash
go tool mygo build   # or: make build
```

A plain `go run ./cmd/site-sucker` works too, outside an app bundle.

Run tests and checks:

```bash
go test ./...
go tool mygo vet .   # go vet, and the build lifetimes of the native UI
```

The interface's tests run the views headless with `ui.Tester`. To look at every page, light and dark, as PNG files:

```bash
SITE_SUCKER_SNAPSHOTS=/tmp/snapshots go test ./internal/desktop -run Snapshots
```

Run the optional integration tests against installed Chrome/Chromium:

```bash
SITE_SUCKER_BROWSER_TEST=1 go test ./internal/crawler -run Browser -v
```

## JavaScript Modes

- `http`: downloads server responses only. This is the fastest mode.
- `auto`: starts with HTTP, renders SPA/application shells, and always renders HTML in Chromium when a proxy is selected.
- `browser`: renders every discovered HTML document in Chromium. Assets still use the concurrent HTTP downloader.

Chromium executes page JavaScript, waits for the configured delay, optionally scrolls for lazy-loaded content, captures final HTML, follows rendered links, and records loaded resource URLs. It deliberately does not click arbitrary buttons because a generic click crawler can submit forms, delete data, log out, or trigger purchases.

When automatic proxy geolocation is enabled, Site Sucker resolves the current exit IP through the proxy before crawling. Chromium then receives the matching locale, `Accept-Language`, `navigator.language`, and IANA timezone. For an Italian exit this is normally `it-IT` with `Europe/Rome`. Manual locale and timezone fields are available on the Chromium tab as a fallback or override when auto detection is disabled.

When a proxy is active, Chromium also disables non-proxied WebRTC UDP to reduce the chance of bypassing the configured route.

Chromium always runs with its sandbox. Site Sucker never passes `--no-sandbox` or another switch that turns the sandbox off, in the crawler and in the interactive browser. `chromedp` adds `--no-sandbox` on its own when the program runs as root, and Site Sucker turns that off. Because Chromium does not start as root with its sandbox, run Site Sucker as a regular user for the `auto` and `browser` modes (for example in a Linux container).

## Interactive Browser

Use `Proxy → Interactive browser → Open browser` to launch a separate visible Chromium process. It uses the selected HTTP, HTTPS, or SOCKS5 proxy, proxy credentials, and WebRTC proxy-leak protection, but it is not connected to `chromedp` or the Chrome DevTools Protocol. The interactive mode does not inject JavaScript, alter `navigator`, force a User-Agent, or emulate locale/timezone; AutoGeo is used only to display the proxy exit profile. The start URL is opened when valid, otherwise Chromium opens a blank tab. You can use the address bar, navigate manually, open tabs, and sign in normally. Cookies, custom site headers, and Basic Auth values from the crawler are not injected into this manual browser. Closing its window or pressing `Close browser` also stops the local proxy bridge. A temporary profile keeps cookies only for that session.

Websites can still observe the browser's real fingerprint, the proxy exit IP, and normal interactive behavior. Site Sucker does not attempt to spoof fingerprint values or bypass anti-bot and anti-fraud checks.

## Proxy Formats

The address can include a scheme or use the separate type selector:

```text
127.0.0.1:8080
http://proxy.example:8080
https://proxy.example:8443
socks5://127.0.0.1:1080
```

Credentials can be entered in separate GUI fields. They are URL-encoded internally. Proxy and site passwords are stored as plain text when a JSON profile is exported, so profile files must be protected.

The proxy check button does not require a start URL. It requests `ipwho.is` through the selected proxy and displays the resulting IP, country, flag, city, ISP, locale, and timezone without sending site cookies or site authorization headers.

## Profiles

Use `File → Export Profile…` (⌘S) and `File → Import Profile…` (⌘O), or the buttons at the bottom of the sidebar. A complete sample is available at `configs/example.json`.

Important internal values for hand-written profiles:

- Browser mode: `http`, `auto`, `browser`.
- Proxy type: `none`, `http`, `https`, `socks5`.
- Host policy: `host`, `domain`, `any`.
- Replacement: `overwrite`, `skip`, `newer`.

## Project Layout

```text
cmd/site-sucker/       application entry point
internal/config/       defaults, validation, JSON profiles
internal/crawler/      scheduler, HTTP/browser fetchers, proxy bridge, parsers, storage
internal/desktop/      mygo app: the engine that runs crawls, and the native UI, its window and menus
internal/desktop/icons Lucide icons (ISC)
resources/             app icon
configs/               example profiles
.github/               CI and Dependabot
```

## Limitations And Safe Use

No generic downloader can guarantee an exact copy of every website. CAPTCHA, DRM, service workers, WebSockets, canvas-only interfaces, anti-bot systems, authenticated API calls, and button-only navigation may need site-specific logic. Browser mode improves support for JavaScript sites but does not bypass access controls.

Only download sites and content you are authorized to copy. Respect copyright, terms of service, rate limits, privacy, and local law. `robots.txt` is enabled by default.

## License

[MIT](LICENSE). The Lucide icons in `internal/desktop/icons` are under the [ISC License](internal/desktop/icons/LICENSE).
