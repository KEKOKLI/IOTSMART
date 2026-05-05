package ingest

import (
	"context"
	"fmt"
	"log"

	"iotsmart/backend/internal/db"
	"iotsmart/backend/internal/models"
	"iotsmart/backend/internal/websocket"
)

type Service struct {
	repo   *db.Repository
	hub    *websocket.Hub
	logger *log.Logger
}

func NewService(repo *db.Repository, hub *websocket.Hub, logger *log.Logger) *Service {
	return &Service{
		repo:   repo,
		hub:    hub,
		logger: logger,
	}
}

func (s *Service) ProcessReading(ctx context.Context, reading models.TelemetryRecord, protocol string) error {
	if reading.DeviceID == "" || reading.Metric == "" {
		return fmt.Errorf("device_id and metric are required")
	}
	if protocol != "" && reading.Protocol == "" {
		reading.Protocol = protocol
	}
	if err := s.repo.EnsureDeviceAndMetricFromReading(ctx, reading, protocol); err != nil {
		return err
	}
	if err := s.repo.InsertTelemetry(ctx, reading); err != nil {
		return err
	}
	s.hub.Broadcast(reading)
	return nil
}

func (s *Service) ProcessBatch(ctx context.Context, readings []models.TelemetryRecord, protocol string) error {
	for _, reading := range readings {
		if err := s.ProcessReading(ctx, reading, protocol); err != nil {
			s.logger.Printf("ingest batch reading failed: %v", err)
			return err
		}
	}
	return nil
}
