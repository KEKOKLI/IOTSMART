package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	App       AppConfig       `yaml:"app"`
	Database  DatabaseConfig  `yaml:"database"`
	Security  SecurityConfig  `yaml:"security"`
	MQTT      MQTTConfig      `yaml:"mqtt"`
	Modbus    ModbusConfig    `yaml:"modbus"`
	Telemetry TelemetryConfig `yaml:"telemetry"`
	Logging   LoggingConfig   `yaml:"logging"`
	Simulator SimulatorConfig `yaml:"simulator"`
}

type AppConfig struct {
	Name     string `yaml:"name"`
	BindHost string `yaml:"bind_host"`
	Port     int    `yaml:"port"`
}

type DatabaseConfig struct {
	Type        string `yaml:"type"`
	Path        string `yaml:"path"`
	AutoMigrate bool   `yaml:"auto_migrate"`
}

type SecurityConfig struct {
	RequireLogin         bool `yaml:"require_login"`
	SessionTimeoutMinute int  `yaml:"session_timeout_minutes"`
	APITokenEnabled      bool `yaml:"api_token_enabled"`
}

type MQTTConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Broker      string `yaml:"broker"`
	Username    string `yaml:"username"`
	Password    string `yaml:"password"`
	TopicPrefix string `yaml:"topic_prefix"`
	TLS         bool   `yaml:"tls"`
}

type ModbusConfig struct {
	Enabled             bool `yaml:"enabled"`
	PollIntervalSeconds int  `yaml:"poll_interval_seconds"`
	TimeoutMS           int  `yaml:"timeout_ms"`
}

type TelemetryConfig struct {
	DefaultIntervalSeconds int `yaml:"default_interval_seconds"`
	RetentionDaysRaw       int `yaml:"retention_days_raw"`
	RetentionDaysAggregate int `yaml:"retention_days_aggregate"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
	Path  string `yaml:"path"`
}

type SimulatorConfig struct {
	Enabled         bool   `yaml:"enabled"`
	ProjectID       string `yaml:"project_id"`
	ProjectName     string `yaml:"project_name"`
	DeviceCount     int    `yaml:"device_count"`
	IntervalSeconds int    `yaml:"interval_seconds"`
}

func Default() Config {
	return Config{
		App: AppConfig{
			Name:     "Small IoT Control Center",
			BindHost: "127.0.0.1",
			Port:     18080,
		},
		Database: DatabaseConfig{
			Type:        "sqlite",
			Path:        filepath.Join("data", "iot_app.db"),
			AutoMigrate: true,
		},
		Security: SecurityConfig{
			RequireLogin:         true,
			SessionTimeoutMinute: 60,
			APITokenEnabled:      true,
		},
		MQTT: MQTTConfig{
			Enabled:     false,
			Broker:      "tcp://127.0.0.1:1883",
			TopicPrefix: "iot/#",
		},
		Modbus: ModbusConfig{
			Enabled:             false,
			PollIntervalSeconds: 30,
			TimeoutMS:           3000,
		},
		Telemetry: TelemetryConfig{
			DefaultIntervalSeconds: 30,
			RetentionDaysRaw:       90,
			RetentionDaysAggregate: 730,
		},
		Logging: LoggingConfig{
			Level: "info",
			Path:  filepath.Join("logs", "app.log"),
		},
		Simulator: SimulatorConfig{
			Enabled:         true,
			ProjectID:       "demo-project",
			ProjectName:     "Demo Project",
			DeviceCount:     20,
			IntervalSeconds: 30,
		},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg.normalize(), nil
		}
		return cfg, fmt.Errorf("read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse yaml: %w", err)
	}

	return cfg.normalize(), nil
}

func (c Config) normalize() Config {
	if c.App.Name == "" {
		c.App.Name = "Small IoT Control Center"
	}
	if c.App.BindHost == "" {
		c.App.BindHost = "127.0.0.1"
	}
	if c.App.Port == 0 {
		c.App.Port = 18080
	}
	if c.Database.Type == "" {
		c.Database.Type = "sqlite"
	}
	if c.Database.Path == "" {
		c.Database.Path = filepath.Join("data", "iot_app.db")
	}
	if c.Logging.Path == "" {
		c.Logging.Path = filepath.Join("logs", "app.log")
	}
	if c.Telemetry.DefaultIntervalSeconds <= 0 {
		c.Telemetry.DefaultIntervalSeconds = 30
	}
	if c.Simulator.ProjectID == "" {
		c.Simulator.ProjectID = "demo-project"
	}
	if c.Simulator.ProjectName == "" {
		c.Simulator.ProjectName = "Demo Project"
	}
	if c.Simulator.DeviceCount <= 0 {
		c.Simulator.DeviceCount = 20
	}
	if c.Simulator.IntervalSeconds <= 0 {
		c.Simulator.IntervalSeconds = c.Telemetry.DefaultIntervalSeconds
	}

	c.Database.Path = filepath.Clean(c.Database.Path)
	c.Logging.Path = filepath.Clean(c.Logging.Path)
	return c
}

func (a AppConfig) PortString() string {
	return fmt.Sprintf("%d", a.Port)
}
