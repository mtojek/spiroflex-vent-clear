package api

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func (ws *WebServer) index(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Hello world"))
}

func (ws *WebServer) apiVentLevel(w http.ResponseWriter, r *http.Request) {
	level := chi.URLParam(r, "level")
	err := ws.ventLevel(r.Context(), level)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) apiVentPause(w http.ResponseWriter, r *http.Request) {
	err := ws.ventPause(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) apiVentMode(w http.ResponseWriter, r *http.Request) {
	mode := chi.URLParam(r, "mode")
	err := ws.ventMode(r.Context(), mode)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w)
}

func (ws *WebServer) apiVentPower(w http.ResponseWriter, r *http.Request) {
	state := chi.URLParam(r, "state")
	err := ws.ventPower(r.Context(), state)
	if err != nil {
		writeError(w, err)
		return
	}
	writeSuccess(w)
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
