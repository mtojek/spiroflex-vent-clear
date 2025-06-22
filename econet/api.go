package econet

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	"github.com/mtojek/spiroflex-vent-clear"
)

const payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"

type Installation struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	HasAccess   bool   `json:"hasAccess"`
	IsConnected bool   `json:"isConnected"`
}

func Installations(ctx context.Context, c *spiroflex.Config, creds *cognitotypes.Credentials) ([]Installation, error) {
	url := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod/get-installations", c.Gateway.Name, c.Region)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("new request failed: %w", err)
	}

	awsCreds := aws.Credentials{
		AccessKeyID:     *creds.AccessKeyId,
		SecretAccessKey: *creds.SecretKey,
		SessionToken:    *creds.SessionToken,
		Source:          "CognitoIdentity",
	}

	s := signer.NewSigner()
	if err := s.SignHTTP(ctx, awsCreds, req, payloadHash, "execute-api", c.Region, time.Now()); err != nil {
		return nil, fmt.Errorf("sign HTTP failed: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %s: %s", resp.Status, string(body))
	}

	var result []Installation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("JSON unmarshal failed: %w", err)
	}
	return result, nil
}
