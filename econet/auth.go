package econet

import (
	"context"
	"fmt"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/mtojek/spiroflex-vent-clear"
)

func auth(ctx context.Context, c *spiroflex.Config) (string, *cognitotypes.Credentials, error) {
	awsCfg, err := spiroflex.LoadAWSConfig(ctx, c)
	if err != nil {
		return "", nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	authResult, err := cognitoAuthenticate(ctx, c, *awsCfg)
	if err != nil {
		return "", nil, fmt.Errorf("Cognito authentication failed: %w", err)
	}
	idToken := *authResult.IdToken

	ci := cognitoidentity.NewFromConfig(*awsCfg)
	provider := fmt.Sprintf("cognito-idp.%s.amazonaws.com/%s", c.Region, c.Cognito.UserPoolID)

	idResp, err := ci.GetId(ctx, &cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(c.Cognito.IdentityPoolID),
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
	return identityID, credsResp.Credentials, nil
}

func cognitoAuthenticate(ctx context.Context, c *spiroflex.Config, awsCfg aws.Config) (*types.AuthenticationResultType, error) {
	srp, err := initSRP(c)
	if err != nil {
		return nil, fmt.Errorf("initiate SRP failed: %w", err)
	}

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
	return tokens, nil
}

func initSRP(c *spiroflex.Config) (*cognitosrp.CognitoSRP, error) {
	srp, err := cognitosrp.NewCognitoSRP(
		c.Cognito.Username,
		c.Cognito.Password,
		c.Cognito.UserPoolID,
		c.Cognito.ClientID,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("cognitosrp.NewCognitoSRP failed: %v", srp)
	}
	return srp, nil
}
