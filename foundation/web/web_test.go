package web_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"github.com/cbrophy/land_trakker/foundation/web"
)

const testSecret = "test-secret-key"

func TestIsAuthenticated_noSession(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	if web.IsAuthenticated(r, testSecret) {
		t.Fatal("expected unauthenticated with no cookie")
	}
}

func TestSetSession_roundtrip(t *testing.T) {
	w := httptest.NewRecorder()
	web.SetSession(w, testSecret)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie to be set")
	}

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(cookies[0])

	if !web.IsAuthenticated(r, testSecret) {
		t.Fatal("expected authenticated after SetSession")
	}
}

func TestClearSession(t *testing.T) {
	w := httptest.NewRecorder()
	web.SetSession(w, testSecret)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w.Result().Cookies() {
		r.AddCookie(c)
	}

	w2 := httptest.NewRecorder()
	web.ClearSession(w2)

	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range w2.Result().Cookies() {
		r2.AddCookie(c)
	}
	if web.IsAuthenticated(r2, testSecret) {
		t.Fatal("expected unauthenticated after ClearSession")
	}
}

func TestRequireAuth_redirectsUnauthenticated(t *testing.T) {
	handler := web.RequireAuth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}

func TestRequireAuth_allowsAuthenticated(t *testing.T) {
	handler := web.RequireAuth(testSecret)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	r := httptest.NewRequest(http.MethodGet, "/protected", nil)
	ws := httptest.NewRecorder()
	web.SetSession(ws, testSecret)
	for _, c := range ws.Result().Cookies() {
		r.AddCookie(c)
	}

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, w.Code)
	}
}

func TestLoginHandler_get(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("secret"), bcrypt.MinCost)
	h := web.LoginHandler(string(hash), testSecret)

	r := httptest.NewRequest(http.MethodGet, "/login", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Land Trakker") {
		t.Fatal("expected login page body to contain 'Land Trakker'")
	}
}

func TestLoginHandler_post_validPassword(t *testing.T) {
	password := "mysecret"
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	h := web.LoginHandler(string(hash), testSecret)

	form := url.Values{"password": {password}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/" {
		t.Fatalf("expected redirect to /, got %q", loc)
	}

	// Verify cookie was set and is valid
	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie after successful login")
	}
	r2 := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, c := range cookies {
		r2.AddCookie(c)
	}
	if !web.IsAuthenticated(r2, testSecret) {
		t.Fatal("expected valid session after login")
	}
}

func TestLoginHandler_post_invalidPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct"), bcrypt.MinCost)
	h := web.LoginHandler(string(hash), testSecret)

	form := url.Values{"password": {"wrong"}}
	r := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	h := web.HealthHandler()

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "OK") {
		t.Fatalf("expected body to contain 'OK', got %q", w.Body.String())
	}
}

func TestLogoutHandler(t *testing.T) {
	h := web.LogoutHandler()

	r := httptest.NewRequest(http.MethodGet, "/logout", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("expected %d, got %d", http.StatusSeeOther, w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "/login" {
		t.Fatalf("expected redirect to /login, got %q", loc)
	}
}
