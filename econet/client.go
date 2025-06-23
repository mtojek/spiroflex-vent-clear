package econet

import (
	"context"
	"fmt"

	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	"github.com/mtojek/spiroflex-vent-clear"
)

type Client struct {
	cfg *spiroflex.Config

	identityID string
	creds      *cognitotypes.Credentials
}

func New(ctx context.Context, cfg *spiroflex.Config) (*Client, error) {
	identityID, creds, err := auth(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("unable to authenticate the client: %v", err)
	}

	return &Client{
		cfg:        cfg,
		identityID: identityID,
		creds:      creds,
	}, nil
}
