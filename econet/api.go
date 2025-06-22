package econet

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
	"github.com/mtojek/spiroflex-vent-clear/app"
)

const (
	payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func Installations(ctx context.Context, c *app.Config, creds *cognitotypes.Credentials) error {
	url := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod/get-installations", c.Gateway.Name, c.Region)
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
	if err := s.SignHTTP(ctx, awsCreds, req, payloadHash, "execute-api", c.Region, time.Now()); err != nil {
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
