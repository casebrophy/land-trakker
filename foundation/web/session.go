package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
)

const sessionCookieName = "lt_session"

func signedToken(secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte("authenticated"))
	return base64.URLEncoding.EncodeToString(mac.Sum(nil))
}

// SetSession creates and sets a signed session cookie.
func SetSession(w http.ResponseWriter, secret string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    signedToken(secret),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// ClearSession deletes the session cookie.
func ClearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   sessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// IsAuthenticated checks if the request has a valid session cookie.
func IsAuthenticated(r *http.Request, secret string) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return false
	}
	expected := signedToken(secret)
	return hmac.Equal([]byte(cookie.Value), []byte(expected))
}
