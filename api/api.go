package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/mtojek/spiroflex-vent-clear"
	"github.com/mtojek/spiroflex-vent-clear/econet"
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
			r.Post("/level/{level:[1-3]?}", ws.ventLevel)
			r.Post("/pause", ws.ventPause)
			r.Post("/mode/{mode:schedule|manual}", ws.ventMode)
			r.Post("/power/{state:on|off}", ws.ventPower)
		})
	})
	return r
}

func (ws *WebServer) index(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello world"))
}

func (ws *WebServer) ventLevel(w http.ResponseWriter, r *http.Request) {
	levelStr := chi.URLParam(r, "level")
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		writeError(w, err)
		return
	}
	level += 2 // levels are enumerated from 3 to 5

	session, targetComponentID, err := ws.prepareEconet(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	err = session.VentLevel(r.Context(), targetComponentID, fmt.Sprintf("%d", level))
	if err != nil {
		writeError(w, fmt.Errorf("unable to modify parameters: %w", err))
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) ventPause(w http.ResponseWriter, r *http.Request) {
	session, targetComponentID, err := ws.prepareEconet(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	err = session.VentPause(r.Context(), targetComponentID)
	if err != nil {
		writeError(w, fmt.Errorf("unable to modify parameters: %w", err))
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) ventMode(w http.ResponseWriter, r *http.Request) {
	mode := chi.URLParam(r, "mode")
	if mode == "schedule" {
		mode = econet.PARAM_SCHEDULE_SCHEDULE
	} else {
		mode = econet.PARAM_SCHEDULE_MANUAL
	}

	session, targetComponentID, err := ws.prepareEconet(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	err = session.VentMode(r.Context(), targetComponentID, mode)
	if err != nil {
		writeError(w, fmt.Errorf("unable to modify parameters: %w", err))
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) ventPower(w http.ResponseWriter, r *http.Request) {
	state := chi.URLParam(r, "state")
	if state == "on" {
		state = econet.PARAM_POWER_ON
	} else {
		state = econet.PARAM_POWER_OFF
	}

	session, targetComponentID, err := ws.prepareEconet(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}

	err = session.VentPower(r.Context(), targetComponentID, state)
	if err != nil {
		writeError(w, fmt.Errorf("unable to modify parameters: %w", err))
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) prepareEconet(ctx context.Context) (*econet.MQTTSession, string, error) {
	client, err := econet.New(ctx, ws.c)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create client: %w", err)
	}

	installations, err := client.Installations(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to fetch installations: %w", err)
	}

	i := slices.IndexFunc(installations, func(ins econet.Installation) bool {
		return ins.Name == ws.c.Installation.Name
	})
	if i < 0 {
		return nil, "", fmt.Errorf("installation not found or invalid name: %w", err)
	}
	installationID := installations[i].ID

	session, err := client.MQTT(ctx, installationID)
	if err != nil {
		return nil, "", fmt.Errorf("MQTT error: %w", err)
	}

	gcob, err := session.GetComponentsOnBus(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to fetch components on bus: %w", err)
	}

	var targetComponentID string
	for _, c := range gcob {
		if c.ComponentName == "ecoVENT MINI OEM" {
			targetComponentID = c.ComponentID
		}
	}
	return session, targetComponentID, nil
}

func writeError(w http.ResponseWriter, err error) {
	resp := response{Error: err.Error()}
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(resp)
}

func writeSuccess(w http.ResponseWriter) {
	resp := response{Ok: true}
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(resp)
}
