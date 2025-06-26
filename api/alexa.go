package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/tbuckley/go-alexa"
)

func (ws *WebServer) alexa(w http.ResponseWriter, r *http.Request) {
	req := alexa.GetEchoRequest(r)

	if req.GetRequestType() == "IntentRequest" || req.GetRequestType() == "LaunchRequest" {
		res := alexa.NewResponse()

		switch req.GetIntentName() {
		case "VentClearLevelIntent":
			level, err := req.GetSlotValue("VentLevel")
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}

			err = ws.ventLevel(r.Context(), level)
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}

			writeAlexaSuccess(w, res, fmt.Sprintf("OK! Level %s set.", level))
		case "VentClearPauseIntent":
			err := ws.ventPause(r.Context())
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}
			writeAlexaSuccess(w, res, "OK! Paused now.")
		case "VentClearModeIntent":
			mode, err := req.GetSlotValue("VentMode")
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}

			err = ws.ventMode(r.Context(), mode)
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}
			writeAlexaSuccess(w, res, fmt.Sprintf("OK! Running in %s mode.", mode))
		case "VentClearPowerIntent":
			state, err := req.GetSlotValue("PowerState")
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}

			err = ws.ventPower(r.Context(), state)
			if err != nil {
				writeAlexaError(w, res, err)
				return
			}
			writeAlexaSuccess(w, res, fmt.Sprintf("OK! Power %s.", state))
		default:
			writeAlexaSuccess(w, res, "Hello there, please say a vent action.")
		}
	}
}

func writeAlexaError(w http.ResponseWriter, res *alexa.EchoResponse, err error) {
	log.Printf("Alexa error: %s", err.Error())

	res.OutputSpeech("Sorry! I didn't understand your request.")
	res.EndSession(true)

	json, _ := res.ToJSON()
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Write(json)
}

func writeAlexaSuccess(w http.ResponseWriter, res *alexa.EchoResponse, message string) {
	log.Printf("Alexa output: %s", message)

	res.OutputSpeech(message)
	res.EndSession(true)
	json, _ := res.ToJSON()
	w.Header().Set("Content-Type", "application/json;charset=UTF-8")
	w.Write(json)
}
