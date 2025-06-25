package econet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	signer "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	mqttConnectTimeout   = 10 * time.Second
	mqttSubscribeTimeout = 5 * time.Second
)

type MQTTSession struct {
	clientID       string
	installationID string

	client mqtt.Client

	m                  sync.Mutex
	pending            map[string]chan []byte
	transactionCounter atomic.Int64
}

type OperationRequest struct {
	Name    string          `json:"name"`
	Targets []TargetRequest `json:"targets,omitempty"`

	StatusCode int `json:"statusCode"`
}

type TargetRequest struct {
	Component  string      `json:"component"`
	Parameters interface{} `json:"parameters,omitempty"`
}

type OperationResponse struct {
	Name    string           `json:"name"`
	Targets []TargetResponse `json:"targets,omitempty"`

	StatusCode int `json:"statusCode"`
}

type TargetResponse struct {
	Component  string          `json:"component"`
	Parameters json.RawMessage `json:"parameters,omitempty"`

	StatusCode int `json:"statusCode"`
}

type ComponentOnBus struct {
	ComponentName   string `json:"componentName"`
	ClientID        int    `json:"clientId"`
	DPVersion       string `json:"dpVersion"`
	HardwareVersion string `json:"hardwareVersion"`
	ProgramSeries   string `json:"programSeries"`
	DeviceStatus    int    `json:"deviceStatus"`
	ZDVersion       string `json:"zdVersion"`

	ComponentID string
}

func (s *MQTTSession) GetComponentsOnBus(ctx context.Context) ([]ComponentOnBus, error) {
	resp, err := s.SendInstallationRequest(ctx, []OperationRequest{
		{
			Name: GET_COMPONENTS_ON_BUS,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("SendInstallationRequest failed: %w", err)
	}

	var cobs []ComponentOnBus
	for _, op := range resp {
		for _, t := range op.Targets {
			var cob ComponentOnBus
			err = json.Unmarshal(t.Parameters, &cob)
			if err != nil {
				return nil, fmt.Errorf("can't unmarshal Parameters struct: %w", err)
			}
			cob.ComponentID = t.Component
			cobs = append(cobs, cob)
		}
	}
	return cobs, nil
}

func (s *MQTTSession) VentLevel(ctx context.Context, targetComponentID, level string) error {
	resp, err := s.SendInstallationRequest(ctx, []OperationRequest{
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_SCHEDULE_ID: PARAM_SCHEDULE_MANUAL,
					},
				},
			},
		},
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_POWER_LEVEL_ID: level,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SendInstallationRequest failed: %w", err)
	}
	return verifyParamsModificationStatus(targetComponentID, resp)
}

func (s *MQTTSession) VentPause(ctx context.Context, targetComponentID string) error {
	resp, err := s.SendInstallationRequest(ctx, []OperationRequest{
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_SCHEDULE_ID: PARAM_SCHEDULE_MANUAL,
					},
				},
			},
		},
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_POWER_LEVEL_ID: PARAM_POWER_LEVEL_PAUSE,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SendInstallationRequest failed: %w", err)
	}
	return verifyParamsModificationStatus(targetComponentID, resp)
}

func (s *MQTTSession) VentMode(ctx context.Context, targetComponentID, mode string) error {
	resp, err := s.SendInstallationRequest(ctx, []OperationRequest{
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_SCHEDULE_ID: mode,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SendInstallationRequest failed: %w", err)
	}
	return verifyParamsModificationStatus(targetComponentID, resp)
}

func (s *MQTTSession) VentPower(ctx context.Context, targetComponentID, power string) error {
	resp, err := s.SendInstallationRequest(ctx, []OperationRequest{
		{
			Name: PARAMS_MODIFICATION,
			Targets: []TargetRequest{
				{
					Component: targetComponentID,
					Parameters: map[string]string{
						PARAM_POWER_ID: power,
					},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("SendInstallationRequest failed: %w", err)
	}
	return verifyParamsModificationStatus(targetComponentID, resp)
}

func verifyParamsModificationStatus(targetComponentID string, resp []OperationResponse) error {
	for _, op := range resp {
		for _, t := range op.Targets {
			if t.Component != targetComponentID {
				continue
			}

			if t.StatusCode != 0 {
				return fmt.Errorf("params modification failed, status code: %w", t.StatusCode)
			}
			return nil
		}
	}
	return fmt.Errorf("component not found")
}

func (s *MQTTSession) SendInstallationRequest(ctx context.Context, ops []OperationRequest) ([]OperationResponse, error) {
	type envelopeRequest struct {
		TransactionID string             `json:"transactionId"`
		Operations    []OperationRequest `json:"operations,omitempty"`
	}

	c := s.transactionCounter.Add(1)
	transactionID := fmt.Sprintf("%d", c)

	msg, err := json.Marshal(&envelopeRequest{
		TransactionID: transactionID,
		Operations:    ops,
	})
	if err != nil {
		return nil, fmt.Errorf("can't marshal message envelope: %w", err)
	}

	respCh := make(chan []byte, 1)

	s.m.Lock()
	s.pending[transactionID] = respCh
	s.m.Unlock()

	defer func() {
		s.m.Lock()
		delete(s.pending, transactionID)
		close(respCh)
		s.m.Unlock()
	}()

	topic := fmt.Sprintf("%s/%s/installationRequest", s.installationID, s.clientID)
	log.Printf("Publish message on %s: %s", topic, string(msg))

	token := s.client.Publish(topic, 1, false, msg)
	if !token.WaitTimeout(5 * time.Second) {
		return nil, errors.New("publish timeout")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("publish error: %w", err)
	}

	type envelopeResponse struct {
		TransactionID string              `json:"transactionId"`
		Operations    []OperationResponse `json:"operations,omitempty"`
	}

	select {
	case resp := <-respCh:

		var e envelopeResponse
		err := json.Unmarshal(resp, &e)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshal message, transaction ID: %s, error: %w", transactionID, ctx.Err())
		}
		return e.Operations, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("timeout waiting for response, transaction ID: %s, error: %w", transactionID, ctx.Err())
	}
}

func (s *MQTTSession) Disconnect() {
	s.client.Disconnect(100)
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

		pending: map[string]chan []byte{},
	}
	err = session.startReceiving()
	if err != nil {
		log.Println("MQTT client will disconnect due to error")
		client.Disconnect(0)

		return nil, fmt.Errorf("unable to start receiving: %w", err)
	}
	return session, nil
}

func (s *MQTTSession) startReceiving() error {
	irTopic := fmt.Sprintf("%s/%s/installationResponse", s.installationID, s.clientID)

	log.Printf("Start receiving messages on %s", irTopic)
	err := s.subscribe(irTopic, s.onTransactionalMessage)
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

	var envelope struct {
		TransactionID string `json:"transactionId"`
	}

	err := json.Unmarshal(msg.Payload(), &envelope)
	if err != nil {
		log.Printf("Message will be ignored due to error: %w", err)
		return
	}
	if envelope.TransactionID == "" {
		log.Printf("Message will be ignored due to missing transaction ID")
		return
	}

	s.m.Lock()
	ch, ok := s.pending[envelope.TransactionID]
	s.m.Unlock()

	if ok {
		select {
		case ch <- msg.Payload():
		default:
			log.Printf("full channel for transaction ID: %s", envelope.TransactionID)
		}
	} else {
		log.Printf("unexpected messaged received, transaction ID: %s", envelope.TransactionID)
	}
}
