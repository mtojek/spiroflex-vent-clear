package main

import (
	"context"
	"log"
	"net/http"
	"slices"
	"time"

	"github.com/mtojek/spiroflex-vent-clear"
	"github.com/mtojek/spiroflex-vent-clear/api"
	"github.com/mtojek/spiroflex-vent-clear/econet"
)

func main2() {
	c, err := spiroflex.LoadConfig()
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

	c, err := spiroflex.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	client, err := econet.New(ctx, c)
	if err != nil {
		log.Fatalf("unable to create client: %v", err)
	}

	installations, err := client.Installations(ctx)
	if err != nil {
		log.Fatalf("API call failed: %v", err)
	}

	i := slices.IndexFunc(installations, func(ins econet.Installation) bool {
		return ins.Name == c.Installation.Name
	})
	if i < 0 {
		log.Fatal("Installation not found or invalid name")
	}

	_, err = client.MQTT(ctx, installations[i].ID)
	if err != nil {
		log.Fatalf("MQTT error: %v", err)
	}

	for {
		time.Sleep(100 * time.Millisecond)
	}
}
