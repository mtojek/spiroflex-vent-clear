package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/spf13/viper"
)

type Config struct {
	Cognito CognitoConfig
}

type CognitoConfig struct {
	Username       string
	Password       string
	UserPoolID     string `mapstructure:"user_pool_id"`
	ClientID       string `mapstructure:"client_id"`
	IdentityPoolID string `mapstructure:"identity_pool_id"`
}

func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatal("viper.ReadInConfig failed: ", err)
	}

	var c Config
	if err := viper.Unmarshal(&c); err != nil {
		log.Fatal("viper.Unmarshal failed: ", err)
	}

	csrp, err := cognitosrp.NewCognitoSRP(
		c.Cognito.Username,
		c.Cognito.Password,
		c.Cognito.UserPoolID,
		c.Cognito.ClientID,
		nil)
	if err != nil {
		log.Fatal("cognitosrp.NewCognitoSRP failed: ", err)
	}

	ctx := context.Background()
	cfg, err := awsconfig.LoadDefaultConfig(
		ctx,
		config.WithRegion("eu-central-1"),
		config.WithCredentialsProvider(aws.AnonymousCredentials{}),
	)
	if err != nil {
		log.Fatal("awsconfig.LoadDefaultConfig failed: ", err)
	}
	svc := cip.NewFromConfig(cfg)

	// initiate auth
	resp, err := svc.InitiateAuth(ctx, &cip.InitiateAuthInput{
		AuthFlow:       types.AuthFlowTypeUserSrpAuth,
		ClientId:       aws.String(csrp.GetClientId()),
		AuthParameters: csrp.GetAuthParams(),
	})
	if err != nil {
		log.Fatal("svc.InitiateAuth failed: ", err)
	}

	var ciResp *cip.RespondToAuthChallengeOutput
	// respond to password verifier challenge
	if resp.ChallengeName == types.ChallengeNameTypePasswordVerifier {
		challengeResponses, err := csrp.PasswordVerifierChallenge(resp.ChallengeParameters, time.Now())
		if err != nil {
			log.Fatal("csrp.PasswordVerifierChallenge failed: ", err)
		}

		ciResp, err = svc.RespondToAuthChallenge(context.Background(), &cip.RespondToAuthChallengeInput{
			ChallengeName:      types.ChallengeNameTypePasswordVerifier,
			ChallengeResponses: challengeResponses,
			ClientId:           aws.String(csrp.GetClientId()),
		})
		if err != nil {
			log.Fatal("svc.RespondToAuthChallenge failed: ", err)
		}

		// print the tokens
		fmt.Printf("Access Token: %s\n", *ciResp.AuthenticationResult.AccessToken)
		fmt.Printf("ID Token: %s\n", *ciResp.AuthenticationResult.IdToken)
		fmt.Printf("Refresh Token: %s\n", *ciResp.AuthenticationResult.RefreshToken)
	} else {
		log.Fatal("other challenges await")
	}

	// Now, AWS API Gateway...
	// Step 1: Get Identity ID
	ci := cognitoidentity.NewFromConfig(cfg)
	getIdResp, err := ci.GetId(ctx, &cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(c.Cognito.IdentityPoolID),
		Logins: map[string]string{
			"cognito-idp.eu-central-1.amazonaws.com/" + c.Cognito.UserPoolID: *ciResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Fatal("ci.GetId failed: ", err)
	}

	// Step 2: Get Credentials for Identity
	credsResp, err := ci.GetCredentialsForIdentity(ctx, &cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: getIdResp.IdentityId,
		Logins: map[string]string{
			"cognito-idp.eu-central-1.amazonaws.com/" + c.Cognito.UserPoolID: *ciResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Fatal("ci.GetCredentialsForIdentity failed: " + err.Error())
	}
	creds := credsResp.Credentials

	// Step 3: Create a signed HTTP request to your API
	apiURL := "https://jfe3pa3bw8.execute-api.eu-central-1.amazonaws.com/Prod/get-installations"
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Fatal("http.NewRequest failed: " + err.Error())
	}

	signer := signer.NewSigner()

	awsCreds := aws.Credentials{
		AccessKeyID:     *creds.AccessKeyId,
		SecretAccessKey: *creds.SecretKey,
		SessionToken:    *creds.SessionToken,
		Source:          "CognitoIdentity",
	}

	payloadHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	err = signer.SignHTTP(context.TODO(), awsCreds, req, payloadHash, "execute-api", "eu-central-1", time.Now())
	if err != nil {
		panic("failed to sign request: " + err.Error())
	}

	// Step 4: Send the request
	client := http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		panic("request failed: " + err.Error())
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	fmt.Println("Status:", httpResp.Status)
	fmt.Println("Response:", string(body))
}
