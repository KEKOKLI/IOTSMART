package models

import "time"

type GraphPreset struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	DeviceID  string    `json:"device_id"`
	Metric    string    `json:"metric"`
	GraphType string    `json:"graph_type"`
	TimeRange string    `json:"time_range"`
	CreatedAt time.Time `json:"created_at"`
}
