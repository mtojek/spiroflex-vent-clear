package main

import (
	"context"
	"log"
	"net/http"

	"github.com/mtojek/spiroflex-vent-clear/api"
	"github.com/mtojek/spiroflex-vent-clear/app"
	"github.com/mtojek/spiroflex-vent-clear/econet"
)

func main2() {
	c, err := app.LoadConfig()
	if err != nil {
		log.Fatalf("can't load config: %v", err)
	}

	srv := &http.Server{
		Addr:    c.API.Endpoint,
		Handler: api.Create(),
	}

	log.Printf("Server started at %v", c.API.Endpoint)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("srv.ListenAndServe failed: %v", err)
	}
}

func main() {
	ctx := context.Background()

	c, err := app.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	identityID, creds, err := econet.Auth(ctx, c)
	if err != nil {
		log.Fatalf("failed to fetch credentials: %v", err)
	}

	if err := econet.Installations(ctx, c, creds); err != nil {
		log.Fatalf("API call failed: %v", err)
	}

	if err := econet.MQTT(ctx, c, creds, identityID); err != nil {
		log.Fatalf("MQTT error: %v", err)
	}
}
