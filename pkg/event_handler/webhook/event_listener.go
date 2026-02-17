package webhook

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/port-labs/port-k8s-exporter/pkg/config"
	"github.com/port-labs/port-k8s-exporter/pkg/logger"
	portwebhook "github.com/port-labs/port-k8s-exporter/pkg/port/webhook"
)

// EventListener implements the webhook event listener
type EventListener struct {
	stateKey      string
	webhookClient *portwebhook.Client
}

// NewEventListener creates a new webhook event listener
func NewEventListener(stateKey string) (*EventListener, error) {
	webhookURL := config.ApplicationConfig.WebhookURL
	if webhookURL == "" {
		return nil, fmt.Errorf("webhook URL is required when eventListener.type is WEBHOOK")
	}

	clusterName := config.ApplicationConfig.ClusterName
	if clusterName == "" {
		clusterName = stateKey // fallback
	}

	batchSize := config.ApplicationConfig.WebhookBatchSize
	if batchSize <= 0 {
		batchSize = 50
	}

	batchTimeout := time.Duration(config.ApplicationConfig.WebhookBatchTimeout) * time.Second
	if batchTimeout <= 0 {
		batchTimeout = 5 * time.Second
	}

	client := portwebhook.NewClient(webhookURL, stateKey, clusterName, batchSize, batchTimeout)

	return &EventListener{
		stateKey:      stateKey,
		webhookClient: client,
	}, nil
}

// Run starts the webhook event listener
func (l *EventListener) Run(resync func()) error {
	logger.Infow("Starting WEBHOOK event listener",
		"webhookURL", config.ApplicationConfig.WebhookURL,
		"stateKey", l.stateKey)

	// Execute initial resync
	resync()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	logger.Info("Received shutdown signal")

	// Flush pending events
	l.webhookClient.FlushSync()
	l.webhookClient.Shutdown()

	return nil
}

// GetClient returns the webhook client for use by controllers
func (l *EventListener) GetClient() *portwebhook.Client {
	return l.webhookClient
}
