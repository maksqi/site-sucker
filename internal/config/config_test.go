package config

import (
	"bytes"
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

func TestBrowserGeoValidation(t *testing.T) {
	cfg := Default()
	cfg.StartURL = "https://example.com/"
	cfg.Browser.Locale = "it-IT"
	cfg.Browser.Timezone = "Europe/Rome"
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
	cfg.Browser.Timezone = "Invalid/Timezone"
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() accepted an invalid timezone")
	}
}
