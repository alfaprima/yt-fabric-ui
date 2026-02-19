package auth

import (
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store interface {
	GetUserByID(id string) (*User, error)
	GetUserByUsername(username string) (*User, error)
	ListUsers() ([]User, error)
	SaveUser(user *User) error

	GetSession(token string) (*Session, error)
	ListSessions() ([]Session, error)
	SaveSession(session *Session) error
	DeleteSession(token string) error
	DeleteExpiredSessions(now time.Time) error
}
