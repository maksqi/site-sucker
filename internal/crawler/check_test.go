package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestCheckProxyDoesNotRequireSiteURL(t *testing.T) {
	var receivedAuthorization, receivedCookie, receivedCustomHeader string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuthorization = r.Header.Get("Authorization")
		receivedCookie = r.Header.Get("Cookie")
		receivedCustomHeader = r.Header.Get("X-Site-Secret")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "ip":"203.0.113.7",
            "success":true,
            "country":"Italy",
            "country_code":"IT",
            "region":"Sicily",
            "city":"Aci Catena",
            "connection":{"isp":"NetLabs"}
        }`))
	}))
	defer server.Close()

	cfg := config.Default()
	cfg.StartURL = ""
	cfg.Destination = ""
	cfg.Authentication = config.Authentication{Username: "site-user", Password: "site-password", Cookie: "session=secret"}
	cfg.Network.Headers = map[string]string{"X-Site-Secret": "secret"}
	profile, err := checkProxyAt(context.Background(), cfg, server.URL)
	if err != nil {
		t.Fatalf("checkProxyAt() error = %v", err)
	}
	if profile.IP != "203.0.113.7" || profile.Country != "Italy" || profile.CountryCode != "IT" {
		t.Fatalf("profile = %#v", profile)
	}
	if profile.Flag == "" || profile.ISP != "NetLabs" {
		t.Fatalf("profile = %#v", profile)
	}
	if summary := profile.Summary(); !strings.Contains(summary, "Italy") || !strings.Contains(summary, "NetLabs") {
		t.Fatalf("summary = %q", summary)
	}
	if receivedAuthorization != "" || receivedCookie != "" || receivedCustomHeader != "" {
		t.Fatalf("site credentials leaked to proxy check: Authorization=%q Cookie=%q X-Site-Secret=%q", receivedAuthorization, receivedCookie, receivedCustomHeader)
	}
}

func TestFlagForCountry(t *testing.T) {
	if flag := countryFlag("IT"); len([]rune(flag)) != 2 {
		t.Fatalf("countryFlag(IT) = %q", flag)
	}
}
