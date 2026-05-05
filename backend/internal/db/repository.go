package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"iotsmart/backend/internal/models"
)

const timeLayout = time.RFC3339

type Repository struct {
	db *sql.DB
}

type HealthSummary struct {
	TotalDevices   int
	EnabledDevices int
	OnlineDevices  int
	OfflineDevices int
	LastIngestAt   *time.Time
}

func NewRepository(database *sql.DB) *Repository {
	return &Repository{db: database}
}

func (r *Repository) EnsureProject(ctx context.Context, id, name, description string) error {
	if id == "" {
		return fmt.Errorf("project id is required")
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO projects (id, name, description, created_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			description = excluded.description
	`, id, name, description, timeString(time.Now()))
	return err
}

func (r *Repository) SaveDevice(ctx context.Context, device models.Device) error {
	if device.ID == "" {
		return fmt.Errorf("device id is required")
	}
	if device.ProjectID == "" {
		device.ProjectID = "default-project"
	}
	if device.Name == "" {
		device.Name = device.ID
	}
	if device.Protocol == "" {
		device.Protocol = "unknown"
	}
	if device.CreatedAt.IsZero() {
		device.CreatedAt = time.Now()
	}

	if err := r.EnsureProject(ctx, device.ProjectID, strings.ReplaceAll(device.ProjectID, "-", " "), "auto-created"); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO devices (id, project_id, name, protocol, location, enabled, simulated, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			project_id = excluded.project_id,
			name = excluded.name,
			protocol = excluded.protocol,
			location = excluded.location,
			enabled = excluded.enabled,
			simulated = excluded.simulated
	`, device.ID, device.ProjectID, device.Name, device.Protocol, device.Location, boolToInt(device.Enabled), boolToInt(device.Simulated), timeString(device.CreatedAt))
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM sensor_metrics WHERE device_id = ?`, device.ID); err != nil {
		return err
	}

	for _, metric := range device.Metrics {
		if err := insertMetric(ctx, tx, metricForDevice(device.ID, metric)); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *Repository) UpsertMetric(ctx context.Context, metric models.SensorMetric) error {
	return insertMetric(ctx, r.db, metric)
}

func (r *Repository) EnsureDeviceAndMetricFromReading(ctx context.Context, reading models.TelemetryRecord, protocol string) error {
	device := models.Device{
		ID:        reading.DeviceID,
		ProjectID: "ingest-project",
		Name:      choose(reading.Name, reading.DeviceID),
		Protocol:  choose(protocol, choose(reading.Protocol, "http_ingest")),
		Enabled:   true,
		CreatedAt: time.Now(),
	}
	if err := r.EnsureProject(ctx, device.ProjectID, "Ingest Project", "auto-created from ingest"); err != nil {
		return err
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO devices (id, project_id, name, protocol, location, enabled, simulated, created_at)
		VALUES (?, ?, ?, ?, '', 1, 0, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			protocol = excluded.protocol,
			enabled = 1
	`, device.ID, device.ProjectID, device.Name, device.Protocol, timeString(device.CreatedAt))
	if err != nil {
		return err
	}

	return r.UpsertMetric(ctx, models.SensorMetric{
		ID:        metricID(reading.DeviceID, reading.Metric),
		DeviceID:  reading.DeviceID,
		Metric:    reading.Metric,
		Unit:      reading.Unit,
		DataType:  "float",
		CreatedAt: time.Now(),
	})
}

func (r *Repository) ListDevices(ctx context.Context) ([]models.Device, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, project_id, name, protocol, location, enabled, simulated, created_at
		FROM devices
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []models.Device
	deviceIndex := make(map[string]int)
	for rows.Next() {
		var device models.Device
		var enabled int
		var simulated int
		var createdAt string
		if err := rows.Scan(&device.ID, &device.ProjectID, &device.Name, &device.Protocol, &device.Location, &enabled, &simulated, &createdAt); err != nil {
			return nil, err
		}
		device.Enabled = enabled == 1
		device.Simulated = simulated == 1
		device.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		devices = append(devices, device)
		deviceIndex[device.ID] = len(devices) - 1
	}

	metricRows, err := r.db.QueryContext(ctx, `
		SELECT id, device_id, metric, unit, data_type, min_value, max_value, created_at
		FROM sensor_metrics
		ORDER BY device_id, metric
	`)
	if err != nil {
		return nil, err
	}
	defer metricRows.Close()

	for metricRows.Next() {
		var metric models.SensorMetric
		var minValue sql.NullFloat64
		var maxValue sql.NullFloat64
		var createdAt string
		if err := metricRows.Scan(&metric.ID, &metric.DeviceID, &metric.Metric, &metric.Unit, &metric.DataType, &minValue, &maxValue, &createdAt); err != nil {
			return nil, err
		}
		if minValue.Valid {
			metric.MinValue = &minValue.Float64
		}
		if maxValue.Valid {
			metric.MaxValue = &maxValue.Float64
		}
		metric.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		if index, ok := deviceIndex[metric.DeviceID]; ok {
			devices[index].Metrics = append(devices[index].Metrics, metric)
		}
	}

	return devices, rows.Err()
}

func (r *Repository) DeleteDevice(ctx context.Context, id string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sensor_metrics WHERE device_id = ?`, id); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) InsertTelemetry(ctx context.Context, reading models.TelemetryRecord) error {
	if reading.Quality == "" {
		reading.Quality = "good"
	}
	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO telemetry (device_id, metric, value, unit, quality, ts)
		VALUES (?, ?, ?, ?, ?, ?)
	`, reading.DeviceID, reading.Metric, reading.Value, reading.Unit, reading.Quality, timeString(reading.Timestamp))
	return err
}

func (r *Repository) ListLatestTelemetry(ctx context.Context, limit int) ([]models.TelemetryRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		WITH ranked AS (
			SELECT device_id, metric, value, unit, quality, ts,
				ROW_NUMBER() OVER (PARTITION BY device_id, metric ORDER BY ts DESC, id DESC) AS rn
			FROM telemetry
		)
		SELECT device_id, metric, value, unit, quality, ts
		FROM ranked
		WHERE rn = 1
		ORDER BY ts DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TelemetryRecord
	for rows.Next() {
		var record models.TelemetryRecord
		var ts string
		if err := rows.Scan(&record.DeviceID, &record.Metric, &record.Value, &record.Unit, &record.Quality, &ts); err != nil {
			return nil, err
		}
		record.Timestamp, err = parseTime(ts)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (r *Repository) QueryTelemetry(ctx context.Context, filter models.TelemetryQueryFilter) ([]models.TelemetryRecord, error) {
	if filter.Limit <= 0 || filter.Limit > 2000 {
		filter.Limit = 500
	}

	clauses := []string{"1=1"}
	args := make([]any, 0, 5)
	if filter.DeviceID != "" {
		clauses = append(clauses, "device_id = ?")
		args = append(args, filter.DeviceID)
	}
	if filter.Metric != "" {
		clauses = append(clauses, "metric = ?")
		args = append(args, filter.Metric)
	}
	if filter.From != nil {
		clauses = append(clauses, "ts >= ?")
		args = append(args, timeString(*filter.From))
	}
	if filter.To != nil {
		clauses = append(clauses, "ts <= ?")
		args = append(args, timeString(*filter.To))
	}
	args = append(args, filter.Limit)

	query := fmt.Sprintf(`
		SELECT id, device_id, metric, value, unit, quality, ts
		FROM telemetry
		WHERE %s
		ORDER BY ts DESC, id DESC
		LIMIT ?
	`, strings.Join(clauses, " AND "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.TelemetryRecord
	for rows.Next() {
		var record models.TelemetryRecord
		var ts string
		if err := rows.Scan(&record.ID, &record.DeviceID, &record.Metric, &record.Value, &record.Unit, &record.Quality, &ts); err != nil {
			return nil, err
		}
		record.Timestamp, err = parseTime(ts)
		if err != nil {
			return nil, err
		}
		result = append(result, record)
	}
	return result, rows.Err()
}

func (r *Repository) SaveGraphPreset(ctx context.Context, preset models.GraphPreset) error {
	if preset.ID == "" {
		preset.ID = fmt.Sprintf("graph_%d", time.Now().UnixNano())
	}
	if preset.CreatedAt.IsZero() {
		preset.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO graph_presets (id, name, device_id, metric, graph_type, time_range, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			device_id = excluded.device_id,
			metric = excluded.metric,
			graph_type = excluded.graph_type,
			time_range = excluded.time_range
	`, preset.ID, preset.Name, preset.DeviceID, preset.Metric, preset.GraphType, preset.TimeRange, timeString(preset.CreatedAt))
	return err
}

func (r *Repository) ListGraphPresets(ctx context.Context) ([]models.GraphPreset, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, name, device_id, metric, graph_type, time_range, created_at
		FROM graph_presets
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.GraphPreset
	for rows.Next() {
		var preset models.GraphPreset
		var createdAt string
		if err := rows.Scan(&preset.ID, &preset.Name, &preset.DeviceID, &preset.Metric, &preset.GraphType, &preset.TimeRange, &createdAt); err != nil {
			return nil, err
		}
		preset.CreatedAt, err = parseTime(createdAt)
		if err != nil {
			return nil, err
		}
		result = append(result, preset)
	}
	return result, rows.Err()
}

func (r *Repository) GetHealthSummary(ctx context.Context, offlineAfter time.Duration) (HealthSummary, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT d.enabled, MAX(t.ts)
		FROM devices d
		LEFT JOIN telemetry t ON t.device_id = d.id
		GROUP BY d.id, d.enabled
	`)
	if err != nil {
		return HealthSummary{}, err
	}
	defer rows.Close()

	summary := HealthSummary{}
	var latest *time.Time
	cutoff := time.Now().UTC().Add(-offlineAfter)

	for rows.Next() {
		var enabled int
		var lastTS sql.NullString
		if err := rows.Scan(&enabled, &lastTS); err != nil {
			return summary, err
		}
		summary.TotalDevices++
		if enabled == 1 {
			summary.EnabledDevices++
		}
		if !lastTS.Valid {
			if enabled == 1 {
				summary.OfflineDevices++
			}
			continue
		}

		parsed, err := parseTime(lastTS.String)
		if err != nil {
			return summary, err
		}
		if latest == nil || parsed.After(*latest) {
			copy := parsed
			latest = &copy
		}
		if enabled == 1 {
			if parsed.After(cutoff) || parsed.Equal(cutoff) {
				summary.OnlineDevices++
			} else {
				summary.OfflineDevices++
			}
		}
	}

	summary.LastIngestAt = latest
	return summary, rows.Err()
}

func insertMetric(ctx context.Context, exec sqlExecutor, metric models.SensorMetric) error {
	metric = metricForDevice(metric.DeviceID, metric)
	_, err := exec.ExecContext(ctx, `
		INSERT INTO sensor_metrics (id, device_id, metric, unit, data_type, min_value, max_value, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			unit = excluded.unit,
			data_type = excluded.data_type,
			min_value = excluded.min_value,
			max_value = excluded.max_value
	`, metric.ID, metric.DeviceID, metric.Metric, metric.Unit, metric.DataType, metric.MinValue, metric.MaxValue, timeString(metric.CreatedAt))
	return err
}

func metricForDevice(deviceID string, metric models.SensorMetric) models.SensorMetric {
	if metric.DeviceID == "" {
		metric.DeviceID = deviceID
	}
	if metric.ID == "" {
		metric.ID = metricID(metric.DeviceID, metric.Metric)
	}
	if metric.DataType == "" {
		metric.DataType = "float"
	}
	if metric.CreatedAt.IsZero() {
		metric.CreatedAt = time.Now().UTC()
	}
	return metric
}

func metricID(deviceID, metric string) string {
	return fmt.Sprintf("%s_%s", deviceID, metric)
}

func timeString(t time.Time) string {
	return t.UTC().Format(timeLayout)
}

func parseTime(value string) (time.Time, error) {
	return time.Parse(timeLayout, value)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func choose(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

type sqlExecutor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}
