package webhook

// WebhookAction represents the action type for webhook events
type WebhookAction string

const (
	ActionUpsert WebhookAction = "upsert"
	ActionDelete WebhookAction = "delete"
)

// WebhookPayload represents the payload sent to Port's ingest webhook
type WebhookPayload struct {
	Action       WebhookAction `json:"action"`
	ResourceType string        `json:"resourceType"`
	ClusterName  string        `json:"clusterName"`
	StateKey     string        `json:"stateKey"`
	Namespace    string        `json:"namespace,omitempty"`
	Name         string        `json:"name"`
	Data         interface{}   `json:"data"`
}

// WebhookResponse represents the response from Port's webhook
type WebhookResponse struct {
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}
