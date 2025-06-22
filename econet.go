package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
)

func getInstallations(ctx context.Context, cfg *Config, creds *cognitotypes.Credentials) error {
	url := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod/get-installations", cfg.Gateway.Name, cfg.Region)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("new request failed: %w", err)
	}

	awsCreds := aws.Credentials{
		AccessKeyID:     *creds.AccessKeyId,
		SecretAccessKey: *creds.SecretKey,
		SessionToken:    *creds.SessionToken,
		Source:          "CognitoIdentity",
	}

	s := signer.NewSigner()
	if err := s.SignHTTP(ctx, awsCreds, req, payloadHash, "execute-api", cfg.Region, time.Now()); err != nil {
		return fmt.Errorf("sign HTTP failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	log.Printf("Status: %s\nResponse: %s\n", resp.Status, string(body))
	return nil
}
