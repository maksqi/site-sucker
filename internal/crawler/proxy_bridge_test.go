package crawler

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/maksqi/site-sucker/internal/config"
)

func TestProxyBridgeConnectsThroughAuthenticatedHTTPProxy(t *testing.T) {
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("through-proxy"))
	}))
	defer target.Close()

	expectedAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:secret"))
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodConnect {
			http.Error(w, "CONNECT required", http.StatusMethodNotAllowed)
			return
		}
		if r.Header.Get("Proxy-Authorization") != expectedAuth {
			w.Header().Set("Proxy-Authenticate", `Basic realm="test"`)
			http.Error(w, "proxy auth required", http.StatusProxyAuthRequired)
			return
		}
		destination, err := net.DialTimeout("tcp", r.Host, 5*time.Second)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer destination.Close()
		client, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			return
		}
		defer client.Close()
		_, _ = client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
		done := make(chan struct{}, 2)
		go func() { _, _ = io.Copy(destination, client); done <- struct{}{} }()
		go func() { _, _ = io.Copy(client, destination); done <- struct{}{} }()
		<-done
	}))
	defer upstream.Close()

	cfg := config.Default()
	cfg.StartURL = target.URL
	cfg.Network.InsecureTLS = true
	cfg.Network.Proxy = config.Proxy{Type: config.ProxyHTTP, Address: upstream.URL, Username: "user", Password: "secret"}
	bridge, err := newProxyBridge(cfg)
	if err != nil {
		t.Fatalf("newProxyBridge() error = %v", err)
	}
	defer bridge.Close()
	bridgeURL, _ := url.Parse(bridge.URL())
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(bridgeURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec // Local TLS test server.
		},
		Timeout: 10 * time.Second,
	}
	request, _ := http.NewRequestWithContext(context.Background(), http.MethodGet, target.URL, nil)
	response, err := client.Do(request)
	if err != nil {
		t.Fatalf("request through bridge error = %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "through-proxy" {
		t.Fatalf("response body = %q", body)
	}
}
