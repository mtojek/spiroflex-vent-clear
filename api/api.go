package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func Create() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Route("/api", func(r chi.Router) {
		r.Route("/vent", func(r chi.Router) {
			r.Get("/level/{level}", ventLevel)
			r.Get("/pause", ventPause)
			r.Get("/mode/{mode}", ventMode)
			r.Get("/power/{state}", ventPower)
		})
	})
	return r
}

func ventLevel(w http.ResponseWriter, r *http.Request) {
	panic("not implemented yet")
}

func ventPause(w http.ResponseWriter, r *http.Request) {
	panic("not implemented yet")
}

func ventMode(w http.ResponseWriter, r *http.Request) {
	panic("not implemented yet")
}

func ventPower(w http.ResponseWriter, r *http.Request) {
	panic("not implemented yet")
}
