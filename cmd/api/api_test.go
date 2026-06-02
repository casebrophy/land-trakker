package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cbrophy/land_trakker/foundation/config"
	"github.com/cbrophy/land_trakker/foundation/web"
)

func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Server.SessionSecret = "test-secret"
	// empty admin_password_hash means login will always fail — OK for route tests
	return cfg
}

func TestRoutes_health(t *testing.T) {
	lc := web.NewLogCapture(100)
	r := newRouter(newTestConfig(), nil, nil, nil, nil, lc, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRoutes_unauthenticated_redirectsToLogin(t *testing.T) {
	lc := web.NewLogCapture(100)
	r := newRouter(newTestConfig(), nil, nil, nil, nil, lc, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestRoutes_loginGet(t *testing.T) {
	lc := web.NewLogCapture(100)
	r := newRouter(newTestConfig(), nil, nil, nil, nil, lc, nil, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestRoutes_authenticated_rootOK(t *testing.T) {
	cfg := newTestConfig()
	lc := web.NewLogCapture(100)
	r := newRouter(cfg, nil, nil, nil, nil, lc, nil, nil, nil)

	// Pre-set a valid session cookie
	ws := httptest.NewRecorder()
	web.SetSession(ws, cfg.Server.SessionSecret)
	cookies := ws.Result().Cookies()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// nil querier returns 503; the important thing is auth passed (no redirect)
	if w.Code == http.StatusSeeOther {
		t.Fatalf("expected authenticated request not to redirect, got %d", w.Code)
	}
}
