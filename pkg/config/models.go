package config

import "github.com/port-labs/port-k8s-exporter/pkg/port"

type KafkaConfiguration struct {
	Brokers                 string
	SecurityProtocol        string
	GroupID                 string
	AuthenticationMechanism string
	Username                string
	Password                string
	KafkaSecurityEnabled    bool
}

type ApplicationConfiguration struct {
	ConfigFilePath                  string
	StateKey                        string
	ResyncInterval                  uint
	PortBaseURL                     string
	PortClientId                    string
	PortClientSecret                string
	EventListenerType               string
	CreateDefaultResources          bool
	CreatePortResourcesOrigin       port.CreatePortResourcesOrigin
	OverwriteConfigurationOnRestart bool
	// These Configurations are used only for setting up the Integration on installation or when using OverwriteConfigurationOnRestart flag.
	Resources                    []port.Resource
	DeleteDependents             bool `json:"deleteDependents,omitempty"`
	CreateMissingRelatedEntities bool `json:"createMissingRelatedEntities,omitempty"`
	UpdateEntityOnlyOnDiff       bool `json:"updateEntityOnlyOnDiff,omitempty"`
	// HTTP Logging configuration
	HTTPLoggingEnabled bool   `json:"httpLoggingEnabled,omitempty"`
	LoggingLevel       string `json:"loggingLevel,omitempty"`
	HTTPLoggingTimeout int    `json:"httpLoggingTimeout,omitempty"` // in seconds
	// Bulk sync configuration
	BulkSyncMaxPayloadBytes     int `json:"bulkSyncMaxPayloadBytes,omitempty"`
	BulkSyncMaxEntitiesPerBatch int `json:"bulkSyncMaxEntitiesPerBatch,omitempty"`
	BulkSyncBatchTimeoutSeconds int `json:"bulkSyncBatchTimeoutSeconds,omitempty"`
	// Debug Mode
	DebugMode bool `json:"debugMode,omitempty"`
	// Metrics Configuration
	MetricsEnabled bool `json:"metricsEnabled,omitempty"`
	MetricsPort    int  `json:"metricsPort,omitempty"`
	// JQ Configuration
	AllowAllEnvironmentVariablesInJQ bool     `json:"allowAllEnvironmentVariablesInJQ,omitempty"`
	AllowedEnvironmentVariablesInJQ  []string `json:"allowedEnvironmentVariablesInJQ,omitempty"`

	// Webhook Configuration (NEW)
	WebhookURL          string `json:"webhookUrl,omitempty"`
	WebhookBatchSize    int    `json:"webhookBatchSize,omitempty"`
	WebhookBatchTimeout int    `json:"webhookBatchTimeout,omitempty"` // in seconds
	ClusterName         string `json:"clusterName,omitempty"`

	// Webhook Security Configuration (Advanced Settings)
	WebhookSecret              string `json:"webhookSecret,omitempty"`              // Secret for HMAC signature
	WebhookSignatureHeaderName string `json:"webhookSignatureHeaderName,omitempty"` // Header name for signature (e.g., X-Port-Signature)
	WebhookSignatureAlgorithm  string `json:"webhookSignatureAlgorithm,omitempty"`  // Algorithm: sha256, sha1, sha512
	WebhookSignaturePrefix     string `json:"webhookSignaturePrefix,omitempty"`     // Prefix for signature (e.g., "sha256=")
	WebhookRequestIdentifier   string `json:"webhookRequestIdentifier,omitempty"`   // JQ path to extract request identifier
}
