package crawler

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/maksqi/site-sucker/internal/config"
)

// BrowserSession owns a regular Chromium process used for manual browsing.
// It intentionally does not connect through CDP or modify browser JavaScript APIs.
type BrowserSession struct {
	command    *exec.Cmd
	bridge     *proxyBridge
	profileDir string
	profile    ProxyProfile
	warning    string
	done       chan struct{}
	closeOnce  sync.Once
}

func OpenInteractiveBrowser(ctx context.Context, cfg config.Config, startURL string) (*BrowserSession, error) {
	return openInteractiveBrowser(ctx, cfg, startURL, true)
}

func openInteractiveBrowser(ctx context.Context, cfg config.Config, startURL string, visible bool) (*BrowserSession, error) {
	var profile ProxyProfile
	var geoErr error
	if cfg.Browser.AutoGeo && cfg.Network.Proxy.Type != config.ProxyNone {
		profile, geoErr = CheckProxy(ctx, cfg)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	bridge, err := newProxyBridge(cfg)
	if err != nil {
		return nil, fmt.Errorf("local proxy bridge: %w", err)
	}
	profileDir, err := os.MkdirTemp("", "site-sucker-browser-*")
	if err != nil {
		bridge.Close()
		return nil, fmt.Errorf("temporary Chromium profile: %w", err)
	}
	executable, err := interactiveBrowserExecutable(cfg.Browser.ExecutablePath)
	if err != nil {
		bridge.Close()
		_ = os.RemoveAll(profileDir)
		return nil, err
	}

	args := interactiveBrowserArgs(cfg, interactiveStartURL(startURL), profileDir, bridge, visible)
	command := exec.Command(executable, args...)
	if err := command.Start(); err != nil {
		bridge.Close()
		_ = os.RemoveAll(profileDir)
		return nil, fmt.Errorf("start Chromium: %w", err)
	}
	session := &BrowserSession{
		command: command, bridge: bridge, profileDir: profileDir,
		profile: profile, done: make(chan struct{}),
	}
	if geoErr != nil {
		session.warning = "Could not detect the proxy geo profile: " + geoErr.Error()
	}
	go session.wait()
	go func() {
		select {
		case <-ctx.Done():
			session.Close()
		case <-session.Done():
		}
	}()
	return session, nil
}

func (s *BrowserSession) wait() {
	_ = s.command.Wait()
	s.bridge.Close()
	_ = os.RemoveAll(s.profileDir)
	close(s.done)
}

func (s *BrowserSession) Close() {
	if s == nil {
		return
	}
	s.closeOnce.Do(func() {
		if s.command != nil && s.command.Process != nil {
			_ = s.command.Process.Kill()
		}
	})
	<-s.done
}

func (s *BrowserSession) Done() <-chan struct{} {
	return s.done
}

func (s *BrowserSession) Profile() ProxyProfile {
	return s.profile
}

func (s *BrowserSession) Warning() string {
	return s.warning
}

func interactiveBrowserArgs(cfg config.Config, startURL, profileDir string, bridge *proxyBridge, visible bool) []string {
	args := []string{
		"--user-data-dir=" + profileDir,
		"--no-first-run",
		"--no-default-browser-check",
		"--new-window",
	}
	if !visible {
		args = append(args, "--headless=new")
	}
	if cfg.Network.InsecureTLS {
		args = append(args, "--ignore-certificate-errors")
	}
	if bridge != nil {
		args = append(args,
			"--proxy-server="+bridge.URL(),
			"--proxy-bypass-list=<-loopback>",
			"--force-webrtc-ip-handling-policy=disable_non_proxied_udp",
		)
	}
	return append(args, startURL)
}

func interactiveBrowserExecutable(configured string) (string, error) {
	configured = strings.TrimSpace(configured)
	if configured != "" {
		path, err := exec.LookPath(configured)
		if err != nil {
			return "", fmt.Errorf("Chromium not found at %q", configured)
		}
		return path, nil
	}

	var candidates []string
	switch runtime.GOOS {
	case "darwin":
		candidates = []string{
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			filepath.Join(os.Getenv("HOME"), "Applications/Chromium.app/Contents/MacOS/Chromium"),
			filepath.Join(os.Getenv("HOME"), "Applications/Google Chrome.app/Contents/MacOS/Google Chrome"),
		}
	case "windows":
		candidates = []string{
			"chrome", "chrome.exe",
			`C:\Program Files\Google\Chrome\Application\chrome.exe`,
			`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Chromium\Application\chrome.exe`),
		}
	default:
		candidates = []string{
			"chromium", "chromium-browser", "google-chrome", "google-chrome-stable",
			"google-chrome-beta", "google-chrome-unstable", "/usr/bin/google-chrome",
			"/snap/bin/chromium", "chrome",
		}
	}
	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path, nil
		}
	}
	return "", errors.New("Chrome or Chromium not found; set its path in the Chromium section")
}

func interactiveStartURL(value string) string {
	value = strings.TrimSpace(value)
	parsed, err := url.Parse(value)
	if err == nil && parsed.Host != "" && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		return parsed.String()
	}
	return "about:blank"
}
