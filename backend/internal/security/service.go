package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"log"
	"strings"
	"time"

	"iotsmart/backend/internal/config"
	"iotsmart/backend/internal/db"
	"iotsmart/backend/internal/models"
)

type Service struct {
	cfg    config.SecurityConfig
	repo   *db.Repository
	logger *log.Logger
}

func NewService(cfg config.SecurityConfig, repo *db.Repository, logger *log.Logger) *Service {
	return &Service{
		cfg:    cfg,
		repo:   repo,
		logger: logger,
	}
}

func (s *Service) Status(ctx context.Context) (models.AuthStatus, error) {
	hasUsers, err := s.repo.HasAuthUsers(ctx)
	if err != nil {
		return models.AuthStatus{}, err
	}
	return models.AuthStatus{
		RequireLogin:  s.cfg.RequireLogin,
		SetupRequired: !hasUsers,
		TokenEnabled:  s.cfg.APITokenEnabled,
	}, nil
}

func (s *Service) Bootstrap(ctx context.Context, username, password string) (models.AuthUser, error) {
	hasUsers, err := s.repo.HasAuthUsers(ctx)
	if err != nil {
		return models.AuthUser{}, err
	}
	if hasUsers {
		return models.AuthUser{}, fmt.Errorf("admin user already exists")
	}
	username = normalizeUsername(username)
	if username == "" {
		username = "admin"
	}
	if len(password) < 8 {
		return models.AuthUser{}, fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return models.AuthUser{}, err
	}

	user := models.AuthUser{
		ID:           makeID("user"),
		Username:     username,
		PasswordHash: hash,
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.repo.CreateAuthUser(ctx, user); err != nil {
		return models.AuthUser{}, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, username, password string) (models.AuthUser, models.APIToken, string, error) {
	user, err := s.repo.GetAuthUserByUsername(ctx, normalizeUsername(username))
	if err != nil {
		return models.AuthUser{}, models.APIToken{}, "", err
	}
	if err := ComparePassword(user.PasswordHash, password); err != nil {
		return models.AuthUser{}, models.APIToken{}, "", fmt.Errorf("invalid username or password")
	}
	token, plain, err := s.issueToken(ctx, user.ID, "desktop-session", "session", time.Duration(s.cfg.SessionTimeoutMinute)*time.Minute)
	if err != nil {
		return models.AuthUser{}, models.APIToken{}, "", err
	}
	return user, token, plain, nil
}

func (s *Service) CreateAPIToken(ctx context.Context, userID, name string, ttl time.Duration) (models.APIToken, string, error) {
	if strings.TrimSpace(name) == "" {
		name = "api-token"
	}
	return s.issueToken(ctx, userID, name, "api", ttl)
}

func (s *Service) ChangePassword(ctx context.Context, userID, currentPassword, newPassword string) error {
	user, err := s.repo.GetAuthUserByID(ctx, userID)
	if err != nil {
		return err
	}
	if err := ComparePassword(user.PasswordHash, currentPassword); err != nil {
		return fmt.Errorf("current password is incorrect")
	}
	if len(newPassword) < 8 {
		return fmt.Errorf("new password must be at least 8 characters")
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	return s.repo.UpdateAuthUserPassword(ctx, userID, hash)
}

func (s *Service) ListTokens(ctx context.Context) ([]models.APIToken, error) {
	return s.repo.ListAPITokens(ctx)
}

func (s *Service) DeleteToken(ctx context.Context, id string) error {
	return s.repo.DeleteAPIToken(ctx, id)
}

func (s *Service) Authenticate(ctx context.Context, plainToken string) (models.AuthUser, models.APIToken, error) {
	if strings.TrimSpace(plainToken) == "" {
		return models.AuthUser{}, models.APIToken{}, fmt.Errorf("missing bearer token")
	}
	token, err := s.repo.GetAPITokenByHash(ctx, hashToken(plainToken))
	if err != nil {
		return models.AuthUser{}, models.APIToken{}, fmt.Errorf("invalid token")
	}
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now().UTC()) {
		return models.AuthUser{}, models.APIToken{}, fmt.Errorf("token expired")
	}
	user, err := s.repo.GetAuthUserByID(ctx, token.UserID)
	if err != nil {
		return models.AuthUser{}, models.APIToken{}, err
	}
	if err := s.repo.TouchAPIToken(ctx, token.ID); err != nil {
		s.logger.Printf("touch api token: %v", err)
	}
	return user, token, nil
}

func (s *Service) issueToken(ctx context.Context, userID, name, kind string, ttl time.Duration) (models.APIToken, string, error) {
	plain, err := randomToken()
	if err != nil {
		return models.APIToken{}, "", err
	}

	now := time.Now().UTC()
	var expiresAt *time.Time
	if ttl > 0 {
		value := now.Add(ttl)
		expiresAt = &value
	}

	token := models.APIToken{
		ID:        makeID("token"),
		UserID:    userID,
		Name:      name,
		Kind:      kind,
		TokenHash: hashToken(plain),
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	if err := s.repo.CreateAPIToken(ctx, token); err != nil {
		return models.APIToken{}, "", err
	}
	return token, plain, nil
}

func randomToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func hashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func makeID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
