package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/cbrophy/land_trakker/foundation/config"
	"github.com/cbrophy/land_trakker/foundation/web"
)

func newRouter(cfg *config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// Public routes
	r.Get("/health", web.HealthHandler())
	r.Get("/login", web.LoginHandler(cfg.Server.AdminPasswordHash, cfg.Server.SessionSecret))
	r.Post("/login", web.LoginHandler(cfg.Server.AdminPasswordHash, cfg.Server.SessionSecret))
	r.Get("/logout", web.LogoutHandler())

	// Authenticated routes
	r.Group(func(r chi.Router) {
		r.Use(web.RequireAuth(cfg.Server.SessionSecret))
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.Write([]byte("Land Trakker\n"))
		})
	})

	return r
}
