package main

import (
	"context"
	"log"
	"net/http"
)

const (
	payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func main2() {
	c, err := loadConfig()
	if err != nil {
		log.Fatalf("can't load config: %v", err)
	}

	srv := &http.Server{
		Addr:    c.API.Endpoint,
		Handler: createAPI(),
	}

	log.Printf("Server started at %v", c.API.Endpoint)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("srv.ListenAndServe failed: %v", err)
	}
}

func main() {
	c, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	awsCfg, err := loadAWSConfig(ctx, c)
	if err != nil {
		log.Fatal(err)
	}

	srp, err := initSRP(c)
	authResult, err := authenticate(ctx, *awsCfg, srp)
	if err != nil {
		log.Fatalf("authentication failed: %v", err)
	}

	identityID, creds, err := fetchCredentials(ctx, *awsCfg, c, *authResult.IdToken)
	if err != nil {
		log.Fatalf("failed to fetch credentials: %v", err)
	}

	if err := getInstallations(ctx, c, creds); err != nil {
		log.Fatalf("API call failed: %v", err)
	}

	if err := connect(ctx, c, creds, identityID); err != nil {
		log.Fatalf("MQTT error: %v", err)
	}
}
