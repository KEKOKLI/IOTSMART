package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	paho "github.com/eclipse/paho.mqtt.golang"

	"iotsmart/backend/internal/config"
	"iotsmart/backend/internal/ingest"
	"iotsmart/backend/internal/models"
)

type Status struct {
	Enabled          bool       `json:"enabled"`
	Connected        bool       `json:"connected"`
	Subscription     string     `json:"subscription"`
	MessagesReceived int64      `json:"messages_received"`
	LastMessageAt    *time.Time `json:"last_message_at,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
}

type Connector struct {
	cfg      config.MQTTConfig
	ingestor *ingest.Service
	logger   *log.Logger

	mu     sync.RWMutex
	client paho.Client
	status Status
	cancel context.CancelFunc
}

func New(cfg config.MQTTConfig, ingestor *ingest.Service, logger *log.Logger) *Connector {
	return &Connector{
		cfg:      cfg,
		ingestor: ingestor,
		logger:   logger,
		status: Status{
			Enabled:      cfg.Enabled,
			Subscription: cfg.TopicPrefix,
		},
	}
}

func (c *Connector) Start(parent context.Context) error {
	if !c.cfg.Enabled {
		return nil
	}

	ctx, cancel := context.WithCancel(parent)
	c.cancel = cancel

	options := c.newClientOptions(fmt.Sprintf("iotsmart-%d", time.Now().UnixNano()))
	options.SetAutoReconnect(true)
	options.SetConnectRetry(true)
	options.SetConnectRetryInterval(10 * time.Second)
	options.SetConnectionLostHandler(func(_ paho.Client, err error) {
		c.setConnection(false, err)
	})
	options.SetOnConnectHandler(func(client paho.Client) {
		c.setConnection(true, nil)
		token := client.Subscribe(c.cfg.TopicPrefix, 1, c.handleMessage)
		token.Wait()
		if err := token.Error(); err != nil {
			c.setConnection(true, err)
			c.logger.Printf("mqtt subscribe error: %v", err)
			return
		}
		c.logger.Printf("mqtt subscribed to %s", c.cfg.TopicPrefix)
	})

	c.client = paho.NewClient(options)
	go c.run(ctx)
	return nil
}

func (c *Connector) TestConnection(ctx context.Context) (Status, error) {
	client := paho.NewClient(c.newClientOptions(fmt.Sprintf("iotsmart-test-%d", time.Now().UnixNano())))
	token := client.Connect()
	if !token.WaitTimeout(8 * time.Second) {
		return c.Snapshot(), fmt.Errorf("mqtt connection timed out")
	}
	if err := token.Error(); err != nil {
		c.setConnection(false, err)
		return c.Snapshot(), err
	}
	client.Disconnect(250)
	snapshot := c.Snapshot()
	snapshot.Connected = true
	snapshot.LastError = ""
	_ = ctx
	return snapshot, nil
}

func (c *Connector) PublishTest(ctx context.Context, topic string, payload []byte) error {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return fmt.Errorf("mqtt topic is required")
	}
	if !json.Valid(payload) {
		return fmt.Errorf("mqtt payload must be valid json")
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()
	if client != nil && client.IsConnectionOpen() {
		token := client.Publish(topic, 1, false, payload)
		if !token.WaitTimeout(8 * time.Second) {
			return fmt.Errorf("mqtt publish timed out")
		}
		return token.Error()
	}

	ephemeral := paho.NewClient(c.newClientOptions(fmt.Sprintf("iotsmart-publish-%d", time.Now().UnixNano())))
	connectToken := ephemeral.Connect()
	if !connectToken.WaitTimeout(8 * time.Second) {
		return fmt.Errorf("mqtt connection timed out")
	}
	if err := connectToken.Error(); err != nil {
		c.setConnection(false, err)
		return err
	}
	defer ephemeral.Disconnect(250)

	publishToken := ephemeral.Publish(topic, 1, false, payload)
	if !publishToken.WaitTimeout(8 * time.Second) {
		return fmt.Errorf("mqtt publish timed out")
	}
	_ = ctx
	return publishToken.Error()
}

func (c *Connector) Stop() {
	if c.cancel != nil {
		c.cancel()
	}
	if c.client != nil && c.client.IsConnectionOpen() {
		c.client.Disconnect(250)
	}
}

func (c *Connector) WorkerCount() int {
	if c.cfg.Enabled {
		return 1
	}
	return 0
}

func (c *Connector) Snapshot() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Connector) newClientOptions(clientID string) *paho.ClientOptions {
	options := paho.NewClientOptions()
	options.AddBroker(c.cfg.Broker)
	options.SetClientID(clientID)
	options.SetKeepAlive(30 * time.Second)
	options.SetPingTimeout(5 * time.Second)
	options.SetOrderMatters(false)
	options.SetUsername(c.cfg.Username)
	options.SetPassword(c.cfg.Password)
	return options
}

func (c *Connector) run(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		if c.client != nil && !c.client.IsConnected() {
			token := c.client.Connect()
			if token.WaitTimeout(8*time.Second) && token.Error() != nil {
				c.setConnection(false, token.Error())
				c.logger.Printf("mqtt connect error: %v", token.Error())
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (c *Connector) handleMessage(_ paho.Client, message paho.Message) {
	readings, err := parsePayload(message.Topic(), message.Payload())
	if err != nil {
		c.setLastError(err)
		c.logger.Printf("mqtt payload error: %v", err)
		return
	}

	for _, reading := range readings {
		if err := c.ingestor.ProcessReading(context.Background(), reading, "mqtt"); err != nil {
			c.setLastError(err)
			c.logger.Printf("mqtt ingest error: %v", err)
			continue
		}
	}

	c.mu.Lock()
	now := time.Now().UTC()
	c.status.MessagesReceived += int64(len(readings))
	c.status.LastMessageAt = &now
	c.status.LastError = ""
	c.mu.Unlock()
}

func (c *Connector) setConnection(connected bool, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.Connected = connected
	if err != nil {
		c.status.LastError = err.Error()
	} else if connected {
		c.status.LastError = ""
	}
}

func (c *Connector) setLastError(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.LastError = err.Error()
}

func parsePayload(topic string, payload []byte) ([]models.TelemetryRecord, error) {
	var array []map[string]any
	if err := json.Unmarshal(payload, &array); err == nil {
		return parseReadingMaps(topic, array)
	}

	var object map[string]any
	if err := json.Unmarshal(payload, &object); err != nil {
		return nil, fmt.Errorf("invalid json payload")
	}
	if readings, ok := object["readings"].([]any); ok {
		items := make([]map[string]any, 0, len(readings))
		for _, item := range readings {
			record, ok := item.(map[string]any)
			if ok {
				for key, value := range object {
					if key != "readings" {
						if _, exists := record[key]; !exists {
							record[key] = value
						}
					}
				}
				items = append(items, record)
			}
		}
		if len(items) > 0 {
			return parseReadingMaps(topic, items)
		}
	}

	if metrics, ok := object["metrics"].(map[string]any); ok {
		units, _ := object["units"].(map[string]any)
		deviceID := chooseDeviceID(topic, object)
		name, _ := object["name"].(string)
		protocol, _ := object["protocol"].(string)
		quality, _ := object["quality"].(string)
		baseTime := parseTimestamp(object["timestamp"])
		if baseTime.IsZero() {
			baseTime = time.Now().UTC()
		}
		result := make([]models.TelemetryRecord, 0, len(metrics))
		for metric, raw := range metrics {
			value, ok := toFloat64(raw)
			if !ok {
				continue
			}
			unit := ""
			if units != nil {
				if rawUnit, exists := units[metric]; exists {
					unit, _ = rawUnit.(string)
				}
			}
			result = append(result, models.TelemetryRecord{
				DeviceID:  deviceID,
				Name:      name,
				Protocol:  choose(protocol, "mqtt"),
				Metric:    metric,
				Value:     value,
				Unit:      unit,
				Quality:   choose(quality, "good"),
				Timestamp: baseTime,
			})
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	return parseReadingMaps(topic, []map[string]any{object})
}

func parseReadingMaps(topic string, items []map[string]any) ([]models.TelemetryRecord, error) {
	result := make([]models.TelemetryRecord, 0, len(items))
	for _, item := range items {
		deviceID := chooseDeviceID(topic, item)
		metric, _ := item["metric"].(string)
		value, ok := toFloat64(item["value"])
		if !ok || deviceID == "" || metric == "" {
			continue
		}
		name, _ := item["name"].(string)
		protocol, _ := item["protocol"].(string)
		unit, _ := item["unit"].(string)
		quality, _ := item["quality"].(string)
		timestamp := parseTimestamp(item["timestamp"])
		if timestamp.IsZero() {
			timestamp = time.Now().UTC()
		}
		result = append(result, models.TelemetryRecord{
			DeviceID:  deviceID,
			Name:      name,
			Protocol:  choose(protocol, "mqtt"),
			Metric:    metric,
			Value:     value,
			Unit:      unit,
			Quality:   choose(quality, "good"),
			Timestamp: timestamp,
		})
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("payload did not contain any supported readings")
	}
	return result, nil
}

func chooseDeviceID(topic string, payload map[string]any) string {
	if raw, ok := payload["device_id"].(string); ok && strings.TrimSpace(raw) != "" {
		return raw
	}
	parts := strings.Split(strings.Trim(topic, "/"), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if part == "" || strings.EqualFold(part, "telemetry") {
			continue
		}
		return part
	}
	return ""
}

func parseTimestamp(raw any) time.Time {
	value, _ := raw.(string)
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func toFloat64(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func choose(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
