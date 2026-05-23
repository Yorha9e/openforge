package domain

import "time"

// Context keys shared across auth and server packages.
const (
	UserIDContextKey    = "user_id"
	UserRoleContextKey  = "user_role"
	ProjectIDContextKey = "project_id"
)

type User struct {
	ID          string
	DisplayName string
	AvatarURL   string
	DisabledAt  *time.Time
	CreatedAt   time.Time
}

type Role struct {
	ID        string
	UserID    string
	ProjectID string
	Role      string
	Modules   []string
}
