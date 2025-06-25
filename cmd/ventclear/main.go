package main

import (
	"log"
	"net/http"

	"github.com/mtojek/spiroflex-vent-clear"
	"github.com/mtojek/spiroflex-vent-clear/api"
)

func main() {
	c, err := spiroflex.LoadConfig()
	if err != nil {
		log.Fatalf("can't load config: %v", err)
	}

	webServer := api.NewWebServer(c)
	srv := &http.Server{
		Addr:    c.API.Endpoint,
		Handler: webServer.Handler(),
	}

	log.Printf("Server started at %v", c.API.Endpoint)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("srv.ListenAndServe failed: %v", err)
	}
}
