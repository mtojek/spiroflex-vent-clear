package main

import (
	"context"
	"fmt"
	"log"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	"github.com/spf13/viper"
)

type Config struct {
	Cognito CognitoConfig
}

type CognitoConfig struct {
	Username   string
	Password   string
	UserPoolID string `mapstructure:"user_pool_id"`
	ClientID   string `mapstructure:"client_id"`
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

	// respond to password verifier challenge
	if resp.ChallengeName == types.ChallengeNameTypePasswordVerifier {
		challengeResponses, err := csrp.PasswordVerifierChallenge(resp.ChallengeParameters, time.Now())
		if err != nil {
			log.Fatal("csrp.PasswordVerifierChallenge failed: ", err)
		}

		resp, err := svc.RespondToAuthChallenge(context.Background(), &cip.RespondToAuthChallengeInput{
			ChallengeName:      types.ChallengeNameTypePasswordVerifier,
			ChallengeResponses: challengeResponses,
			ClientId:           aws.String(csrp.GetClientId()),
		})
		if err != nil {
			log.Fatal("svc.RespondToAuthChallenge failed: ", err)
		}

		// print the tokens
		fmt.Printf("Access Token: %s\n", *resp.AuthenticationResult.AccessToken)
		fmt.Printf("ID Token: %s\n", *resp.AuthenticationResult.IdToken)
		fmt.Printf("Refresh Token: %s\n", *resp.AuthenticationResult.RefreshToken)
	} else {
		log.Println("other challenges await")
	}

	// Now, AWS API Gateway...

}
