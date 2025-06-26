package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mtojek/spiroflex-vent-clear"
	"github.com/tbuckley/go-alexa"
)

type WebServer struct {
	c *spiroflex.Config
}

type response struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func NewWebServer(c *spiroflex.Config) *WebServer {
	return &WebServer{
		c: c,
	}
}

func (ws *WebServer) Handler() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Get("/", ws.index)
	r.Route("/api", func(r chi.Router) {
		r.Route("/vent", func(r chi.Router) {
			r.Post("/level/{level:[1-3]?}", ws.apiVentLevel)
			r.Post("/pause", ws.apiVentPause)
			r.Post("/mode/{mode:schedule|manual}", ws.apiVentMode)
			r.Post("/power/{state:on|off}", ws.apiVentPower)
		})
	})

	skill := alexa.New(ws.c.Alexa.AppID)
	r.HandleFunc("/alexa", func(w http.ResponseWriter, r *http.Request) {
		skill.HandlerFuncWithNext(w, r, ws.alexa)
	})
	return r
}
