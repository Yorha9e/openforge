package domain

import "time"

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
