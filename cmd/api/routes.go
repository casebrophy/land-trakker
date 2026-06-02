package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/cbrophy/land_trakker/foundation/config"
	"github.com/cbrophy/land_trakker/foundation/web"
)

func newRouter(cfg *config.Config, q web.ListingsQuerier, sc web.SearchCore) http.Handler {
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
		r.Get("/", web.ListingsHandler(q))
		r.Get("/listings/{id}", web.ListingDetailHandler(q))

		// Saved searches CRUD
		r.Get("/searches", web.SearchesHandler(sc))
		r.Get("/searches/new", web.SearchesNewHandler())
		r.Post("/searches", web.SearchesCreateHandler(sc))
		r.Get("/searches/{id}/edit", web.SearchesEditHandler(sc))
		r.Post("/searches/{id}", web.SearchesUpdateHandler(sc))
		r.Post("/searches/{id}/delete", web.SearchesDeleteHandler(sc))

		// Daily digest
		r.Get("/digest", web.DigestHandler(sc, q))
		r.Post("/digest/mark-seen", web.DigestMarkSeenHandler(sc))
	})

	return r
}
