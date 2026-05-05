package scheduler

import (
	"context"
	"fmt"
	"log"
	"math"
	"time"

	"iotsmart/backend/internal/config"
	"iotsmart/backend/internal/db"
	"iotsmart/backend/internal/models"
	"iotsmart/backend/internal/websocket"
)

type Simulator struct {
	cfg     config.Config
	repo    *db.Repository
	hub     *websocket.Hub
	logger  *log.Logger
	cancel  context.CancelFunc
	running bool
}

func NewSimulator(cfg config.Config, repo *db.Repository, hub *websocket.Hub, logger *log.Logger) *Simulator {
	return &Simulator{
		cfg:    cfg,
		repo:   repo,
		hub:    hub,
		logger: logger,
	}
}

func (s *Simulator) Start(parent context.Context) error {
	if !s.cfg.Simulator.Enabled || s.running {
		return nil
	}

	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	if err := s.seedDevices(ctx); err != nil {
		cancel()
		return err
	}

	s.publishBatch(ctx)

	s.running = true
	go s.loop(ctx)
	return nil
}

func (s *Simulator) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

func (s *Simulator) WorkerCount() int {
	if s.cfg.Simulator.Enabled {
		return 1
	}
	return 0
}

func (s *Simulator) loop(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(s.cfg.Simulator.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.logger.Println("simulator stopped")
			return
		case <-ticker.C:
			s.publishBatch(ctx)
		}
	}
}

func (s *Simulator) publishBatch(ctx context.Context) {
	now := time.Now().UTC()
	for i := 1; i <= s.cfg.Simulator.DeviceCount; i++ {
		deviceID := fmt.Sprintf("sim_%03d", i)
		readings := []models.TelemetryRecord{
			{
				DeviceID:  deviceID,
				Name:      fmt.Sprintf("Simulated Sensor %03d", i),
				Protocol:  "simulator",
				Metric:    "temperature",
				Value:     21.5 + float64(i%5) + waveform(i, now, 2.2),
				Unit:      "C",
				Quality:   "good",
				Timestamp: now,
			},
			{
				DeviceID:  deviceID,
				Name:      fmt.Sprintf("Simulated Sensor %03d", i),
				Protocol:  "simulator",
				Metric:    "humidity",
				Value:     45 + float64(i%12) + waveform(i+7, now, 6.0),
				Unit:      "%",
				Quality:   "good",
				Timestamp: now,
			},
		}

		if i%4 == 0 {
			readings = append(readings, models.TelemetryRecord{
				DeviceID:  deviceID,
				Name:      fmt.Sprintf("Simulated Sensor %03d", i),
				Protocol:  "simulator",
				Metric:    "voltage",
				Value:     220 + waveform(i+13, now, 5.0),
				Unit:      "V",
				Quality:   "good",
				Timestamp: now,
			})
		}

		for _, reading := range readings {
			if err := s.repo.InsertTelemetry(ctx, reading); err != nil {
				s.logger.Printf("simulator insert telemetry: %v", err)
				continue
			}
			s.hub.Broadcast(reading)
		}
	}
}

func (s *Simulator) seedDevices(ctx context.Context) error {
	if err := s.repo.EnsureProject(ctx, s.cfg.Simulator.ProjectID, s.cfg.Simulator.ProjectName, "built-in simulator devices"); err != nil {
		return err
	}

	for i := 1; i <= s.cfg.Simulator.DeviceCount; i++ {
		device := models.Device{
			ID:        fmt.Sprintf("sim_%03d", i),
			ProjectID: s.cfg.Simulator.ProjectID,
			Name:      fmt.Sprintf("Simulated Sensor %03d", i),
			Protocol:  "simulator",
			Location:  fmt.Sprintf("Zone %02d", (i-1)/5+1),
			Enabled:   true,
			Simulated: true,
			CreatedAt: time.Now().UTC(),
			Metrics: []models.SensorMetric{
				{
					Metric:   "temperature",
					Unit:     "C",
					DataType: "float",
					MinValue: floatPtr(-20),
					MaxValue: floatPtr(80),
				},
				{
					Metric:   "humidity",
					Unit:     "%",
					DataType: "float",
					MinValue: floatPtr(0),
					MaxValue: floatPtr(100),
				},
			},
		}

		if i%4 == 0 {
			device.Metrics = append(device.Metrics, models.SensorMetric{
				Metric:   "voltage",
				Unit:     "V",
				DataType: "float",
				MinValue: floatPtr(180),
				MaxValue: floatPtr(260),
			})
		}

		if err := s.repo.SaveDevice(ctx, device); err != nil {
			return err
		}
	}
	return nil
}

func waveform(seed int, ts time.Time, amplitude float64) float64 {
	phase := float64(ts.Unix()/30+int64(seed)) / 12.0
	return math.Sin(phase) * amplitude
}

func floatPtr(value float64) *float64 {
	return &value
}
