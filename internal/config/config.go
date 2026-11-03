package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	ModeHTTP    = "http"
	ModeAuto    = "auto"
	ModeBrowser = "browser"

	ProxyNone   = "none"
	ProxyHTTP   = "http"
	ProxyHTTPS  = "https"
	ProxySOCKS5 = "socks5"

	ScopeHost    = "host"
	ScopeDomain  = "domain"
	ScopeAnyHost = "any"

	ReplaceOverwrite = "overwrite"
	ReplaceSkip      = "skip"
	ReplaceNewer     = "newer"
)

type Config struct {
	StartURL       string         `json:"start_url"`
	Destination    string         `json:"destination"`
	AskDestination bool           `json:"ask_destination"`
	Limits         Limits         `json:"limits"`
	Network        Network        `json:"network"`
	Browser        Browser        `json:"browser"`
	Scope          Scope          `json:"scope"`
	Output         Output         `json:"output"`
	Filters        Filters        `json:"filters"`
	Authentication Authentication `json:"authentication"`
}

type Limits struct {
	MaxDepthEnabled bool  `json:"max_depth_enabled"`
	MaxDepth        int   `json:"max_depth"`
	MaxFilesEnabled bool  `json:"max_files_enabled"`
	MaxFiles        int   `json:"max_files"`
	MinSizeEnabled  bool  `json:"min_size_enabled"`
	MinSizeKB       int64 `json:"min_size_kb"`
	MaxSizeEnabled  bool  `json:"max_size_enabled"`
	MaxSizeKB       int64 `json:"max_size_kb"`
	MinImageEnabled bool  `json:"min_image_enabled"`
	MinImageWidth   int   `json:"min_image_width"`
	MinImageHeight  int   `json:"min_image_height"`
}

type Network struct {
	Connections       int               `json:"connections"`
	Attempts          int               `json:"attempts"`
	TimeoutSeconds    int               `json:"timeout_seconds"`
	DelayMilliseconds int               `json:"delay_milliseconds"`
	MaxRedirects      int               `json:"max_redirects"`
	UserAgent         string            `json:"user_agent"`
	InsecureTLS       bool              `json:"insecure_tls"`
	Headers           map[string]string `json:"headers"`
	Proxy             Proxy             `json:"proxy"`
}

type Proxy struct {
	Type     string `json:"type"`
	Address  string `json:"address"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type Browser struct {
	Mode             string `json:"mode"`
	ExecutablePath   string `json:"executable_path"`
	WaitMilliseconds int    `json:"wait_milliseconds"`
	Headless         bool   `json:"headless"`
	AutoScroll       bool   `json:"auto_scroll"`
	ViewportWidth    int    `json:"viewport_width"`
	ViewportHeight   int    `json:"viewport_height"`
	AutoGeo          bool   `json:"auto_geo"`
	Locale           string `json:"locale"`
	Timezone         string `json:"timezone"`
}

type Scope struct {
	HostPolicy         string `json:"host_policy"`
	ExternalAssets     bool   `json:"external_assets"`
	KeepQuery          bool   `json:"keep_query"`
	RespectRobots      bool   `json:"respect_robots"`
	IncludePattern     string `json:"include_pattern"`
	ExcludePattern     string `json:"exclude_pattern"`
	CrawlNoFollowLinks bool   `json:"crawl_nofollow_links"`
}

type Output struct {
	CreateHostFolder bool   `json:"create_host_folder"`
	LocalizeLinks    bool   `json:"localize_links"`
	Replacement      string `json:"replacement"`
	PreserveTimes    bool   `json:"preserve_times"`
	SaveErrorPages   bool   `json:"save_error_pages"`
}

type Filters struct {
	HTML      bool `json:"html"`
	Styles    bool `json:"styles"`
	Scripts   bool `json:"scripts"`
	Images    bool `json:"images"`
	Fonts     bool `json:"fonts"`
	Media     bool `json:"media"`
	Documents bool `json:"documents"`
	Other     bool `json:"other"`
}

type Authentication struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Cookie   string `json:"cookie"`
}

func Default() Config {
	home, _ := os.UserHomeDir()
	destination := filepath.Join(home, "Downloads", "site-sucker")
	if home == "" {
		destination = "downloads"
	}
	return Config{
		Destination: destination,
		Limits: Limits{
			MaxDepthEnabled: true,
			MaxDepth:        3,
			MaxFilesEnabled: true,
			MaxFiles:        500,
			MaxSizeEnabled:  true,
			MaxSizeKB:       10000,
			MinImageWidth:   64,
			MinImageHeight:  64,
		},
		Network: Network{
			Connections:    10,
			Attempts:       2,
			TimeoutSeconds: 30,
			MaxRedirects:   10,
			UserAgent:      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 Safari/537.36 SiteSucker/1.0",
			Headers:        make(map[string]string),
			Proxy:          Proxy{Type: ProxyNone},
		},
		Browser: Browser{
			Mode:             ModeAuto,
			WaitMilliseconds: 1500,
			Headless:         true,
			AutoScroll:       true,
			ViewportWidth:    1440,
			ViewportHeight:   900,
			AutoGeo:          true,
		},
		Scope: Scope{
			HostPolicy:     ScopeHost,
			ExternalAssets: true,
			KeepQuery:      true,
			RespectRobots:  true,
		},
		Output: Output{
			CreateHostFolder: true,
			LocalizeLinks:    true,
			Replacement:      ReplaceNewer,
			PreserveTimes:    true,
		},
		Filters: Filters{
			HTML: true, Styles: true, Scripts: true, Images: true,
			Fonts: true, Media: true, Documents: true, Other: true,
		},
	}
}

func (c Config) Validate() error {
	start, err := url.Parse(strings.TrimSpace(c.StartURL))
	if err != nil || start.Host == "" || (start.Scheme != "http" && start.Scheme != "https") {
		return errors.New("enter a valid start URL with http:// or https://")
	}
	if strings.TrimSpace(c.Destination) == "" && !c.AskDestination {
		return errors.New("choose a destination folder")
	}
	if c.Network.Connections < 1 || c.Network.Connections > 100 {
		return errors.New("connections must be between 1 and 100")
	}
	if c.Network.Attempts < 1 || c.Network.Attempts > 20 {
		return errors.New("attempts must be between 1 and 20")
	}
	if c.Network.TimeoutSeconds < 1 || c.Network.TimeoutSeconds > 86400 {
		return errors.New("timeout must be between 1 and 86400 seconds")
	}
	if c.Network.DelayMilliseconds < 0 || c.Network.DelayMilliseconds > 3600000 {
		return errors.New("request delay must be between 0 and 3600000 ms")
	}
	if c.Network.MaxRedirects < 0 || c.Network.MaxRedirects > 100 {
		return errors.New("redirects must be between 0 and 100")
	}
	if c.Limits.MaxDepthEnabled && (c.Limits.MaxDepth < 0 || c.Limits.MaxDepth > 1000) {
		return errors.New("maximum depth must be between 0 and 1000")
	}
	if c.Limits.MaxFilesEnabled && (c.Limits.MaxFiles < 1 || c.Limits.MaxFiles > 10000000) {
		return errors.New("maximum files must be between 1 and 10000000")
	}
	if c.Limits.MaxSizeEnabled && c.Limits.MaxSizeKB < 1 {
		return errors.New("maximum file size must be greater than zero")
	}
	if c.Limits.MaxSizeKB > 1000000000 || c.Limits.MinSizeKB > 1000000000 {
		return errors.New("file size cannot exceed 1000000000 KB")
	}
	if c.Limits.MinSizeEnabled && c.Limits.MinSizeKB < 0 {
		return errors.New("minimum file size cannot be negative")
	}
	if c.Limits.MinSizeEnabled && c.Limits.MaxSizeEnabled && c.Limits.MinSizeKB > c.Limits.MaxSizeKB {
		return errors.New("minimum file size is greater than the maximum")
	}
	if c.Limits.MinImageEnabled && (c.Limits.MinImageWidth < 1 || c.Limits.MinImageHeight < 1) {
		return errors.New("minimum image dimensions must be greater than zero")
	}
	switch c.Browser.Mode {
	case ModeHTTP, ModeAuto, ModeBrowser:
	default:
		return fmt.Errorf("unsupported browser mode %q", c.Browser.Mode)
	}
	if c.Browser.WaitMilliseconds < 0 || c.Browser.WaitMilliseconds > 3600000 {
		return errors.New("browser wait must be between 0 and 3600000 ms")
	}
	if c.Browser.ViewportWidth < 320 || c.Browser.ViewportWidth > 16384 || c.Browser.ViewportHeight < 240 || c.Browser.ViewportHeight > 16384 {
		return errors.New("browser window size is out of range")
	}
	if c.Browser.Locale != "" {
		localePattern := regexp.MustCompile(`^[A-Za-z]{2,3}(?:-[A-Za-z]{2})?$`)
		if !localePattern.MatchString(c.Browser.Locale) {
			return errors.New("browser locale must look like it-IT or en-US")
		}
	}
	if c.Browser.Timezone != "" {
		if _, err := time.LoadLocation(c.Browser.Timezone); err != nil {
			return fmt.Errorf("unknown browser timezone %q", c.Browser.Timezone)
		}
	}
	switch c.Scope.HostPolicy {
	case ScopeHost, ScopeDomain, ScopeAnyHost:
	default:
		return fmt.Errorf("unsupported crawl scope %q", c.Scope.HostPolicy)
	}
	switch c.Output.Replacement {
	case ReplaceOverwrite, ReplaceSkip, ReplaceNewer:
	default:
		return fmt.Errorf("unsupported file replacement mode %q", c.Output.Replacement)
	}
	if c.Scope.IncludePattern != "" {
		if _, err := regexp.Compile(c.Scope.IncludePattern); err != nil {
			return fmt.Errorf("include regex: %w", err)
		}
	}
	if c.Scope.ExcludePattern != "" {
		if _, err := regexp.Compile(c.Scope.ExcludePattern); err != nil {
			return fmt.Errorf("exclude regex: %w", err)
		}
	}
	if c.Network.Proxy.Type != ProxyNone {
		if _, err := ProxyURL(c.Network.Proxy); err != nil {
			return err
		}
	}
	return nil
}

func ProxyURL(p Proxy) (*url.URL, error) {
	address := strings.TrimSpace(p.Address)
	if address == "" {
		return nil, errors.New("enter the proxy address")
	}
	if !strings.Contains(address, "://") {
		address = p.Type + "://" + address
	}
	u, err := url.Parse(address)
	if err != nil || u.Host == "" {
		return nil, errors.New("invalid proxy address")
	}
	switch p.Type {
	case ProxyHTTP, ProxyHTTPS, ProxySOCKS5:
		u.Scheme = p.Type
	default:
		return nil, fmt.Errorf("unsupported proxy type %q", p.Type)
	}
	if p.Username != "" {
		u.User = url.UserPassword(p.Username, p.Password)
	}
	return u, nil
}

func Decode(r io.Reader) (Config, error) {
	c := Default()
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return Config{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, errors.New("unexpected data after the JSON profile")
		}
		return Config{}, err
	}
	return c, c.Validate()
}

func Encode(w io.Writer, c Config) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(c)
}
