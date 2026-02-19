package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

const SessionCookieName = "ytf_session"

type Service struct {
	store           Store
	sessionDuration time.Duration
}

func NewService(store Store) *Service {
	return &Service{store: store, sessionDuration: 8 * time.Hour}
}

func (s *Service) SetSessionDuration(d time.Duration) {
	s.sessionDuration = d
}

func (s *Service) BootstrapAdmin(username, password string) error {
	username = strings.TrimSpace(username)
	password = strings.TrimSpace(password)
	if username == "" || password == "" {
		return nil
	}

	if _, err := s.store.GetUserByUsername(username); err == nil {
		return nil
	}

	users, err := s.store.ListUsers()
	if err != nil {
		return err
	}
	for _, u := range users {
		if u.Role == RoleAdmin {
			return nil
		}
	}

	return s.CreateUser(username, password, RoleAdmin)
}

func (s *Service) CreateUser(username, password string, role Role) error {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return errors.New("username and password are required")
	}
	if !validRole(role) {
		return errors.New("invalid role")
	}
	if _, err := s.store.GetUserByUsername(username); err == nil {
		return errors.New("username already exists")
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	id, err := randomHex(16)
	if err != nil {
		return err
	}

	user := &User{
		ID:           id,
		Username:     username,
		PasswordHash: hash,
		Role:         role,
		Enabled:      true,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	return s.store.SaveUser(user)
}

func (s *Service) ResetPassword(userID, newPassword string) error {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return err
	}
	hash, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	user.PasswordHash = hash
	user.UpdatedAt = time.Now().UTC()
	return s.store.SaveUser(user)
}

func (s *Service) SetUserRole(userID string, role Role) error {
	if !validRole(role) {
		return errors.New("invalid role")
	}
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return err
	}
	user.Role = role
	user.UpdatedAt = time.Now().UTC()
	return s.store.SaveUser(user)
}

func (s *Service) SetUserEnabled(userID string, enabled bool) error {
	user, err := s.store.GetUserByID(userID)
	if err != nil {
		return err
	}
	user.Enabled = enabled
	user.UpdatedAt = time.Now().UTC()
	return s.store.SaveUser(user)
}

func (s *Service) EnsureAtLeastOneAdmin() error {
	users, err := s.store.ListUsers()
	if err != nil {
		return err
	}
	for _, user := range users {
		if user.Role == RoleAdmin && user.Enabled {
			return nil
		}
	}
	return errors.New("at least one enabled admin is required")
}

func (s *Service) ListUsers() ([]User, error) {
	return s.store.ListUsers()
}

func (s *Service) GetUserByID(userID string) (*User, error) {
	return s.store.GetUserByID(userID)
}

func (s *Service) Authenticate(username, password string) (*User, error) {
	user, err := s.store.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return nil, errors.New("invalid credentials")
	}
	if !user.Enabled {
		return nil, errors.New("account disabled")
	}
	if err := VerifyPassword(user.PasswordHash, password); err != nil {
		return nil, errors.New("invalid credentials")
	}
	user.LastLoginAt = time.Now().UTC()
	user.UpdatedAt = time.Now().UTC()
	if err := s.store.SaveUser(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) CreateSession(userID string) (*Session, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	sess := &Session{
		Token:      token,
		UserID:     userID,
		CreatedAt:  now,
		LastSeenAt: now,
		ExpiresAt:  now.Add(s.sessionDuration),
	}
	if err := s.store.SaveSession(sess); err != nil {
		return nil, err
	}
	return sess, nil
}

func (s *Service) DeleteSession(token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSession(token)
}

func (s *Service) ValidateSession(token string) (*User, error) {
	if token == "" {
		return nil, errors.New("missing session token")
	}
	if err := s.store.DeleteExpiredSessions(time.Now().UTC()); err != nil {
		return nil, err
	}

	sess, err := s.store.GetSession(token)
	if err != nil {
		return nil, err
	}
	if sess.ExpiresAt.Before(time.Now().UTC()) {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("session expired")
	}

	user, err := s.store.GetUserByID(sess.UserID)
	if err != nil {
		return nil, err
	}
	if !user.Enabled {
		_ = s.store.DeleteSession(token)
		return nil, errors.New("account disabled")
	}

	sess.LastSeenAt = time.Now().UTC()
	sess.ExpiresAt = time.Now().UTC().Add(s.sessionDuration)
	if err := s.store.SaveSession(sess); err != nil {
		return nil, err
	}

	return user, nil
}

func validRole(role Role) bool {
	return role == RoleViewer || role == RoleOperator || role == RoleAdmin
}

func randomHex(size int) (string, error) {
	b := make([]byte, size)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
