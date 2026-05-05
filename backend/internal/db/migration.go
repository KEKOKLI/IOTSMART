package db

import "database/sql"

func Migrate(database *sql.DB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS projects (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL,
			name TEXT NOT NULL,
			protocol TEXT NOT NULL,
			location TEXT,
			enabled INTEGER NOT NULL DEFAULT 1,
			simulated INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS sensor_metrics (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL,
			metric TEXT NOT NULL,
			unit TEXT,
			data_type TEXT NOT NULL DEFAULT 'float',
			min_value REAL,
			max_value REAL,
			created_at TEXT NOT NULL
		);`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_sensor_metrics_device_metric
		ON sensor_metrics(device_id, metric);`,
		`CREATE TABLE IF NOT EXISTS telemetry (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			device_id TEXT NOT NULL,
			metric TEXT NOT NULL,
			value REAL NOT NULL,
			unit TEXT,
			quality TEXT DEFAULT 'good',
			ts TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_telemetry_device_metric_ts
		ON telemetry(device_id, metric, ts);`,
		`CREATE TABLE IF NOT EXISTS graph_presets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			device_id TEXT NOT NULL,
			metric TEXT NOT NULL,
			graph_type TEXT NOT NULL,
			time_range TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS auth_users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS api_tokens (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			kind TEXT NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TEXT,
			created_at TEXT NOT NULL,
			last_used_at TEXT
		);`,
		`CREATE INDEX IF NOT EXISTS idx_api_tokens_user_id
		ON api_tokens(user_id);`,
	}

	for _, stmt := range statements {
		if _, err := database.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}
