package webhook

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/port-labs/port-k8s-exporter/pkg/goutils"
	"github.com/port-labs/port-k8s-exporter/pkg/logger"
)

// Client handles sending events to Port's ingest webhook
type Client struct {
	httpClient   *resty.Client
	webhookURL   string
	stateKey     string
	clusterName  string
	batchSize    int
	batchTimeout time.Duration

	buffer      []WebhookPayload
	bufferMutex sync.Mutex
	lastFlush   time.Time
	flushChan   chan struct{}
	doneChan    chan struct{}
	wg          sync.WaitGroup
}

// NewClient creates a new webhook client
func NewClient(webhookURL, stateKey, clusterName string, batchSize int, batchTimeout time.Duration) *Client {
	httpClient := resty.New().
		SetBaseURL(webhookURL).
		SetTimeout(30 * time.Second).
		SetRetryCount(3).
		SetRetryWaitTime(1 * time.Second).
		SetRetryMaxWaitTime(10 * time.Second).
		AddRetryCondition(func(r *resty.Response, err error) bool {
			if err != nil {
				return true
			}
			return goutils.IsRetryableStatusCode(r.StatusCode())
		})

	c := &Client{
		httpClient:   httpClient,
		webhookURL:   webhookURL,
		stateKey:     stateKey,
		clusterName:  clusterName,
		batchSize:    batchSize,
		batchTimeout: batchTimeout,
		buffer:       make([]WebhookPayload, 0, batchSize),
		lastFlush:    time.Now(),
		flushChan:    make(chan struct{}, 1),
		doneChan:     make(chan struct{}),
	}

	c.wg.Add(1)
	go c.backgroundFlusher()

	return c
}

// Enqueue adds a payload to the buffer
func (c *Client) Enqueue(payload WebhookPayload) {
	c.bufferMutex.Lock()
	c.buffer = append(c.buffer, payload)
	shouldFlush := len(c.buffer) >= c.batchSize
	c.bufferMutex.Unlock()

	if shouldFlush {
		select {
		case c.flushChan <- struct{}{}:
		default:
		}
	}
}

// SendUpsert queues an upsert event
func (c *Client) SendUpsert(resourceType, namespace, name string, rawObject interface{}) {
	c.Enqueue(WebhookPayload{
		Action:       ActionUpsert,
		ResourceType: resourceType,
		ClusterName:  c.clusterName,
		StateKey:     c.stateKey,
		Namespace:    namespace,
		Name:         name,
		Data:         rawObject,
	})
}

// SendDelete queues a delete event
func (c *Client) SendDelete(resourceType, namespace, name string) {
	c.Enqueue(WebhookPayload{
		Action:       ActionDelete,
		ResourceType: resourceType,
		ClusterName:  c.clusterName,
		StateKey:     c.stateKey,
		Namespace:    namespace,
		Name:         name,
		Data: map[string]string{
			"name":      name,
			"namespace": namespace,
			"kind":      resourceType,
		},
	})
}

func (c *Client) backgroundFlusher() {
	defer c.wg.Done()
	ticker := time.NewTicker(c.batchTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-c.doneChan:
			c.flush()
			return
		case <-c.flushChan:
			c.flush()
		case <-ticker.C:
			c.bufferMutex.Lock()
			shouldFlush := len(c.buffer) > 0 && time.Since(c.lastFlush) >= c.batchTimeout
			c.bufferMutex.Unlock()
			if shouldFlush {
				c.flush()
			}
		}
	}
}

func (c *Client) flush() {
	c.bufferMutex.Lock()
	if len(c.buffer) == 0 {
		c.bufferMutex.Unlock()
		return
	}

	payloads := make([]WebhookPayload, len(c.buffer))
	copy(payloads, c.buffer)
	c.buffer = c.buffer[:0]
	c.lastFlush = time.Now()
	c.bufferMutex.Unlock()

	logger.Infow("Flushing webhook batch", "count", len(payloads))

	for _, payload := range payloads {
		c.send(payload)
	}
}

func (c *Client) send(payload WebhookPayload) error {
	req := c.httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(payload)

	if payload.Action == ActionDelete {
		req.SetHeader("X-Port-Delete", "true")
	}

	resp, err := req.Post("")
	if err != nil {
		logger.Errorw("Webhook request failed",
			"resourceType", payload.ResourceType,
			"name", payload.Name,
			"action", payload.Action,
			"error", err)
		return err
	}

	if resp.StatusCode() >= 400 {
		logger.Errorw("Webhook returned error",
			"resourceType", payload.ResourceType,
			"name", payload.Name,
			"status", resp.StatusCode(),
			"body", string(resp.Body()))
		return fmt.Errorf("webhook returned status %d", resp.StatusCode())
	}

	logger.Debugw("Webhook payload sent",
		"resourceType", payload.ResourceType,
		"name", payload.Name,
		"action", payload.Action)

	return nil
}

// FlushSync forces synchronous flush
func (c *Client) FlushSync() {
	c.flush()
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown() {
	close(c.doneChan)
	c.wg.Wait()
}
