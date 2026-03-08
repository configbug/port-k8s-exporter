package webhook

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/port-labs/port-k8s-exporter/pkg/goutils"
	"github.com/port-labs/port-k8s-exporter/pkg/logger"
)

// SecurityConfig holds webhook security settings
type SecurityConfig struct {
	Secret              string // Secret for HMAC signature
	SignatureHeaderName string // Header name for signature
	SignatureAlgorithm  string // Algorithm: sha256, sha1, sha512
	SignaturePrefix     string // Prefix for signature value
	RequestIdentifier   string // JQ path for request identifier
}

// Client handles sending events to Port's ingest webhook
type Client struct {
	httpClient   *resty.Client
	webhookURL   string
	stateKey     string
	clusterName  string
	batchSize    int
	batchTimeout time.Duration
	security     SecurityConfig

	// Buffer management
	buffer        []WebhookPayload
	bufferMutex   sync.Mutex
	maxBufferSize int        // Maximum buffer size (backpressure)
	bufferCond    *sync.Cond // Condition variable for backpressure

	// Worker pool
	numWorkers int
	workerChan chan WebhookPayload

	// Rate limiting
	rateLimiter <-chan time.Time

	// Lifecycle
	lastFlush time.Time
	flushChan chan struct{}
	doneChan  chan struct{}
	wg        sync.WaitGroup

	// Metrics
	droppedCount int64
	sentCount    int64
}

// NewClient creates a new webhook client
func NewClient(webhookURL, stateKey, clusterName string, batchSize int, batchTimeout time.Duration, security SecurityConfig) *Client {
	return NewClientWithOptions(webhookURL, stateKey, clusterName, batchSize, batchTimeout, security, 1000, 5, 100)
}

// NewClientWithOptions creates a new webhook client with advanced options
// maxBufferSize: maximum payloads in buffer before backpressure (default 1000)
// numWorkers: number of parallel workers for sending (default 5)
// rateLimit: max requests per second (default 100)
func NewClientWithOptions(webhookURL, stateKey, clusterName string, batchSize int, batchTimeout time.Duration, security SecurityConfig, maxBufferSize, numWorkers, rateLimit int) *Client {
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

	// Set default values for security config
	if security.SignatureHeaderName == "" {
		security.SignatureHeaderName = "X-Port-Signature"
	}
	if security.SignatureAlgorithm == "" {
		security.SignatureAlgorithm = "sha256"
	}

	// Set sensible defaults
	if maxBufferSize <= 0 {
		maxBufferSize = 1000
	}
	if numWorkers <= 0 {
		numWorkers = 5
	}
	if rateLimit <= 0 {
		rateLimit = 100
	}

	c := &Client{
		httpClient:    httpClient,
		webhookURL:    webhookURL,
		stateKey:      stateKey,
		clusterName:   clusterName,
		batchSize:     batchSize,
		batchTimeout:  batchTimeout,
		security:      security,
		buffer:        make([]WebhookPayload, 0, batchSize),
		maxBufferSize: maxBufferSize,
		numWorkers:    numWorkers,
		workerChan:    make(chan WebhookPayload, numWorkers*10), // Buffered channel for workers
		rateLimiter:   time.Tick(time.Second / time.Duration(rateLimit)),
		lastFlush:     time.Now(),
		flushChan:     make(chan struct{}, 1),
		doneChan:      make(chan struct{}),
	}

	c.bufferCond = sync.NewCond(&c.bufferMutex)

	// Start background flusher
	c.wg.Add(1)
	go c.backgroundFlusher()

	// Start worker pool
	for i := 0; i < numWorkers; i++ {
		c.wg.Add(1)
		go c.worker(i)
	}

	logger.Infow("Webhook client initialized",
		"maxBufferSize", maxBufferSize,
		"numWorkers", numWorkers,
		"rateLimit", rateLimit,
		"batchSize", batchSize)

	return c
}

// Enqueue adds a payload to the buffer with backpressure support
func (c *Client) Enqueue(payload WebhookPayload) {
	c.bufferMutex.Lock()

	// Backpressure: wait if buffer is full (with timeout)
	waitStart := time.Now()
	maxWait := 5 * time.Second
	for len(c.buffer) >= c.maxBufferSize {
		// Check if we've been waiting too long
		if time.Since(waitStart) > maxWait {
			atomic.AddInt64(&c.droppedCount, 1)
			c.bufferMutex.Unlock()
			logger.Warnw("Dropped webhook payload due to buffer overflow",
				"resourceType", payload.ResourceType,
				"name", payload.Name,
				"bufferSize", c.maxBufferSize,
				"droppedTotal", atomic.LoadInt64(&c.droppedCount))
			return
		}
		// Wait for signal that buffer has space (with timeout)
		c.bufferCond.Wait()
	}

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
	// Clean the object before sending to reduce memory and payload size
	cleanedObject := cleanK8sObject(rawObject)

	c.Enqueue(WebhookPayload{
		Action:       ActionUpsert,
		ResourceType: resourceType,
		ClusterName:  c.clusterName,
		StateKey:     c.stateKey,
		Namespace:    namespace,
		Name:         name,
		Data:         cleanedObject,
	})
}

// cleanK8sObject removes unnecessary fields from K8s objects to reduce memory usage
// This removes managedFields and last-applied-configuration which can be 30-50% of the object size
func cleanK8sObject(obj interface{}) interface{} {
	objMap, ok := obj.(map[string]interface{})
	if !ok {
		return obj
	}

	// Create a shallow copy to avoid modifying the original
	cleaned := make(map[string]interface{}, len(objMap))
	for k, v := range objMap {
		cleaned[k] = v
	}

	// Clean metadata
	if metadata, ok := cleaned["metadata"].(map[string]interface{}); ok {
		cleanedMetadata := make(map[string]interface{}, len(metadata))
		for k, v := range metadata {
			cleanedMetadata[k] = v
		}

		// Remove managedFields (very large, not needed for Port mappings)
		delete(cleanedMetadata, "managedFields")

		// Clean annotations
		if annotations, ok := cleanedMetadata["annotations"].(map[string]interface{}); ok {
			cleanedAnnotations := make(map[string]interface{}, len(annotations))
			for k, v := range annotations {
				// Remove kubectl.kubernetes.io/last-applied-configuration (duplicates entire object)
				if k == "kubectl.kubernetes.io/last-applied-configuration" {
					continue
				}
				cleanedAnnotations[k] = v
			}
			if len(cleanedAnnotations) > 0 {
				cleanedMetadata["annotations"] = cleanedAnnotations
			} else {
				delete(cleanedMetadata, "annotations")
			}
		}

		cleaned["metadata"] = cleanedMetadata
	}

	return cleaned
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

// worker is a goroutine that processes payloads from the worker channel
func (c *Client) worker(id int) {
	defer c.wg.Done()

	for payload := range c.workerChan {
		// Rate limiting: wait for token
		<-c.rateLimiter

		if err := c.send(payload); err != nil {
			logger.Debugw("Worker failed to send payload",
				"worker", id,
				"resourceType", payload.ResourceType,
				"name", payload.Name,
				"error", err)
		} else {
			atomic.AddInt64(&c.sentCount, 1)
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

	// Signal that buffer has space for waiting goroutines
	c.bufferCond.Broadcast()
	c.bufferMutex.Unlock()

	logger.Infow("Flushing webhook batch",
		"count", len(payloads),
		"sentTotal", atomic.LoadInt64(&c.sentCount),
		"droppedTotal", atomic.LoadInt64(&c.droppedCount))

	// Send payloads to worker pool (non-blocking with timeout)
	for _, payload := range payloads {
		select {
		case c.workerChan <- payload:
			// Successfully queued to worker
		case <-time.After(100 * time.Millisecond):
			// Worker channel full, log warning but don't block
			logger.Warnw("Worker channel full, payload may be delayed",
				"resourceType", payload.ResourceType,
				"name", payload.Name)
			// Try one more time with blocking
			c.workerChan <- payload
		}
	}
}

func (c *Client) send(payload WebhookPayload) error {
	// Serialize payload for signature
	body, err := json.Marshal(payload)
	if err != nil {
		logger.Errorw("Failed to marshal payload", "error", err)
		return err
	}

	req := c.httpClient.R().
		SetHeader("Content-Type", "application/json").
		SetBody(body)

	// Add signature header if secret is configured
	if c.security.Secret != "" {
		signature := c.computeSignature(body)
		req.SetHeader(c.security.SignatureHeaderName, signature)
		logger.Debugw("Added signature header",
			"header", c.security.SignatureHeaderName,
			"algorithm", c.security.SignatureAlgorithm)
	}

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

// computeSignature computes HMAC signature for the payload
func (c *Client) computeSignature(payload []byte) string {
	var h hash.Hash

	switch c.security.SignatureAlgorithm {
	case "sha1":
		h = hmac.New(sha1.New, []byte(c.security.Secret))
	case "sha512":
		h = hmac.New(sha512.New, []byte(c.security.Secret))
	case "sha256":
		fallthrough
	default:
		h = hmac.New(sha256.New, []byte(c.security.Secret))
	}

	h.Write(payload)
	signature := hex.EncodeToString(h.Sum(nil))

	// Add prefix if configured
	if c.security.SignaturePrefix != "" {
		signature = c.security.SignaturePrefix + signature
	}

	return signature
}

// FlushSync forces synchronous flush
func (c *Client) FlushSync() {
	c.flush()
	// Wait a bit for workers to process
	time.Sleep(100 * time.Millisecond)
}

// Shutdown gracefully shuts down the client
func (c *Client) Shutdown() {
	logger.Infow("Shutting down webhook client",
		"sentTotal", atomic.LoadInt64(&c.sentCount),
		"droppedTotal", atomic.LoadInt64(&c.droppedCount))

	// Signal background flusher to stop
	close(c.doneChan)

	// Give some time for final flush
	time.Sleep(200 * time.Millisecond)

	// Close worker channel (this will cause workers to exit after processing remaining items)
	close(c.workerChan)

	// Wait for all goroutines to finish
	c.wg.Wait()

	logger.Infow("Webhook client shutdown complete",
		"finalSentTotal", atomic.LoadInt64(&c.sentCount),
		"finalDroppedTotal", atomic.LoadInt64(&c.droppedCount))
}

// GetStats returns the current stats for monitoring
func (c *Client) GetStats() (sent, dropped int64, bufferLen int) {
	c.bufferMutex.Lock()
	bufferLen = len(c.buffer)
	c.bufferMutex.Unlock()
	return atomic.LoadInt64(&c.sentCount), atomic.LoadInt64(&c.droppedCount), bufferLen
}
