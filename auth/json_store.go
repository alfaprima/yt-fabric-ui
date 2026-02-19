package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type jsonData struct {
	Users    []User    `json:"users"`
	Sessions []Session `json:"sessions"`
}

type JSONStore struct {
	path string
	mu   sync.Mutex
}

func NewJSONStore(path string) *JSONStore {
	return &JSONStore{path: path}
}

func (s *JSONStore) load() (*jsonData, error) {
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return nil, err
	}

	if _, err := os.Stat(s.path); errors.Is(err, os.ErrNotExist) {
		return &jsonData{Users: []User{}, Sessions: []Session{}}, nil
	}

	b, err := os.ReadFile(s.path)
	if err != nil {
		return nil, err
	}
	if len(b) == 0 {
		return &jsonData{Users: []User{}, Sessions: []Session{}}, nil
	}

	var d jsonData
	if err := json.Unmarshal(b, &d); err != nil {
		return nil, err
	}
	if d.Users == nil {
		d.Users = []User{}
	}
	if d.Sessions == nil {
		d.Sessions = []Session{}
	}
	return &d, nil
}

func (s *JSONStore) save(d *jsonData) error {
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0600)
}

func (s *JSONStore) GetUserByID(id string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	for _, u := range d.Users {
		if u.ID == id {
			user := u
			return &user, nil
		}
	}
	return nil, ErrNotFound
}

func (s *JSONStore) GetUserByUsername(username string) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	for _, u := range d.Users {
		if u.Username == username {
			user := u
			return &user, nil
		}
	}
	return nil, ErrNotFound
}

func (s *JSONStore) ListUsers() ([]User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	return d.Users, nil
}

func (s *JSONStore) SaveUser(user *User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}

	updated := false
	for i := range d.Users {
		if d.Users[i].ID == user.ID {
			d.Users[i] = *user
			updated = true
			break
		}
	}
	if !updated {
		d.Users = append(d.Users, *user)
	}

	return s.save(d)
}

func (s *JSONStore) GetSession(token string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	for _, sess := range d.Sessions {
		if sess.Token == token {
			s := sess
			return &s, nil
		}
	}
	return nil, ErrNotFound
}

func (s *JSONStore) ListSessions() ([]Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return nil, err
	}
	return d.Sessions, nil
}

func (s *JSONStore) SaveSession(session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}
	updated := false
	for i := range d.Sessions {
		if d.Sessions[i].Token == session.Token {
			d.Sessions[i] = *session
			updated = true
			break
		}
	}
	if !updated {
		d.Sessions = append(d.Sessions, *session)
	}
	return s.save(d)
}

func (s *JSONStore) DeleteSession(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}
	filtered := make([]Session, 0, len(d.Sessions))
	for _, sess := range d.Sessions {
		if sess.Token != token {
			filtered = append(filtered, sess)
		}
	}
	d.Sessions = filtered
	return s.save(d)
}

func (s *JSONStore) DeleteExpiredSessions(now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, err := s.load()
	if err != nil {
		return err
	}
	filtered := make([]Session, 0, len(d.Sessions))
	for _, sess := range d.Sessions {
		if sess.ExpiresAt.After(now) {
			filtered = append(filtered, sess)
		}
	}
	d.Sessions = filtered
	return s.save(d)
}
