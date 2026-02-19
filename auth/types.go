package auth

import "time"

type Role string

const (
	RoleViewer  Role = "viewer"
	RoleOperator     = "operator"
	RoleAdmin        = "admin"
)

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	Role         Role      `json:"role"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	LastLoginAt  time.Time `json:"last_login_at,omitempty"`
}

type Session struct {
	Token      string    `json:"token"`
	UserID     string    `json:"user_id"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func HasRequiredRole(userRole Role, required Role) bool {
	if userRole == required {
		return true
	}

	rank := map[Role]int{
		RoleViewer:  1,
		RoleOperator: 2,
		RoleAdmin:   3,
	}

	return rank[userRole] >= rank[required]
}
