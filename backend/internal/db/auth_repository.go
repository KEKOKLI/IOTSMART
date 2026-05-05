package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"iotsmart/backend/internal/models"
)

func (r *Repository) HasAuthUsers(ctx context.Context) (bool, error) {
	var count int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(1) FROM auth_users`).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *Repository) CreateAuthUser(ctx context.Context, user models.AuthUser) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO auth_users (id, username, password_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`, user.ID, user.Username, user.PasswordHash, timeString(user.CreatedAt), timeString(user.UpdatedAt))
	return err
}

func (r *Repository) GetAuthUserByUsername(ctx context.Context, username string) (models.AuthUser, error) {
	return r.getAuthUser(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM auth_users WHERE username = ?`, username)
}

func (r *Repository) GetAuthUserByID(ctx context.Context, id string) (models.AuthUser, error) {
	return r.getAuthUser(ctx, `SELECT id, username, password_hash, created_at, updated_at FROM auth_users WHERE id = ?`, id)
}

func (r *Repository) UpdateAuthUserPassword(ctx context.Context, id string, passwordHash string) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE auth_users
		SET password_hash = ?, updated_at = ?
		WHERE id = ?
	`, passwordHash, timeString(time.Now().UTC()), id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *Repository) CreateAPIToken(ctx context.Context, token models.APIToken) error {
	var expires string
	if token.ExpiresAt != nil {
		expires = timeString(*token.ExpiresAt)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO api_tokens (id, user_id, name, kind, token_hash, expires_at, created_at, last_used_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULL)
	`, token.ID, token.UserID, token.Name, token.Kind, token.TokenHash, nullIfEmpty(expires), timeString(token.CreatedAt))
	return err
}

func (r *Repository) ListAPITokens(ctx context.Context) ([]models.APIToken, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, name, kind, token_hash, expires_at, created_at, last_used_at
		FROM api_tokens
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.APIToken
	for rows.Next() {
		token, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, token)
	}
	return result, rows.Err()
}

func (r *Repository) GetAPITokenByHash(ctx context.Context, hash string) (models.APIToken, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, user_id, name, kind, token_hash, expires_at, created_at, last_used_at
		FROM api_tokens
		WHERE token_hash = ?
	`, hash)
	return scanToken(row)
}

func (r *Repository) DeleteAPIToken(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM api_tokens WHERE id = ?`, id)
	return err
}

func (r *Repository) TouchAPIToken(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE api_tokens SET last_used_at = ? WHERE id = ?`, timeString(time.Now().UTC()), id)
	return err
}

func (r *Repository) GetDevice(ctx context.Context, id string) (models.Device, error) {
	devices, err := r.ListDevices(ctx)
	if err != nil {
		return models.Device{}, err
	}
	for _, device := range devices {
		if device.ID == id {
			return device, nil
		}
	}
	return models.Device{}, sql.ErrNoRows
}

func (r *Repository) DeleteGraphPreset(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM graph_presets WHERE id = ?`, id)
	return err
}

type scanRow interface {
	Scan(dest ...any) error
}

func (r *Repository) getAuthUser(ctx context.Context, query string, arg string) (models.AuthUser, error) {
	row := r.db.QueryRowContext(ctx, query, arg)
	var user models.AuthUser
	var createdAt string
	var updatedAt string
	if err := row.Scan(&user.ID, &user.Username, &user.PasswordHash, &createdAt, &updatedAt); err != nil {
		return models.AuthUser{}, err
	}
	var err error
	user.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return models.AuthUser{}, err
	}
	user.UpdatedAt, err = parseTime(updatedAt)
	if err != nil {
		return models.AuthUser{}, err
	}
	return user, nil
}

func scanToken(row scanRow) (models.APIToken, error) {
	var token models.APIToken
	var expiresAt sql.NullString
	var createdAt string
	var lastUsedAt sql.NullString
	if err := row.Scan(&token.ID, &token.UserID, &token.Name, &token.Kind, &token.TokenHash, &expiresAt, &createdAt, &lastUsedAt); err != nil {
		return models.APIToken{}, err
	}
	var err error
	token.CreatedAt, err = parseTime(createdAt)
	if err != nil {
		return models.APIToken{}, err
	}
	if expiresAt.Valid {
		value, err := parseTime(expiresAt.String)
		if err != nil {
			return models.APIToken{}, err
		}
		token.ExpiresAt = &value
	}
	if lastUsedAt.Valid {
		value, err := parseTime(lastUsedAt.String)
		if err != nil {
			return models.APIToken{}, err
		}
		token.LastUsedAt = &value
	}
	return token, nil
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func (r *Repository) SaveGraphPresetWithValidation(ctx context.Context, preset models.GraphPreset) error {
	if preset.Name == "" || preset.DeviceID == "" || preset.Metric == "" {
		return fmt.Errorf("graph preset requires name, device_id and metric")
	}
	return r.SaveGraphPreset(ctx, preset)
}
