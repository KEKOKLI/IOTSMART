package models

import "time"

type HealthStatus struct {
	Status         string     `json:"status"`
	DB             string     `json:"db"`
	MQTT           string     `json:"mqtt"`
	Workers        int        `json:"workers"`
	Uptime         string     `json:"uptime"`
	TotalDevices   int        `json:"total_devices"`
	EnabledDevices int        `json:"enabled_devices"`
	OnlineDevices  int        `json:"online_devices"`
	OfflineDevices int        `json:"offline_devices"`
	LastIngestAt   *time.Time `json:"last_ingest_at,omitempty"`
}
