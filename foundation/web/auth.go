package web

import (
	"embed"
	"html/template"
	"net/http"

	"golang.org/x/crypto/bcrypt"
)

//go:embed templates
var templateFS embed.FS

var loginTmpl = template.Must(template.ParseFS(templateFS, "templates/login.html"))

// RequireAuth returns middleware that redirects unauthenticated requests to /login.
func RequireAuth(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !IsAuthenticated(r, secret) {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LoginHandler handles GET (render form) and POST (check password, set cookie).
func LoginHandler(passwordHash, secret string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			password := r.FormValue("password")
			if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				loginTmpl.Execute(w, map[string]any{"Error": "Invalid password"})
				return
			}
			SetSession(w, secret)
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		loginTmpl.Execute(w, nil)
	}
}

// LogoutHandler clears the session cookie and redirects to /login.
func LogoutHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ClearSession(w)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
