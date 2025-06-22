package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	cognitotypes "github.com/aws/aws-sdk-go-v2/service/cognitoidentity/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func connect(ctx context.Context, cfg *Config, creds *cognitotypes.Credentials, identityID string) error {
	now := time.Now()

	s := signer.NewSigner()
	awsCreds := aws.Credentials{
		AccessKeyID:     *creds.AccessKeyId,
		SecretAccessKey: *creds.SecretKey,
		Source:          "CognitoIdentity",
	}

	mqttURL := fmt.Sprintf("wss://%s.iot.%s.amazonaws.com/mqtt", cfg.IoT.Name, cfg.Region)
	req, err := http.NewRequest("GET", mqttURL, nil)
	if err != nil {
		return fmt.Errorf("new mqtt request failed: %w", err)
	}

	signedURL, _, err := s.PresignHTTP(ctx, awsCreds, req, payloadHash, "iotdevicegateway", cfg.Region, now)
	if err != nil {
		return fmt.Errorf("presign mqtt failed: %w", err)
	}

	if creds.SessionToken != nil {
		signedURL += "&X-Amz-Security-Token=" + url.QueryEscape(*creds.SessionToken)
	}

	clientID := fmt.Sprintf("%s-%d", identityID, now.UnixMilli())
	opts := mqtt.NewClientOptions().
		AddBroker(signedURL).
		SetClientID(clientID).
		SetProtocolVersion(3)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("mqtt connect failed: %w", token.Error())
	}

	// Subscriptions
	if err := subscribe(client, fmt.Sprintf("%s/installationNotifications", cfg.Installation.ID), logMessage); err != nil {
		return fmt.Errorf("subscribe 1 failed: %w", err)
	}
	if err := subscribe(client, fmt.Sprintf("%s/%s/installationResponse", cfg.Installation.ID, clientID), logMessage); err != nil {
		return fmt.Errorf("subscribe 2 failed: %w", err)
	}

	// Publications
	if err := publish(client, cfg, clientID, "1", `{"name":"GET_COMPONENTS_ON_BUS"}`); err != nil {
		return err
	}
	if err := publish(client, cfg, clientID, "2", `{"name":"GET_VALUES","targets":[{"component":"1007376820","parameters":["u6342","u6338","u81","u6630","u6639","u7074","u6640","u6343","u6344","u86","u7015","u6417","u6418","u6419","u6420","u6421","u6422","u6423","u6904","u7076","u6205","u6209","u6207","u6212","u6208","u6350","u6353","u6938","u6931","u6939","u6322","u6828","u6809","u6202","u6829","u6203","u6273","u6270","u6265","u6288","u6285","u6306","u6300","u6354","u7151","u78","u6699","u6705"]}]}`); err != nil {
		return err
	}
	if err := publish(client, cfg, clientID, "3", `{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u6630":"H0L1"}}]}`); err != nil {
		return err
	}
	if err := publish(client, cfg, clientID, "4", `{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u81":"5"}}]}`); err != nil {
		return err
	}

	for {
		time.Sleep(100 * time.Millisecond)
	}
}

func logMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Message received on %s: %s", msg.Topic(), string(msg.Payload()))
}

func subscribe(client mqtt.Client, topic string, handler mqtt.MessageHandler) error {
	t := client.Subscribe(topic, 1, handler)
	t.Wait()
	return t.Error()
}

func publish(client mqtt.Client, cfg *Config, clientID, txnID, op string) error {
	topic := fmt.Sprintf("%s/%s/installationRequest", cfg.Installation.ID, clientID)
	payload := fmt.Sprintf(`{"transactionId":"%s","operations":[%s]}`, txnID, op)

	t := client.Publish(topic, 1, false, payload)
	t.Wait()
	if err := t.Error(); err != nil {
		return fmt.Errorf("publish txn %s failed: %w", txnID, err)
	}
	log.Println("Message published:", txnID)
	time.Sleep(200 * time.Millisecond)
	return nil
}
