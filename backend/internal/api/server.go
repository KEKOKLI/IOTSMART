package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"iotsmart/backend/internal/config"
	industrialconnector "iotsmart/backend/internal/connectors/industrial"
	modbusconnector "iotsmart/backend/internal/connectors/modbus"
	mqttconnector "iotsmart/backend/internal/connectors/mqtt"
	"iotsmart/backend/internal/db"
	"iotsmart/backend/internal/ingest"
	"iotsmart/backend/internal/logger"
	"iotsmart/backend/internal/models"
	"iotsmart/backend/internal/scheduler"
	"iotsmart/backend/internal/security"
	"iotsmart/backend/internal/websocket"
)

type authUserContextKey struct{}
type authTokenContextKey struct{}

type Server struct {
	cfg       config.Config
	repo      *db.Repository
	hub       *websocket.Hub
	database  *sql.DB
	logger    *logger.Logger
	simulator *scheduler.Simulator
	mqtt      *mqttconnector.Connector
	ingestor  *ingest.Service
	auth      *security.Service
	startedAt time.Time
}

func NewServer(
	cfg config.Config,
	repo *db.Repository,
	hub *websocket.Hub,
	database *sql.DB,
	appLogger *logger.Logger,
	simulator *scheduler.Simulator,
	mqtt *mqttconnector.Connector,
	ingestor *ingest.Service,
	auth *security.Service,
) *Server {
	return &Server{
		cfg:       cfg,
		repo:      repo,
		hub:       hub,
		database:  database,
		logger:    appLogger,
		simulator: simulator,
		mqtt:      mqtt,
		ingestor:  ingestor,
		auth:      auth,
		startedAt: time.Now(),
	}
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/health", s.handleHealth)

	mux.HandleFunc("/api/v1/auth/status", s.handleAuthStatus)
	mux.HandleFunc("/api/v1/auth/bootstrap", s.handleAuthBootstrap)
	mux.HandleFunc("/api/v1/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/v1/auth/logout", s.withAuth(s.handleAuthLogout))
	mux.HandleFunc("/api/v1/auth/change-password", s.withAuth(s.handleAuthChangePassword))
	mux.HandleFunc("/api/v1/auth/tokens", s.withAuth(s.handleAuthTokens))
	mux.HandleFunc("/api/v1/auth/tokens/{id}", s.withAuth(s.handleAuthTokenByID))

	mux.HandleFunc("/api/v1/devices", s.withAuth(s.handleDevices))
	mux.HandleFunc("/api/v1/devices/{id}", s.withAuth(s.handleDeviceByID))

	mux.HandleFunc("/api/v1/ingest", s.withAuth(s.handleIngest))
	mux.HandleFunc("/api/v1/telemetry/latest", s.withAuth(s.handleLatestTelemetry))
	mux.HandleFunc("/api/v1/telemetry/query", s.withAuth(s.handleQueryTelemetry))
	mux.HandleFunc("/api/v1/telemetry/export", s.withAuth(s.handleExportTelemetryCSV))

	mux.HandleFunc("/api/v1/graphs", s.withAuth(s.handleGraphs))
	mux.HandleFunc("/api/v1/graphs/{id}", s.withAuth(s.handleGraphByID))

	mux.HandleFunc("/api/v1/protocols/status", s.withAuth(s.handleProtocolStatus))
	mux.HandleFunc("/api/v1/protocols/mqtt/test", s.withAuth(s.handleMQTTConnectionTest))
	mux.HandleFunc("/api/v1/protocols/mqtt/publish", s.withAuth(s.handleMQTTPublishTest))
	mux.HandleFunc("/api/v1/protocols/modbus/test", s.withAuth(s.handleModbusTCPTest))
	mux.HandleFunc("/api/v1/protocols/modbus/read", s.withAuth(s.handleModbusReadRegisters))
	mux.HandleFunc("/api/v1/protocols/modbus/rtu/test", s.withAuth(s.handleModbusRTUTest))
	mux.HandleFunc("/api/v1/protocols/modbus/rtu/read", s.withAuth(s.handleModbusRTUReadRegisters))
	mux.HandleFunc("/api/v1/protocols/opcua/test", s.withAuth(s.handleOPCUATest))
	mux.HandleFunc("/api/v1/protocols/opcua/browse", s.withAuth(s.handleOPCUABrowse))
	mux.HandleFunc("/api/v1/protocols/opcua/read", s.withAuth(s.handleOPCUARead))
	mux.HandleFunc("/api/v1/protocols/bacnet/test", s.withAuth(s.handleBACnetTest))
	mux.HandleFunc("/api/v1/protocols/bacnet/read-property", s.withAuth(s.handleBACnetReadProperty))
	mux.HandleFunc("/api/v1/protocols/ads/test", s.withAuth(s.handleADSTest))
	mux.HandleFunc("/api/v1/protocols/ads/read-symbol", s.withAuth(s.handleADSReadSymbol))
	mux.HandleFunc("/api/v1/protocols/odbc/test", s.withAuth(s.handleODBCTest))
	mux.HandleFunc("/api/v1/protocols/odbc/query", s.withAuth(s.handleODBCQuery))
	mux.HandleFunc("/api/v1/logs", s.withAuth(s.handleLogs))
	mux.HandleFunc("/ws/live", s.withAuth(s.handleLiveWebsocket))

	return s.withLogging(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	dbStatus := "ok"
	if err := s.database.PingContext(r.Context()); err != nil {
		dbStatus = "error"
	}

	summary, err := s.repo.GetHealthSummary(r.Context(), 2*time.Duration(s.cfg.Telemetry.DefaultIntervalSeconds)*time.Second)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	mqttStatus := "disabled"
	if s.mqtt != nil {
		snapshot := s.mqtt.Snapshot()
		if snapshot.Enabled {
			if snapshot.Connected {
				mqttStatus = "connected"
			} else {
				mqttStatus = "disconnected"
			}
		}
	}

	status := models.HealthStatus{
		Status:         "ok",
		DB:             dbStatus,
		MQTT:           mqttStatus,
		Workers:        s.simulator.WorkerCount() + workerCount(s.mqtt),
		Uptime:         time.Since(s.startedAt).Round(time.Second).String(),
		TotalDevices:   summary.TotalDevices,
		EnabledDevices: summary.EnabledDevices,
		OnlineDevices:  summary.OnlineDevices,
		OfflineDevices: summary.OfflineDevices,
		LastIngestAt:   summary.LastIngestAt,
	}

	if dbStatus != "ok" {
		status.Status = "degraded"
	}
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	status, err := s.auth.Status(r.Context())
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, status)
}

func (s *Server) handleAuthBootstrap(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	user, err := s.auth.Bootstrap(r.Context(), payload.Username, payload.Password)
	if err != nil {
		s.writeError(w, http.StatusConflict, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, map[string]any{
		"user": user,
	})
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var payload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}

	user, token, plain, err := s.auth.Login(r.Context(), payload.Username, payload.Password)
	if err != nil {
		s.writeError(w, http.StatusUnauthorized, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"access_token": plain,
		"token_id":     token.ID,
		"expires_at":   token.ExpiresAt,
	})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	token, ok := s.currentToken(r.Context())
	if ok && token.ID != "" {
		if err := s.auth.DeleteToken(r.Context(), token.ID); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAuthChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	user, ok := s.currentUser(r.Context())
	if !ok {
		s.writeError(w, http.StatusUnauthorized, fmt.Errorf("login is required to change password"))
		return
	}

	var payload struct {
		CurrentPassword string `json:"current_password"`
		OldPassword     string `json:"old_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	currentPassword := payload.CurrentPassword
	if currentPassword == "" {
		currentPassword = payload.OldPassword
	}
	if err := s.auth.ChangePassword(r.Context(), user.ID, currentPassword, payload.NewPassword); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleAuthTokens(w http.ResponseWriter, r *http.Request) {
	user, ok := s.currentUser(r.Context())
	if !ok {
		s.writeError(w, http.StatusUnauthorized, fmt.Errorf("login is required for token management"))
		return
	}
	switch r.Method {
	case http.MethodGet:
		tokens, err := s.auth.ListTokens(r.Context())
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, tokens)
	case http.MethodPost:
		var payload struct {
			Name string `json:"name"`
			Days int    `json:"days"`
		}
		if err := s.decodeJSON(r, &payload); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		if payload.Days <= 0 {
			payload.Days = 365
		}
		token, plain, err := s.auth.CreateAPIToken(r.Context(), user.ID, payload.Name, time.Duration(payload.Days)*24*time.Hour)
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, map[string]any{
			"token":        token,
			"access_token": plain,
		})
	default:
		s.writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleAuthTokenByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		s.writeMethodNotAllowed(w, http.MethodDelete)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("missing token id"))
		return
	}
	if err := s.auth.DeleteToken(r.Context(), id); err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDevices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		devices, err := s.repo.ListDevices(r.Context())
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, devices)
	case http.MethodPost:
		var device models.Device
		if err := s.decodeJSON(r, &device); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		if !device.Enabled && !device.Simulated {
			device.Enabled = true
		}
		if err := s.repo.SaveDevice(r.Context(), device); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, device)
	default:
		s.writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleDeviceByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		s.writeError(w, http.StatusBadRequest, errors.New("missing device id"))
		return
	}

	switch r.Method {
	case http.MethodGet:
		device, err := s.repo.GetDevice(r.Context(), id)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				s.writeError(w, http.StatusNotFound, err)
				return
			}
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, device)
	case http.MethodPut:
		var device models.Device
		if err := s.decodeJSON(r, &device); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		device.ID = id
		if err := s.repo.SaveDevice(r.Context(), device); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		s.writeJSON(w, http.StatusOK, device)
	case http.MethodDelete:
		if err := s.repo.DeleteDevice(r.Context(), id); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		s.writeMethodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var reading models.TelemetryRecord
	if err := s.decodeJSON(r, &reading); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.ingestor.ProcessReading(r.Context(), reading, "http_ingest"); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	s.writeJSON(w, http.StatusAccepted, map[string]string{"status": "accepted"})
}

func (s *Server) handleLatestTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	limit := parseIntWithDefault(r.URL.Query().Get("limit"), 50)
	records, err := s.repo.ListLatestTelemetry(r.Context(), limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleQueryTelemetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	filter, err := s.telemetryFilterFromRequest(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	records, err := s.repo.QueryTelemetry(r.Context(), filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, records)
}

func (s *Server) handleExportTelemetryCSV(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	filter, err := s.telemetryFilterFromRequest(r)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	filter.Limit = parseIntWithDefault(r.URL.Query().Get("limit"), 5000)

	records, err := s.repo.QueryTelemetry(r.Context(), filter)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}

	filename := fmt.Sprintf("telemetry_%s.csv", time.Now().UTC().Format("20060102_150405"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	defer writer.Flush()

	_ = writer.Write([]string{"device_id", "metric", "value", "unit", "quality", "timestamp"})
	for _, record := range records {
		_ = writer.Write([]string{
			record.DeviceID,
			record.Metric,
			strconv.FormatFloat(record.Value, 'f', -1, 64),
			record.Unit,
			record.Quality,
			record.Timestamp.Format(time.RFC3339),
		})
	}
}

func (s *Server) handleGraphs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		presets, err := s.repo.ListGraphPresets(r.Context())
		if err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		s.writeJSON(w, http.StatusOK, presets)
	case http.MethodPost:
		var preset models.GraphPreset
		if err := s.decodeJSON(r, &preset); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		if preset.GraphType == "" {
			preset.GraphType = "line"
		}
		if preset.TimeRange == "" {
			preset.TimeRange = "1h"
		}
		if err := s.repo.SaveGraphPresetWithValidation(r.Context(), preset); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		s.writeJSON(w, http.StatusCreated, preset)
	default:
		s.writeMethodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (s *Server) handleGraphByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		s.writeError(w, http.StatusBadRequest, fmt.Errorf("missing graph preset id"))
		return
	}
	switch r.Method {
	case http.MethodPut:
		var preset models.GraphPreset
		if err := s.decodeJSON(r, &preset); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		preset.ID = id
		if err := s.repo.SaveGraphPresetWithValidation(r.Context(), preset); err != nil {
			s.writeError(w, http.StatusBadRequest, err)
			return
		}
		s.writeJSON(w, http.StatusOK, preset)
	case http.MethodDelete:
		if err := s.repo.DeleteGraphPreset(r.Context(), id); err != nil {
			s.writeError(w, http.StatusInternalServerError, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		s.writeMethodNotAllowed(w, http.MethodPut, http.MethodDelete)
	}
}

func (s *Server) handleProtocolStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	mqttStatus := map[string]any{
		"enabled": false,
		"status":  "disabled",
	}
	if s.mqtt != nil {
		snapshot := s.mqtt.Snapshot()
		mqttStatus = map[string]any{
			"enabled":           snapshot.Enabled,
			"status":            statusText(snapshot),
			"broker":            s.cfg.MQTT.Broker,
			"subscription":      snapshot.Subscription,
			"messages_received": snapshot.MessagesReceived,
			"last_message_at":   snapshot.LastMessageAt,
			"last_error":        snapshot.LastError,
		}
	}

	payload := map[string]any{
		"mqtt": mqttStatus,
		"modbus": map[string]any{
			"enabled": s.cfg.Modbus.Enabled,
			"status":  protocolState(s.cfg.Modbus.Enabled),
			"timeout": s.cfg.Modbus.TimeoutMS,
		},
		"modbus_rtu": map[string]any{
			"enabled": false,
			"status":  "web_api_available",
			"extra":   "serial_probe_read_registers",
		},
		"opcua": map[string]any{
			"enabled": false,
			"status":  "web_api_available",
			"extra":   "browse_read_node",
		},
		"bacnet": map[string]any{
			"enabled": false,
			"status":  "web_api_available",
			"extra":   "who_is_read_property",
		},
		"ads": map[string]any{
			"enabled": false,
			"status":  "web_api_available",
			"extra":   "twincat_read_symbol",
		},
		"odbc": map[string]any{
			"enabled": false,
			"status":  "web_api_available",
			"extra":   "live_read_query",
		},
		"zigbee": map[string]any{
			"enabled": false,
			"status":  "mqtt_bridge_pending",
		},
		"lora": map[string]any{
			"enabled": false,
			"status":  "mqtt_bridge_pending",
		},
		"bluetooth": map[string]any{
			"enabled": false,
			"status":  "worker_pending",
		},
		"simulator": map[string]any{
			"enabled": s.cfg.Simulator.Enabled,
			"status":  protocolState(s.cfg.Simulator.Enabled),
		},
	}

	s.writeJSON(w, http.StatusOK, payload)
}

func (s *Server) handleMQTTConnectionTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if s.mqtt == nil {
		s.writeError(w, http.StatusServiceUnavailable, fmt.Errorf("mqtt connector is not configured"))
		return
	}
	snapshot, err := s.mqtt.TestConnection(r.Context())
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":       true,
		"broker":   s.cfg.MQTT.Broker,
		"status":   "connected",
		"snapshot": snapshot,
	})
}

func (s *Server) handleMQTTPublishTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	if s.mqtt == nil {
		s.writeError(w, http.StatusServiceUnavailable, fmt.Errorf("mqtt connector is not configured"))
		return
	}

	var payload struct {
		Topic   string          `json:"topic"`
		Payload json.RawMessage `json:"payload"`
	}
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	if len(payload.Payload) == 0 {
		payload.Payload = json.RawMessage(`{"device_id":"mqtt_test_001","metric":"temperature","value":24.8,"unit":"C"}`)
	}
	if err := s.mqtt.PublishTest(r.Context(), payload.Topic, payload.Payload); err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"topic": strings.TrimSpace(payload.Topic),
	})
}

func (s *Server) handleModbusTCPTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var payload modbusconnector.TCPRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modbusconnector.TestTCPConnection(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleModbusReadRegisters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}

	var payload modbusconnector.ReadRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := modbusconnector.ReadRegisters(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleModbusRTUTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.ModbusRTUProbeRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.TestModbusRTU(payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleModbusRTUReadRegisters(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.ModbusRTUReadRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.ReadModbusRTURegisters(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOPCUATest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.OPCUAProbeRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.TestOPCUA(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOPCUABrowse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.OPCUABrowseRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.BrowseOPCUA(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleOPCUARead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.OPCUAReadRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.ReadOPCUANode(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBACnetTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.BACnetProbeRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.TestBACnet(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBACnetReadProperty(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.BACnetReadPropertyRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.ReadBACnetProperty(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleADSTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.TCPProbeRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.TestADS(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleADSReadSymbol(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.ADSReadSymbolRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.ReadADSSymbol(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleODBCTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.ODBCProbeRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.TestODBC(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleODBCQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeMethodNotAllowed(w, http.MethodPost)
		return
	}
	var payload industrialconnector.ODBCQueryRequest
	if err := s.decodeJSON(r, &payload); err != nil {
		s.writeError(w, http.StatusBadRequest, err)
		return
	}
	result, err := industrialconnector.QueryODBC(r.Context(), payload)
	if err != nil {
		s.writeError(w, http.StatusBadGateway, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}

	limit := parseIntWithDefault(r.URL.Query().Get("limit"), 200)
	content, err := tailLines(s.logger.Path(), limit)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err)
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]any{
		"path":  s.logger.Path(),
		"lines": content,
	})
}

func (s *Server) handleLiveWebsocket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	s.hub.Handle(w, r)
}

func (s *Server) withAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if !s.cfg.Security.RequireLogin {
				next(w, r)
				return
			}
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
			return
		}
		user, apiToken, err := s.auth.Authenticate(r.Context(), token)
		if err != nil {
			s.writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
			return
		}
		ctx := context.WithValue(r.Context(), authUserContextKey{}, user)
		ctx = context.WithValue(ctx, authTokenContextKey{}, apiToken)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) currentUser(ctx context.Context) (models.AuthUser, bool) {
	user, ok := ctx.Value(authUserContextKey{}).(models.AuthUser)
	return user, ok
}

func (s *Server) currentToken(ctx context.Context) (models.APIToken, bool) {
	token, ok := ctx.Value(authTokenContextKey{}).(models.APIToken)
	return token, ok
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Printf("write json response: %v", err)
	}
}

func (s *Server) decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (s *Server) writeError(w http.ResponseWriter, status int, err error) {
	s.logger.Printf("api error (%d): %v", status, err)
	s.writeJSON(w, status, map[string]string{
		"error": err.Error(),
	})
}

func (s *Server) writeMethodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	s.writeError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.logger.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func (s *Server) telemetryFilterFromRequest(r *http.Request) (models.TelemetryQueryFilter, error) {
	filter := models.TelemetryQueryFilter{
		DeviceID: r.URL.Query().Get("device_id"),
		Metric:   r.URL.Query().Get("metric"),
		Limit:    parseIntWithDefault(r.URL.Query().Get("limit"), 500),
	}

	if from := strings.TrimSpace(r.URL.Query().Get("from")); from != "" {
		parsed, err := time.Parse(time.RFC3339, from)
		if err != nil {
			return filter, fmt.Errorf("invalid from timestamp: %w", err)
		}
		filter.From = &parsed
	}
	if to := strings.TrimSpace(r.URL.Query().Get("to")); to != "" {
		parsed, err := time.Parse(time.RFC3339, to)
		if err != nil {
			return filter, fmt.Errorf("invalid to timestamp: %w", err)
		}
		filter.To = &parsed
	}
	return filter, nil
}

func bearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return strings.TrimSpace(r.Header.Get("X-API-Token"))
}

func protocolState(enabled bool) string {
	if enabled {
		return "enabled"
	}
	return "disabled"
}

func statusText(status mqttconnector.Status) string {
	if !status.Enabled {
		return "disabled"
	}
	if status.Connected {
		return "connected"
	}
	if status.LastError != "" {
		return "error"
	}
	return "disconnected"
}

func parseIntWithDefault(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func tailLines(path string, limit int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	if limit > len(lines) {
		limit = len(lines)
	}
	if limit < 0 {
		limit = 0
	}
	start := len(lines) - limit
	if start < 0 {
		start = 0
	}
	result := make([]string, 0, limit)
	for _, line := range lines[start:] {
		if strings.TrimSpace(line) != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

func workerCount(connector *mqttconnector.Connector) int {
	if connector == nil {
		return 0
	}
	return connector.WorkerCount()
}
