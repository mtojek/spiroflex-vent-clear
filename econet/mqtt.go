package econet

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/mtojek/spiroflex-vent-clear"
)

const (
	mqttConnectTimeout   = 10 * time.Second
	mqttSubscribeTimeout = 5 * time.Second
)

type MQTTSession struct {
	clientID       string
	installationID string

	client mqtt.Client

	m       sync.Mutex
	pending map[string]chan []byte
}

func (s *MQTTSession) startReceiving() error {
	inTopic := fmt.Sprintf("%s/installationNotifications", s.installationID)
	irTopic := fmt.Sprintf("%s/%s/installationResponse", s.installationID, s.clientID)

	log.Printf("Start receiving messages on %s", inTopic)
	err := s.subscribe(inTopic, s.onTransactionalMessage)
	if err != nil {
		return fmt.Errorf("unable to subscribe to installationNotifications: %w", err)
	}

	log.Printf("Start receiving messages on %s", irTopic)
	err = s.subscribe(irTopic, s.onTransactionalMessage)
	if err != nil {
		return fmt.Errorf("unable to subscribe to installationResponse: %w", err)
	}
	return nil
}

func (s *MQTTSession) subscribe(topic string, handler mqtt.MessageHandler) error {
	t := s.client.Subscribe(topic, 1, handler)
	t.WaitTimeout(mqttSubscribeTimeout)
	return t.Error()
}

func (s *MQTTSession) onTransactionalMessage(client mqtt.Client, msg mqtt.Message) {
	log.Printf("Message received on %s: %s", msg.Topic(), string(msg.Payload()))

	// TODO
}

func (c *Client) MQTT(ctx context.Context, installationID string) (*MQTTSession, error) {
	now := time.Now()

	s := signer.NewSigner()
	awsCreds := aws.Credentials{
		AccessKeyID:     *c.creds.AccessKeyId,
		SecretAccessKey: *c.creds.SecretKey,
		Source:          "CognitoIdentity",
	}

	mqttURL := fmt.Sprintf("wss://%s.iot.%s.amazonaws.com/mqtt", c.cfg.IoT.Name, c.cfg.Region)
	req, err := http.NewRequest("GET", mqttURL, nil)
	if err != nil {
		return nil, fmt.Errorf("can't build HTTP request: %w", err)
	}

	signedURL, _, err := s.PresignHTTP(ctx, awsCreds, req, payloadHash, "iotdevicegateway", c.cfg.Region, now)
	if err != nil {
		return nil, fmt.Errorf("unable to presign MQTT URL: %w", err)
	}

	if c.creds.SessionToken != nil {
		signedURL += "&X-Amz-Security-Token=" + url.QueryEscape(*c.creds.SessionToken)
	}

	clientID := fmt.Sprintf("%s-%d", c.identityID, now.UnixMilli())
	opts := mqtt.NewClientOptions().
		AddBroker(signedURL).
		SetClientID(clientID).
		SetProtocolVersion(3)

	client := mqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(mqttConnectTimeout) {
		return nil, errors.New("connection to MQTT broker timed out")
	}
	if token.Error() != nil {
		return nil, fmt.Errorf("unable to connect to MQTT broker: %w", token.Error())
	}
	log.Printf("MQTT client connected, installationID: %s, clientID: %s", installationID, clientID)

	session := &MQTTSession{
		clientID:       clientID,
		installationID: installationID,

		client: client,
	}
	err = session.startReceiving()
	if err != nil {
		log.Println("MQTT client will disconnect due to error")
		client.Disconnect(0)

		return nil, fmt.Errorf("unable to start receiving: %w", err)
	}
	return session, nil

	// Publications
	/*if err := publish(client, c.cfg, clientID, "1", `{"name":"GET_COMPONENTS_ON_BUS"}`, installationID); err != nil {
		return err
	}
	if err := publish(client, c.cfg, clientID, "2", `{"name":"GET_VALUES","targets":[{"component":"1007376820","parameters":["u6342","u6338","u81","u6630","u6639","u7074","u6640","u6343","u6344","u86","u7015","u6417","u6418","u6419","u6420","u6421","u6422","u6423","u6904","u7076","u6205","u6209","u6207","u6212","u6208","u6350","u6353","u6938","u6931","u6939","u6322","u6828","u6809","u6202","u6829","u6203","u6273","u6270","u6265","u6288","u6285","u6306","u6300","u6354","u7151","u78","u6699","u6705"]}]}`, installationID); err != nil {
		return err
	}
	if err := publish(client, c.cfg, clientID, "3", `{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u6630":"H0L1"}}]}`, installationID); err != nil {
		return err
	}
	if err := publish(client, c.cfg, clientID, "4", `{"name":"PARAMS_MODIFICATION","targets":[{"component":"1007376820","parameters":{"u81":"5"}}]}`, installationID); err != nil {
		return err
	}

	for {
		time.Sleep(100 * time.Millisecond)
	}*/
}

func publish(client mqtt.Client, c *spiroflex.Config, clientID, txnID, op string, installationID string) error {
	topic := fmt.Sprintf("%s/%s/installationRequest", installationID, clientID)
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
