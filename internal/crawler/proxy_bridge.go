package crawler

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/maksqi/site-sucker/internal/config"
)

// proxyBridge presents an unauthenticated local HTTP proxy to Chromium and
// forwards its traffic through the user-selected HTTP or SOCKS5 proxy.
type proxyBridge struct {
	server   *http.Server
	listener net.Listener
	upstream *url.URL
	dial     func(context.Context, string, string) (net.Conn, error)
	insecure bool
	once     sync.Once
}

func newProxyBridge(cfg config.Config) (*proxyBridge, error) {
	if cfg.Network.Proxy.Type == config.ProxyNone {
		return nil, nil
	}
	upstream, err := config.ProxyURL(cfg.Network.Proxy)
	if err != nil {
		return nil, err
	}
	direct := &net.Dialer{Timeout: time.Duration(cfg.Network.TimeoutSeconds) * time.Second, KeepAlive: 30 * time.Second}
	dial := direct.DialContext
	if cfg.Network.Proxy.Type == config.ProxySOCKS5 {
		socksDialer, err := proxy.FromURL(upstream, direct)
		if err != nil {
			return nil, err
		}
		if contextDialer, ok := socksDialer.(proxy.ContextDialer); ok {
			dial = contextDialer.DialContext
		} else {
			dial = func(_ context.Context, network, address string) (net.Conn, error) {
				return socksDialer.Dial(network, address)
			}
		}
	}

	bridge := &proxyBridge{upstream: upstream, dial: dial, insecure: cfg.Network.InsecureTLS}
	transport := &http.Transport{
		DialContext:         dial,
		ForceAttemptHTTP2:   true,
		MaxIdleConns:        max(20, cfg.Network.Connections*2),
		IdleConnTimeout:     60 * time.Second,
		TLSHandshakeTimeout: time.Duration(cfg.Network.TimeoutSeconds) * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: cfg.Network.InsecureTLS}, //nolint:gosec // User-controlled compatibility option.
	}
	if cfg.Network.Proxy.Type == config.ProxyHTTP || cfg.Network.Proxy.Type == config.ProxyHTTPS {
		transport.Proxy = http.ProxyURL(upstream)
	}

	bridge.server = &http.Server{
		Handler:           http.HandlerFunc(bridge.handle(transport)),
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	bridge.listener = listener
	go func() { _ = bridge.server.Serve(listener) }()
	return bridge, nil
}

func (b *proxyBridge) URL() string {
	if b == nil || b.listener == nil {
		return ""
	}
	return "http://" + b.listener.Addr().String()
}

func (b *proxyBridge) Close() {
	if b == nil {
		return
	}
	b.once.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = b.server.Shutdown(ctx)
	})
}

func (b *proxyBridge) handle(transport *http.Transport) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			b.handleConnect(w, r)
			return
		}
		r.RequestURI = ""
		r.Header.Del("Proxy-Authorization")
		resp, err := transport.RoundTrip(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		copyHeaders(w.Header(), resp.Header)
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func (b *proxyBridge) handleConnect(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	var upstreamConn net.Conn
	var err error
	if b.upstream.Scheme == config.ProxyHTTP || b.upstream.Scheme == config.ProxyHTTPS {
		upstreamConn, err = b.connectViaHTTPProxy(ctx, r.Host)
	} else {
		upstreamConn, err = b.dial(ctx, "tcp", r.Host)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer upstreamConn.Close()

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "hijacking unavailable", http.StatusInternalServerError)
		return
	}
	clientConn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	defer clientConn.Close()
	_, _ = clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))

	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(upstreamConn, clientConn); done <- struct{}{} }()
	go func() { _, _ = io.Copy(clientConn, upstreamConn); done <- struct{}{} }()
	<-done
}

func (b *proxyBridge) connectViaHTTPProxy(ctx context.Context, target string) (net.Conn, error) {
	port := b.upstream.Port()
	if port == "" {
		port = "80"
		if b.upstream.Scheme == config.ProxyHTTPS {
			port = "443"
		}
	}
	address := net.JoinHostPort(b.upstream.Hostname(), port)
	conn, err := b.dial(ctx, "tcp", address)
	if err != nil {
		return nil, err
	}
	if b.upstream.Scheme == config.ProxyHTTPS {
		tlsConn := tls.Client(conn, &tls.Config{ServerName: b.upstream.Hostname(), InsecureSkipVerify: b.insecure}) //nolint:gosec // User-controlled compatibility option.
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, err
		}
		conn = tlsConn
	}
	req := &http.Request{Method: http.MethodConnect, URL: &url.URL{Opaque: target}, Host: target, Header: make(http.Header)}
	if b.upstream.User != nil {
		password, _ := b.upstream.User.Password()
		credentials := base64.StdEncoding.EncodeToString([]byte(b.upstream.User.Username() + ":" + password))
		req.Header.Set("Proxy-Authorization", "Basic "+credentials)
	}
	if err := req.WriteProxy(conn); err != nil {
		conn.Close()
		return nil, err
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		conn.Close()
		return nil, fmt.Errorf("upstream proxy CONNECT: %s", resp.Status)
	}
	return conn, nil
}

func copyHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
