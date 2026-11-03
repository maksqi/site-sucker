package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/maksqi/site-sucker/internal/config"
)

const proxyCheckURL = "https://ipwho.is/"

type ProxyProfile struct {
	IP          string
	Country     string
	CountryCode string
	Region      string
	City        string
	Flag        string
	Timezone    string
	Locale      string
	ISP         string
}

func (p ProxyProfile) Summary() string {
	country := strings.TrimSpace(p.Country)
	if country == "" {
		country = "unknown"
	}
	location := country
	if p.CountryCode != "" {
		location += " (" + p.CountryCode + ")"
	}
	if p.Flag != "" {
		location = p.Flag + "  " + location
	}
	lines := []string{
		"Country: " + location,
		"External IP: " + fallbackText(p.IP),
		"City/region: " + fallbackText(strings.Trim(strings.Join([]string{p.City, p.Region}, ", "), ", ")),
		"Timezone: " + fallbackText(p.Timezone),
		"Chromium language: " + fallbackText(p.Locale),
	}
	if p.ISP != "" {
		lines = append(lines, "ISP: "+p.ISP)
	}
	return strings.Join(lines, "\n")
}

func CheckProxy(ctx context.Context, cfg config.Config) (ProxyProfile, error) {
	return checkProxyAt(ctx, cfg, proxyCheckURL)
}

func applyAutomaticProxyGeo(ctx context.Context, cfg *config.Config) (ProxyProfile, error) {
	if !cfg.Browser.AutoGeo || cfg.Network.Proxy.Type == config.ProxyNone {
		return ProxyProfile{}, nil
	}
	profile, err := CheckProxy(ctx, *cfg)
	if err != nil {
		return ProxyProfile{}, err
	}
	if profile.Locale != "" {
		cfg.Browser.Locale = profile.Locale
		if cfg.Network.Headers == nil {
			cfg.Network.Headers = make(map[string]string)
		}
		if !hasHeader(cfg.Network.Headers, "Accept-Language") {
			cfg.Network.Headers["Accept-Language"] = acceptLanguageForLocale(profile.Locale)
		}
	}
	if profile.Timezone != "" {
		cfg.Browser.Timezone = profile.Timezone
	}
	return profile, nil
}

func checkProxyAt(ctx context.Context, cfg config.Config, endpoint string) (ProxyProfile, error) {
	testConfig := config.Default()
	testConfig.StartURL = endpoint
	testConfig.Destination = os.TempDir()
	testConfig.Browser.Mode = config.ModeHTTP
	testConfig.Network = cfg.Network
	// A proxy check must never send credentials intended for the downloaded site.
	testConfig.Network.Headers = make(map[string]string)
	testConfig.Authentication = config.Authentication{}
	testConfig.Limits.MinSizeEnabled = false
	testConfig.Limits.MaxSizeEnabled = true
	testConfig.Limits.MaxSizeKB = 1024
	if err := testConfig.Validate(); err != nil {
		return ProxyProfile{}, err
	}
	target, err := url.Parse(endpoint)
	if err != nil || target.Host == "" {
		return ProxyProfile{}, errors.New("invalid check service address")
	}
	fetcher, err := newHTTPFetcher(testConfig)
	if err != nil {
		return ProxyProfile{}, err
	}
	defer fetcher.close()
	result, err := fetcher.fetch(ctx, target)
	if err != nil {
		return ProxyProfile{}, err
	}
	var response struct {
		IP          string `json:"ip"`
		Success     *bool  `json:"success"`
		Message     string `json:"message"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Region      string `json:"region"`
		City        string `json:"city"`
		Flag        struct {
			Emoji string `json:"emoji"`
		} `json:"flag"`
		Connection struct {
			ISP string `json:"isp"`
			Org string `json:"org"`
		} `json:"connection"`
		Timezone struct {
			ID string `json:"id"`
		} `json:"timezone"`
	}
	if err := json.Unmarshal(result.Body, &response); err != nil {
		return ProxyProfile{}, fmt.Errorf("geolocation service response: %w", err)
	}
	if response.Success != nil && !*response.Success {
		return ProxyProfile{}, fmt.Errorf("geolocation service: %s", fallbackText(response.Message))
	}
	if response.IP == "" {
		return ProxyProfile{}, errors.New("geolocation service returned no IP")
	}
	code := strings.ToUpper(strings.TrimSpace(response.CountryCode))
	flag := response.Flag.Emoji
	if flag == "" {
		flag = countryFlag(code)
	}
	isp := response.Connection.ISP
	if isp == "" {
		isp = response.Connection.Org
	}
	return ProxyProfile{
		IP: response.IP, Country: response.Country, CountryCode: code,
		Region: response.Region, City: response.City, Flag: flag,
		Timezone: response.Timezone.ID, Locale: localeForCountry(code), ISP: isp,
	}, nil
}

func localeForCountry(countryCode string) string {
	locales := map[string]string{
		"AL": "sq-AL", "AR": "es-AR", "AT": "de-AT", "AU": "en-AU",
		"BE": "nl-BE", "BG": "bg-BG", "BR": "pt-BR", "CA": "en-CA",
		"CH": "de-CH", "CL": "es-CL", "CN": "zh-CN", "CO": "es-CO",
		"CZ": "cs-CZ", "DE": "de-DE", "DK": "da-DK", "EE": "et-EE",
		"ES": "es-ES", "FI": "fi-FI", "FR": "fr-FR", "GB": "en-GB",
		"GR": "el-GR", "HK": "zh-HK", "HR": "hr-HR", "HU": "hu-HU",
		"ID": "id-ID", "IE": "en-IE", "IL": "he-IL", "IN": "hi-IN",
		"IS": "is-IS", "IT": "it-IT", "JP": "ja-JP", "KR": "ko-KR",
		"LT": "lt-LT", "LU": "fr-LU", "LV": "lv-LV", "MX": "es-MX",
		"MY": "ms-MY", "NL": "nl-NL", "NO": "nb-NO", "NZ": "en-NZ",
		"PE": "es-PE", "PH": "en-PH", "PK": "ur-PK", "PL": "pl-PL",
		"PT": "pt-PT", "RO": "ro-RO", "RS": "sr-RS", "RU": "ru-RU",
		"SE": "sv-SE", "SG": "en-SG", "SK": "sk-SK", "SI": "sl-SI",
		"TH": "th-TH", "TR": "tr-TR", "TW": "zh-TW", "UA": "uk-UA",
		"US": "en-US", "VN": "vi-VN", "ZA": "en-ZA",
	}
	if locale := locales[strings.ToUpper(countryCode)]; locale != "" {
		return locale
	}
	if len(countryCode) == 2 {
		return "en-" + strings.ToUpper(countryCode)
	}
	return "en-US"
}

func countryFlag(countryCode string) string {
	code := strings.ToUpper(countryCode)
	if len(code) != 2 || code[0] < 'A' || code[0] > 'Z' || code[1] < 'A' || code[1] > 'Z' {
		return ""
	}
	return string([]rune{
		rune(0x1F1E6) + rune(code[0]-'A'),
		rune(0x1F1E6) + rune(code[1]-'A'),
	})
}

func fallbackText(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}
	return value
}
