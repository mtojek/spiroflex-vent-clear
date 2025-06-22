package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

func initSRP(cfg *Config) (*cognitosrp.CognitoSRP, error) {
	srp, err := cognitosrp.NewCognitoSRP(
		cfg.Cognito.Username,
		cfg.Cognito.Password,
		cfg.Cognito.UserPoolID,
		cfg.Cognito.ClientID,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("cognitosrp.NewCognitoSRP failed: %v", srp)
	}
	return srp, nil
}

func authenticate(ctx context.Context, awsCfg aws.Config, srp *cognitosrp.CognitoSRP) (*types.AuthenticationResultType, error) {
	cipClient := cip.NewFromConfig(awsCfg)

	initResp, err := cipClient.InitiateAuth(ctx, &cip.InitiateAuthInput{
		AuthFlow:       types.AuthFlowTypeUserSrpAuth,
		ClientId:       aws.String(srp.GetClientId()),
		AuthParameters: srp.GetAuthParams(),
	})
	if err != nil {
		return nil, fmt.Errorf("initiate auth failed: %w", err)
	}

	if initResp.ChallengeName != types.ChallengeNameTypePasswordVerifier {
		return nil, fmt.Errorf("unexpected challenge name: %s", initResp.ChallengeName)
	}

	challengeResponses, err := srp.PasswordVerifierChallenge(initResp.ChallengeParameters, time.Now())
	if err != nil {
		return nil, fmt.Errorf("password verifier challenge failed: %w", err)
	}

	resp, err := cipClient.RespondToAuthChallenge(ctx, &cip.RespondToAuthChallengeInput{
		ChallengeName:      types.ChallengeNameTypePasswordVerifier,
		ChallengeResponses: challengeResponses,
		ClientId:           aws.String(srp.GetClientId()),
	})
	if err != nil {
		return nil, fmt.Errorf("respond to auth challenge failed: %w", err)
	}

	tokens := resp.AuthenticationResult
	log.Printf("Access Token: %s\nID Token: %s\nRefresh Token: %s\n", *tokens.AccessToken, *tokens.IdToken, *tokens.RefreshToken)
	return tokens, nil
}

func fetchCredentials(ctx context.Context, awsCfg aws.Config, cfg *Config, idToken string) (string, *cognitotypes.Credentials, error) {
	ci := cognitoidentity.NewFromConfig(awsCfg)
	provider := fmt.Sprintf("cognito-idp.%s.amazonaws.com/%s", cfg.Region, cfg.Cognito.UserPoolID)

	idResp, err := ci.GetId(ctx, &cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(cfg.Cognito.IdentityPoolID),
		Logins:         map[string]string{provider: idToken},
	})
	if err != nil {
		return "", nil, fmt.Errorf("unable to get Cognito ID: %w", err)
	}

	identityID := *idResp.IdentityId

	credsResp, err := ci.GetCredentialsForIdentity(ctx, &cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: aws.String(identityID),
		Logins:     map[string]string{provider: idToken},
	})
	if err != nil {
		return "", nil, fmt.Errorf("unable to get Cognito credentials: %w", err)
	}

	log.Println("IdentityId:", identityID)
	return identityID, credsResp.Credentials, nil
}
