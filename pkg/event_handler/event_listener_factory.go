package event_handler

import (
	"fmt"

	"github.com/port-labs/port-k8s-exporter/pkg/event_handler/consumer"
	"github.com/port-labs/port-k8s-exporter/pkg/event_handler/polling"
	"github.com/port-labs/port-k8s-exporter/pkg/event_handler/webhook"
	"github.com/port-labs/port-k8s-exporter/pkg/logger"
	"github.com/port-labs/port-k8s-exporter/pkg/port/cli"
)

func CreateEventListener(stateKey string, eventListenerType string, portClient *cli.PortClient) (IListener, error) {
	logger.Infof("Received event listener type: %s", eventListenerType)
	switch eventListenerType {
	case "KAFKA":
		if portClient == nil {
			return nil, fmt.Errorf("portClient is required for KAFKA event listener")
		}
		return consumer.NewEventListener(stateKey, portClient)
	case "POLLING":
		if portClient == nil {
			return nil, fmt.Errorf("portClient is required for POLLING event listener")
		}
		return polling.NewEventListener(stateKey, portClient), nil
	case "WEBHOOK":
		return webhook.NewEventListener(stateKey)
	default:
		return nil, fmt.Errorf("unknown event listener type: %s", eventListenerType)
	}
}
