package models

import "time"

type TelemetryRecord struct {
	ID        int64     `json:"id,omitempty"`
	DeviceID  string    `json:"device_id"`
	Name      string    `json:"name,omitempty"`
	Protocol  string    `json:"protocol,omitempty"`
	Metric    string    `json:"metric"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit,omitempty"`
	Quality   string    `json:"quality,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type TelemetryQueryFilter struct {
	DeviceID string
	Metric   string
	From     *time.Time
	To       *time.Time
	Limit    int
}
