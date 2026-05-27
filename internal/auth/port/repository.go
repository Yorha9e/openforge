package port

import (
	"context"
)

type Project struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	GitURL string `json:"git_url"`
	Config string `json:"config"`
}

type User struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
	AvatarURL   string `json:"avatar_url"`
}

type UserRole struct {
	UserID    string   `json:"user_id"`
	ProjectID string   `json:"project_id"`
	Role      string   `json:"role"` // admin/pm/dev_lead/dev/observer
	Modules   []string `json:"modules"`
}

type AuthRepository interface {
	CreateProject(ctx context.Context, p *Project) error
	GetProject(ctx context.Context, id string) (*Project, error)
	CreateUser(ctx context.Context, u *User) error
	GetUser(ctx context.Context, id string) (*User, error)
	AssignRole(ctx context.Context, r *UserRole) error
	GetUserRole(ctx context.Context, userID, projectID string) (*UserRole, error)
	RegisterUser(ctx context.Context, id, displayName, passwordHash string) error
	GetUserPasswordHash(ctx context.Context, id string) (string, error)
	ListUserProjects(ctx context.Context, userID string) ([]*Project, error)
	UserHasProjectAccess(ctx context.Context, userID, projectID string) (bool, error)
}
