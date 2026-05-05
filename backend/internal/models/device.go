package models

import "time"

type Device struct {
	ID        string         `json:"id"`
	ProjectID string         `json:"project_id"`
	Name      string         `json:"name"`
	Protocol  string         `json:"protocol"`
	Location  string         `json:"location,omitempty"`
	Enabled   bool           `json:"enabled"`
	Simulated bool           `json:"simulated"`
	CreatedAt time.Time      `json:"created_at"`
	Metrics   []SensorMetric `json:"metrics,omitempty"`
}

type SensorMetric struct {
	ID        string    `json:"id"`
	DeviceID  string    `json:"device_id"`
	Metric    string    `json:"metric"`
	Unit      string    `json:"unit,omitempty"`
	DataType  string    `json:"data_type"`
	MinValue  *float64  `json:"min_value,omitempty"`
	MaxValue  *float64  `json:"max_value,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
