package models

import "time"

type AuthUser struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type APIToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	Name       string     `json:"name"`
	Kind       string     `json:"kind"`
	TokenHash  string     `json:"-"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

type AuthStatus struct {
	RequireLogin  bool `json:"require_login"`
	SetupRequired bool `json:"setup_required"`
	TokenEnabled  bool `json:"token_enabled"`
}
