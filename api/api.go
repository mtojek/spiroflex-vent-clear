package api

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/mtojek/spiroflex-vent-clear/econet"
)

func (ws *WebServer) ventLevel(ctx context.Context, levelStr string) error {
	level, err := strconv.Atoi(levelStr)
	if err != nil {
		return fmt.Errorf("can't convert level to int: %w", err)
	}

	if level < 1 || level > 3 {
		return fmt.Errorf("level must be between 1 and 3")
	}

	level += 2 // levels are enumerated from 3 to 5

	session, targetComponentID, err := ws.prepareEconet(ctx)
	if err != nil {
		return err
	}

	err = session.VentLevel(ctx, targetComponentID, fmt.Sprintf("%d", level))
	if err != nil {
		return fmt.Errorf("unable to modify parameters: %w", err)
	}
	return nil
}

func (ws *WebServer) ventPause(ctx context.Context) error {
	session, targetComponentID, err := ws.prepareEconet(ctx)
	if err != nil {
		return err
	}

	err = session.VentPause(ctx, targetComponentID)
	if err != nil {
		return fmt.Errorf("unable to modify parameters: %w", err)
	}
	return nil
}

func (ws *WebServer) ventMode(ctx context.Context, mode string) error {
	if mode == "schedule" {
		mode = econet.PARAM_MODE_SCHEDULE
	} else {
		mode = econet.PARAM_MODE_MANUAL
	}

	session, targetComponentID, err := ws.prepareEconet(ctx)
	if err != nil {
		return err
	}

	err = session.VentMode(ctx, targetComponentID, mode)
	if err != nil {
		return fmt.Errorf("unable to modify parameters: %w", err)
	}
	return nil
}

func (ws *WebServer) ventPower(ctx context.Context, state string) error {
	if state == "on" {
		state = econet.PARAM_POWER_ON
	} else {
		state = econet.PARAM_POWER_OFF
	}

	session, targetComponentID, err := ws.prepareEconet(ctx)
	if err != nil {
		return err
	}

	err = session.VentPower(ctx, targetComponentID, state)
	if err != nil {
		return fmt.Errorf("unable to modify parameters: %w", err)
	}
	return nil
}

func (ws *WebServer) prepareEconet(ctx context.Context) (*econet.MQTTSession, string, error) {
	client, err := econet.New(ctx, ws.c)
	if err != nil {
		return nil, "", fmt.Errorf("unable to create client: %w", err)
	}

	installations, err := client.Installations(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to fetch installations: %w", err)
	}

	i := slices.IndexFunc(installations, func(ins econet.Installation) bool {
		return ins.Name == ws.c.Installation.Name
	})
	if i < 0 {
		return nil, "", fmt.Errorf("installation not found or invalid name: %w", err)
	}
	installationID := installations[i].ID

	session, err := client.MQTT(ctx, installationID)
	if err != nil {
		return nil, "", fmt.Errorf("MQTT error: %w", err)
	}

	gcob, err := session.GetComponentsOnBus(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("unable to fetch components on bus: %w", err)
	}

	var targetComponentID string
	for _, c := range gcob {
		if c.ComponentName == "ecoVENT MINI OEM" {
			targetComponentID = c.ComponentID
		}
	}
	return session, targetComponentID, nil
}
