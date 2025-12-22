package server

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aezizhu/LuciCodex/internal/config"
)

func TestServer_Health(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	s.mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := "ok"
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}
}

func TestServer_Plan_InvalidMethod(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("GET", "/v1/plan", nil)
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()

	s.mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}
}

func TestServer_Plan_EmptyBody(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	req, _ := http.NewRequest("POST", "/v1/plan", bytes.NewReader([]byte{}))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()

	s.mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestServer_Plan_MissingPrompt(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	body := []byte(`{"model": "test"}`)
	req, _ := http.NewRequest("POST", "/v1/plan", bytes.NewReader(body))
	req.Header.Set("X-Auth-Token", s.GetToken())
	rr := httptest.NewRecorder()

	s.mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestServer_Unauthorized(t *testing.T) {
	cfg := config.Config{}
	s := New(cfg)

	// Request without auth token
	req, _ := http.NewRequest("POST", "/v1/plan", bytes.NewReader([]byte(`{}`)))
	rr := httptest.NewRecorder()

	s.mux.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusUnauthorized {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusUnauthorized)
	}
}
