package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfigRoundTrip(t *testing.T) {
	original := Default()
	original.StartURL = "https://example.com/"
	original.Network.Proxy = Proxy{Type: ProxySOCKS5, Address: "127.0.0.1:1080", Username: "user", Password: "secret"}
	var encoded bytes.Buffer
	if err := Encode(&encoded, original); err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	decoded, err := Decode(&encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.StartURL != original.StartURL || decoded.Network.Proxy.Type != ProxySOCKS5 {
		t.Fatalf("decoded config does not match: %#v", decoded)
	}
}

func TestAskDestinationAllowsEmptyPath(t *testing.T) {
	cfg := Default()
	cfg.StartURL = "https://example.com/"
	cfg.Destination = ""
	cfg.AskDestination = true
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestDecodeRejectsTrailingJSON(t *testing.T) {
	cfg := Default()
	cfg.StartURL = "https://example.com/"
	var encoded bytes.Buffer
	if err := Encode(&encoded, cfg); err != nil {
		t.Fatal(err)
	}
	encoded.WriteString(`{"extra":true}`)
	if _, err := Decode(&encoded); err == nil {
		t.Fatal("Decode() accepted trailing JSON")
	}
}

func TestDecodeDropsLegacyBrowserKeys(t *testing.T) {
	legacy := `{"start_url":"https://example.com/","browser":{"mode":"browser","auto_geo":true,"locale":"it-IT","timezone":"Europe/Rome"}}`
	cfg, err := Decode(strings.NewReader(legacy))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if cfg.Browser.Mode != ModeBrowser || cfg.Browser.ViewportWidth != Default().Browser.ViewportWidth {
		t.Fatalf("decoded browser settings = %#v", cfg.Browser)
	}
	for _, unknown := range []string{
		`{"start_url":"https://example.com/","browser":{"mode":"browser","fingerprint":true}}`,
		`{"start_url":"https://example.com/","locale":"it-IT"}`,
	} {
		if _, err := Decode(strings.NewReader(unknown)); err == nil {
			t.Fatalf("Decode() accepted an unknown key: %s", unknown)
		}
	}
}
