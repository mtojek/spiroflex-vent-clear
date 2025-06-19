package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	cognitosrp "github.com/alexrudd/cognito-srp/v4"
	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/config"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentity"
	cip "github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/viper"
)

const (
	payloadHash = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

type Config struct {
	Region  string
	Cognito CognitoConfig
	Gateway APIGateway
	IoT     AWSIoT

	Installation Installation
}

type CognitoConfig struct {
	Username       string
	Password       string
	UserPoolID     string `mapstructure:"user_pool_id"`
	ClientID       string `mapstructure:"client_id"`
	IdentityPoolID string `mapstructure:"identity_pool_id"`
}

type APIGateway struct {
	Name string
}

type AWSIoT struct {
	Name string
}

type Installation struct {
	ID string
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
		log.Printf("Access Token: %s\n", *ciResp.AuthenticationResult.AccessToken)
		log.Printf("ID Token: %s\n", *ciResp.AuthenticationResult.IdToken)
		log.Printf("Refresh Token: %s\n", *ciResp.AuthenticationResult.RefreshToken)
	} else {
		log.Fatal("other challenges await")
	}

	// Now, AWS API Gateway...
	// Step 1: Get Identity ID
	ci := cognitoidentity.NewFromConfig(cfg)
	getIdResp, err := ci.GetId(ctx, &cognitoidentity.GetIdInput{
		IdentityPoolId: aws.String(c.Cognito.IdentityPoolID),
		Logins: map[string]string{
			fmt.Sprintf("cognito-idp.%s.amazonaws.com/%s", c.Region, c.Cognito.UserPoolID): *ciResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Fatal("ci.GetId failed: ", err)
	}
	identityID := *getIdResp.IdentityId
	log.Println("IdentityId: " + identityID)

	// Step 2: Get Credentials for Identity
	credsResp, err := ci.GetCredentialsForIdentity(ctx, &cognitoidentity.GetCredentialsForIdentityInput{
		IdentityId: &identityID,
		Logins: map[string]string{
			fmt.Sprintf("cognito-idp.%s.amazonaws.com/%s", c.Region, c.Cognito.UserPoolID): *ciResp.AuthenticationResult.IdToken,
		},
	})
	if err != nil {
		log.Fatal("ci.GetCredentialsForIdentity failed: " + err.Error())
	}
	creds := credsResp.Credentials

	// Step 3: Create a signed HTTP request to your API
	apiURL := fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/Prod/get-installations", c.Gateway.Name, c.Region)
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

	now := time.Now()
	err = signer.SignHTTP(context.TODO(), awsCreds, req, payloadHash, "execute-api", c.Region, now)
	if err != nil {
		log.Fatal("signer.SignHTTP failed: " + err.Error())
	}

	// Step 4: Send the request
	client := http.Client{}
	httpResp, err := client.Do(req)
	if err != nil {
		log.Fatal("client.Do failed: " + err.Error())
	}
	defer httpResp.Body.Close()

	body, _ := io.ReadAll(httpResp.Body)

	log.Println("Status:", httpResp.Status)
	log.Println("Response:", string(body))

	// Step 5: Connect to MQTT
	sessionToken := creds.SessionToken
	awsCreds.SessionToken = "" // ????

	mqttURL := fmt.Sprintf("wss://%s.iot.%s.amazonaws.com/mqtt", c.IoT.Name, c.Region)
	req, err = http.NewRequest("GET", mqttURL, nil)
	if err != nil {
		log.Fatal("http.NewRequest mqtt failed: ", err)
	}

	signedURL, _, err := signer.PresignHTTP(
		ctx,
		awsCreds,
		req,
		payloadHash,
		"iotdevicegateway",
		c.Region,
		now,
	)
	if err != nil {
		log.Fatal("signer.PresignHTTP mqtt failed: ", err)
	}

	if sessionToken != nil {
		signedURL += "&X-Amz-Security-Token=" + url.QueryEscape(*sessionToken)
	}

	log.Println(signedURL)

	opts := mqtt.NewClientOptions()
	opts.ProtocolVersion = 3
	opts.AddBroker(signedURL)

	clientID := fmt.Sprintf("%s-%d", identityID, now.Unix()*1000)
	log.Println(clientID)

	opts.SetClientID(clientID)

	mqttClient := mqtt.NewClient(opts)
	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("MQTT connect error: ", token.Error())
	}

	// Step 6. Subscribe to installation notifications and responses

	if token := mqttClient.Subscribe(fmt.Sprintf("%s/installationNotifications", c.Installation.ID), 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("Message received on %s: %s", msg.Topic(), string(msg.Payload()))
	}); token.Wait() && token.Error() != nil {
		log.Fatalf("Subscribe failed: %v", token.Error())
	}

	if token2 := mqttClient.Subscribe(fmt.Sprintf("%s/%s/installationResponse", c.Installation.ID, clientID), 1, func(client mqtt.Client, msg mqtt.Message) {
		log.Printf("Message 2 received on %s: %s", msg.Topic(), string(msg.Payload()))
	}); token2.Wait() && token2.Error() != nil {
		log.Fatalf("Subscribe 2 failed: %v", token2.Error())
	}

	// Step 7. Publish GET_VALUES
	token := mqttClient.Publish(
		fmt.Sprintf("%s/%s/installationRequest", c.Installation.ID, clientID),
		1,
		false,
		`{"transactionId":"2","operations":[{"name":"GET_VALUES","targets":[{"component":"1007376820","parameters":["u6342","u6338","u81","u6630","u6639","u7074","u6640","u6343","u6344","u86","u7015","u6417","u6418","u6419","u6420","u6421","u6422","u6423","u6904","u7076","u6205","u6209","u6207","u6212","u6208","u6350","u6353","u6938","u6931","u6939","u6322","u6828","u6809","u6202","u6829","u6203","u6273","u6270","u6265","u6288","u6285","u6306","u6300","u6354","u7151","u78","u6699","u6705"]}]}]}`, // message payload
	)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("Publish error: %v", err)
	} else {
		log.Println("Message published successfully")
	}

	time.Sleep(500 * time.Millisecond)

	// Step 8.
	token = mqttClient.Publish(
		fmt.Sprintf("%s/%s/installationRequest", c.Installation.ID, clientID),
		1,
		false,
		`{"transactionId":"3","operations":[{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u6630":"H0L1"}}]}]}`, // tryb reczny
	)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("Publish error: %v", err)
	} else {
		log.Println("Message published successfully")
	}

	time.Sleep(100 * time.Millisecond)

	token = mqttClient.Publish(
		fmt.Sprintf("%s/%s/installationRequest", c.Installation.ID, clientID),
		1,
		false,
		`{"transactionId":"4","operations":[{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u81":"5"}}]}]}`, // moc 3
	)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatalf("Publish error: %v", err)
	} else {
		log.Println("Message published successfully")
	}

	// Don't need to implement ping for now
	for {
		time.Sleep(100 * time.Millisecond)
	}
}
