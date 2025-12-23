package llm

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
)

// maxErrorBodySize limits error response reads to prevent memory exhaustion
const maxErrorBodySize = 4096

// readErrorBody reads up to maxErrorBodySize bytes from the response body
// to prevent memory exhaustion from large error responses on embedded systems.
func readErrorBody(body io.Reader) []byte {
	data, _ := io.ReadAll(io.LimitReader(body, maxErrorBodySize))
	return data
}

func newHTTPClient(cfg config.Config, timeout time.Duration) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = proxyFunc(cfg)
	// Optimize for embedded routers with limited resources
	transport.MaxIdleConns = 10
	transport.MaxIdleConnsPerHost = 5
	transport.IdleConnTimeout = 60 * time.Second
	transport.DisableCompression = false // Enable compression for bandwidth savings
	transport.ForceAttemptHTTP2 = false  // HTTP/1.1 is more reliable on embedded systems

	// CRITICAL: Completely disable HTTP/2 to fix protocol mismatch errors
	// 1. Set TLSNextProto to empty map - prevents HTTP/2 upgrade after TLS
	transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)

	// 2. Configure TLS to only advertise HTTP/1.1 in ALPN
	// This prevents the server from responding with HTTP/2 frames
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	}
	transport.TLSClientConfig.NextProtos = []string{"http/1.1"}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}
}

func proxyFunc(cfg config.Config) func(*http.Request) (*url.URL, error) {
	httpProxyURL := parseProxy(cfg.HTTPProxy)
	httpsProxyURL := parseProxy(cfg.HTTPSProxy)
	noProxyList := parseNoProxy(cfg.NoProxy)

	if httpProxyURL == nil && httpsProxyURL == nil && len(noProxyList) == 0 {
		return http.ProxyFromEnvironment
	}

	return func(req *http.Request) (*url.URL, error) {
		host := strings.ToLower(req.URL.Hostname())
		if host != "" && shouldBypassProxy(host, noProxyList) {
			return nil, nil
		}
		switch req.URL.Scheme {
		case "https":
			if httpsProxyURL != nil {
				return httpsProxyURL, nil
			}
		case "http":
			if httpProxyURL != nil {
				return httpProxyURL, nil
			}
		}
		if httpsProxyURL != nil {
			return httpsProxyURL, nil
		}
		if httpProxyURL != nil {
			return httpProxyURL, nil
		}
		return nil, nil
	}
}

func parseProxy(raw string) *url.URL {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return nil
	}
	return u
}

func parseNoProxy(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	res := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.ToLower(strings.TrimSpace(part)); trimmed != "" {
			res = append(res, trimmed)
		}
	}
	return res
}

func shouldBypassProxy(host string, patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "*" {
			return true
		}
		if strings.HasPrefix(pattern, ".") {
			if strings.HasSuffix(host, pattern) || host == strings.TrimPrefix(pattern, ".") {
				return true
			}
			continue
		}
		if pattern == host {
			return true
		}
		if strings.Contains(pattern, ".") {
			if strings.HasSuffix(host, "."+pattern) {
				return true
			}
		}
	}
	return false
}
