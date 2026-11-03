package crawler

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/proxy"

	"github.com/maksqi/site-sucker/internal/config"
)

type httpFetcher struct {
	cfg        config.Config
	client     *http.Client
	authOrigin *url.URL
}

type skipError struct {
	reason string
}

func (e *skipError) Error() string { return e.reason }

func newHTTPFetcher(cfg config.Config) (*httpFetcher, error) {
	dialer := &net.Dialer{Timeout: time.Duration(cfg.Network.TimeoutSeconds) * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          max(20, cfg.Network.Connections*2),
		MaxIdleConnsPerHost:   max(10, cfg.Network.Connections),
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   time.Duration(cfg.Network.TimeoutSeconds) * time.Second,
		ExpectContinueTimeout: time.Second,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: cfg.Network.InsecureTLS}, //nolint:gosec // User-controlled compatibility option.
	}

	if cfg.Network.Proxy.Type != config.ProxyNone {
		proxyURL, err := config.ProxyURL(cfg.Network.Proxy)
		if err != nil {
			return nil, err
		}
		if cfg.Network.Proxy.Type == config.ProxySOCKS5 {
			proxyDialer, err := proxy.FromURL(proxyURL, dialer)
			if err != nil {
				return nil, fmt.Errorf("create SOCKS5 dialer: %w", err)
			}
			if contextDialer, ok := proxyDialer.(proxy.ContextDialer); ok {
				transport.DialContext = contextDialer.DialContext
			} else {
				transport.DialContext = func(_ context.Context, network, address string) (net.Conn, error) {
					return proxyDialer.Dial(network, address)
				}
			}
		} else {
			transport.Proxy = http.ProxyURL(proxyURL)
		}
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	authOrigin, _ := url.Parse(strings.TrimSpace(cfg.StartURL))
	client := &http.Client{
		Transport: transport,
		Timeout:   time.Duration(cfg.Network.TimeoutSeconds) * time.Second,
		Jar:       jar,
	}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > cfg.Network.MaxRedirects {
			return errors.New("too many redirects")
		}
		return nil
	}
	if authOrigin != nil && authOrigin.Host != "" {
		jar.SetCookies(authOrigin, parseCookieHeader(cfg.Authentication.Cookie))
	}
	return &httpFetcher{cfg: cfg, client: client, authOrigin: authOrigin}, nil
}

func (f *httpFetcher) fetch(ctx context.Context, target *url.URL) (fetchResult, error) {
	var lastErr error
	for attempt := 1; attempt <= f.cfg.Network.Attempts; attempt++ {
		if attempt > 1 {
			backoff := time.Duration(attempt-1) * 300 * time.Millisecond
			select {
			case <-ctx.Done():
				return fetchResult{}, ctx.Err()
			case <-time.After(backoff):
			}
		}
		result, retry, err := f.fetchOnce(ctx, target)
		if err == nil {
			if retry && attempt < f.cfg.Network.Attempts {
				continue
			}
			return result, nil
		}
		lastErr = err
		if !retry {
			break
		}
	}
	return fetchResult{}, lastErr
}

func (f *httpFetcher) fetchOnce(ctx context.Context, target *url.URL) (fetchResult, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return fetchResult{}, false, err
	}
	req.Header.Set("User-Agent", f.cfg.Network.UserAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	for key, value := range f.cfg.Network.Headers {
		if strings.TrimSpace(key) != "" && (sameOrigin(target, f.authOrigin) || !sensitiveHeader(key)) {
			req.Header.Set(key, value)
		}
	}
	if f.cfg.Authentication.Username != "" && sameOrigin(target, f.authOrigin) {
		req.SetBasicAuth(f.cfg.Authentication.Username, f.cfg.Authentication.Password)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return fetchResult{}, true, err
	}
	defer resp.Body.Close()

	retryStatus := false
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		retryStatus = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		if !f.cfg.Output.SaveErrorPages {
			return fetchResult{}, retryStatus, fmt.Errorf("HTTP %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
	}

	maxBytes := int64(0)
	if f.cfg.Limits.MaxSizeEnabled {
		maxBytes = f.cfg.Limits.MaxSizeKB * 1024
		if length, _ := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); length > maxBytes {
			return fetchResult{}, false, &skipError{reason: fmt.Sprintf("file above the size limit: %d KB", length/1024)}
		}
	}
	reader := io.Reader(resp.Body)
	if maxBytes > 0 {
		reader = io.LimitReader(resp.Body, maxBytes+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return fetchResult{}, true, err
	}
	if maxBytes > 0 && int64(len(body)) > maxBytes {
		return fetchResult{}, false, &skipError{reason: fmt.Sprintf("file above the size limit: > %d KB", f.cfg.Limits.MaxSizeKB)}
	}
	if f.cfg.Limits.MinSizeEnabled && int64(len(body)) < f.cfg.Limits.MinSizeKB*1024 {
		return fetchResult{}, false, &skipError{reason: fmt.Sprintf("file below the size limit: %d bytes", len(body))}
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	lastModified, _ := http.ParseTime(resp.Header.Get("Last-Modified"))
	return fetchResult{
		Body:         body,
		ContentType:  contentType,
		StatusCode:   resp.StatusCode,
		FinalURL:     resp.Request.URL,
		LastModified: lastModified,
	}, retryStatus, nil
}

func (f *httpFetcher) cookies(target *url.URL) []*http.Cookie {
	if f.client.Jar == nil {
		return nil
	}
	return f.client.Jar.Cookies(target)
}

func (f *httpFetcher) setCookies(target *url.URL, cookies []*http.Cookie) {
	if f.client.Jar != nil && len(cookies) > 0 {
		f.client.Jar.SetCookies(target, cookies)
	}
}

func (f *httpFetcher) close() {
	f.client.CloseIdleConnections()
}

func sameOrigin(first, second *url.URL) bool {
	if first == nil || second == nil {
		return false
	}
	return strings.EqualFold(first.Scheme, second.Scheme) && strings.EqualFold(first.Host, second.Host)
}

func sensitiveHeader(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "authorization", "cookie", "proxy-authorization":
		return true
	default:
		return false
	}
}
