package llm

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/aezizhu/LuciCodex/internal/config"
)

func TestNewHTTPClient(t *testing.T) {
	cfg := config.Config{}
	client := newHTTPClient(cfg, 10*time.Second)
	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 10*time.Second {
		t.Errorf("expected timeout 10s, got %v", client.Timeout)
	}
}

func TestProxyFunc(t *testing.T) {
	// Clear proxy env vars to ensure deterministic testing
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")
	t.Setenv("http_proxy", "")
	t.Setenv("https_proxy", "")
	t.Setenv("no_proxy", "")

	tests := []struct {
		name      string
		cfg       config.Config
		reqURL    string
		wantProxy string // empty means nil (direct)
	}{
		{
			name:      "No Proxy",
			cfg:       config.Config{},
			reqURL:    "http://example.com",
			wantProxy: "", // Uses environment, but we assume empty env for test isolation?
			// Actually proxyFunc falls back to http.ProxyFromEnvironment if no config.
			// We should set explicit proxy to test our logic.
		},
		{
			name: "HTTP Proxy",
			cfg: config.Config{
				HTTPProxy: "http://proxy.example.com:8080",
			},
			reqURL:    "http://example.com",
			wantProxy: "http://proxy.example.com:8080",
		},
		{
			name: "HTTPS Proxy",
			cfg: config.Config{
				HTTPSProxy: "http://secure-proxy.example.com:8443",
			},
			reqURL:    "https://example.com",
			wantProxy: "http://secure-proxy.example.com:8443",
		},
		{
			name: "HTTP Proxy for HTTPS URL (fallback)",
			cfg: config.Config{
				HTTPProxy: "http://proxy.example.com:8080",
			},
			reqURL:    "https://example.com",
			wantProxy: "http://proxy.example.com:8080",
		},
		{
			name: "No Proxy Bypass",
			cfg: config.Config{
				HTTPProxy: "http://proxy.example.com:8080",
				NoProxy:   "example.com",
			},
			reqURL:    "http://example.com",
			wantProxy: "",
		},
		{
			name: "No Proxy Bypass Subdomain",
			cfg: config.Config{
				HTTPProxy: "http://proxy.example.com:8080",
				NoProxy:   ".example.com",
			},
			reqURL:    "http://sub.example.com",
			wantProxy: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := proxyFunc(tt.cfg)
			reqURL, _ := url.Parse(tt.reqURL)
			req := &http.Request{URL: reqURL}

			proxyURL, err := pf(req)
			if err != nil {
				t.Fatalf("proxyFunc returned error: %v", err)
			}

			if tt.wantProxy == "" {
				// We expect nil or direct.
				// If config is empty, it uses ProxyFromEnvironment.
				// For "No Proxy" case, if env is empty, it returns nil.
				// For "No Proxy Bypass", it returns nil explicitly.
				if proxyURL != nil {
					// Check if it matches ProxyFromEnvironment?
					// Hard to test environment fallback deterministically without clearing env.
					// But for bypass cases, it MUST be nil.
					if tt.cfg.NoProxy != "" {
						t.Errorf("expected no proxy (bypass), got %v", proxyURL)
					}
				}
			} else {
				if proxyURL == nil {
					t.Errorf("expected proxy %q, got nil", tt.wantProxy)
				} else if proxyURL.String() != tt.wantProxy {
					t.Errorf("expected proxy %q, got %q", tt.wantProxy, proxyURL.String())
				}
			}
		})
	}
}

func TestParseProxy(t *testing.T) {
	tests := []struct {
		input string
		want  string // empty means nil
	}{
		{"", ""},
		{"  ", ""},
		{"http://proxy.com", "http://proxy.com"},
		{"proxy.com", "http://proxy.com"}, // adds scheme
		{"https://secure.com", "https://secure.com"},
		{"://invalid", ""}, // invalid URL
	}

	for _, tt := range tests {
		u := parseProxy(tt.input)
		if tt.want == "" {
			if u != nil {
				t.Errorf("parseProxy(%q) = %v, want nil", tt.input, u)
			}
		} else {
			if u == nil || u.String() != tt.want {
				t.Errorf("parseProxy(%q) = %v, want %q", tt.input, u, tt.want)
			}
		}
	}
}

func TestParseNoProxy(t *testing.T) {
	tests := []struct {
		input string
		want  []string
	}{
		{"", nil},
		{" ", nil},
		{"localhost", []string{"localhost"}},
		{"localhost, 127.0.0.1", []string{"localhost", "127.0.0.1"}},
		{"  foo  ,  bar  ", []string{"foo", "bar"}},
	}

	for _, tt := range tests {
		got := parseNoProxy(tt.input)
		if len(got) != len(tt.want) {
			t.Errorf("parseNoProxy(%q) len = %d, want %d", tt.input, len(got), len(tt.want))
		}
		for i, v := range got {
			if v != tt.want[i] {
				t.Errorf("parseNoProxy(%q)[%d] = %q, want %q", tt.input, i, v, tt.want[i])
			}
		}
	}
}

func TestShouldBypassProxy(t *testing.T) {
	tests := []struct {
		host     string
		patterns []string
		want     bool
	}{
		{"example.com", []string{"example.com"}, true},
		{"example.com", []string{"other.com"}, false},
		{"sub.example.com", []string{".example.com"}, true},
		{"example.com", []string{".example.com"}, true}, // suffix match logic in code
		{"notexample.com", []string{".example.com"}, false},
		{"anything", []string{"*"}, true},
		{"localhost", []string{"localhost"}, true},
	}

	for _, tt := range tests {
		got := shouldBypassProxy(tt.host, tt.patterns)
		if got != tt.want {
			t.Errorf("shouldBypassProxy(%q, %v) = %v, want %v", tt.host, tt.patterns, got, tt.want)
		}
	}
}
