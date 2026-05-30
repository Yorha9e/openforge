package adapter

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	port "openforge/internal/auth/port"
)

type PGAuthRepository struct {
	db *sql.DB
}

func NewPGAuthRepository(db *sql.DB) *PGAuthRepository {
	return &PGAuthRepository{db: db}
}

func (r *PGAuthRepository) CreateProject(ctx context.Context, p *port.Project) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO project (id, name, git_url, config) VALUES ($1, $2, $3, $4)
	`, p.ID, p.Name, p.GitURL, p.Config)
	return err
}

func (r *PGAuthRepository) GetProject(ctx context.Context, id string) (*port.Project, error) {
	var p port.Project
	err := r.db.QueryRowContext(ctx, `SELECT id, name, git_url, config FROM project WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&p.ID, &p.Name, &p.GitURL, &p.Config)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &p, err
}

func (r *PGAuthRepository) CreateUser(ctx context.Context, u *port.User) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO "user" (id, display_name, avatar_url) VALUES ($1, $2, $3)
		ON CONFLICT (id) DO UPDATE SET display_name = EXCLUDED.display_name, avatar_url = EXCLUDED.avatar_url
	`, u.ID, u.DisplayName, u.AvatarURL)
	return err
}

// RegisterUser creates a new user with a bcrypt password hash.
func (r *PGAuthRepository) RegisterUser(ctx context.Context, id, displayName, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO "user" (id, display_name, password_hash, created_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (id) DO NOTHING
	`, id, displayName, passwordHash)
	return err
}

// GetUserPasswordHash retrieves the stored bcrypt hash for a given user ID.
func (r *PGAuthRepository) GetUserPasswordHash(ctx context.Context, id string) (string, error) {
	var hash string
	err := r.db.QueryRowContext(ctx,
		`SELECT password_hash FROM "user" WHERE id = $1 AND disabled_at IS NULL`, id,
	).Scan(&hash)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return hash, err
}

// ListUserProjects returns all projects a user has an active role assignment in.
func (r *PGAuthRepository) ListUserProjects(ctx context.Context, userID string) ([]*port.Project, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT p.id, p.name, p.git_url, p.config
		FROM project p
		INNER JOIN user_role ur ON p.id = ur.project_id
		WHERE ur.user_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var projects []*port.Project
	for rows.Next() {
		var p port.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.GitURL, &p.Config); err != nil {
			return nil, err
		}
		projects = append(projects, &p)
	}
	return projects, nil
}

// UserHasProjectAccess checks whether a user has an active role in a project.
func (r *PGAuthRepository) UserHasProjectAccess(ctx context.Context, userID, projectID string) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM user_role WHERE user_id = $1 AND project_id = $2)`,
		userID, projectID,
	).Scan(&exists)
	return exists, err
}

func (r *PGAuthRepository) GetUser(ctx context.Context, id string) (*port.User, error) {
	var u port.User
	err := r.db.QueryRowContext(ctx, `SELECT id, display_name, avatar_url FROM "user" WHERE id = $1 AND disabled_at IS NULL`, id).
		Scan(&u.ID, &u.DisplayName, &u.AvatarURL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &u, err
}

func (r *PGAuthRepository) AssignRole(ctx context.Context, ur *port.UserRole) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO user_role (user_id, project_id, role, modules) VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, project_id) DO UPDATE SET role = EXCLUDED.role, modules = EXCLUDED.modules
	`, ur.UserID, ur.ProjectID, ur.Role, pq.Array(ur.Modules))
	return err
}

func (r *PGAuthRepository) GetUserRole(ctx context.Context, userID, projectID string) (*port.UserRole, error) {
	var ur port.UserRole
	err := r.db.QueryRowContext(ctx, `SELECT user_id, project_id, role, modules FROM user_role WHERE user_id = $1 AND project_id = $2`, userID, projectID).
		Scan(&ur.UserID, &ur.ProjectID, &ur.Role, pq.Array(&ur.Modules))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &ur, err
}

// CreateInvitation creates a new invitation in the database
func (r *PGAuthRepository) CreateInvitation(ctx context.Context, inv *port.Invitation) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO invitation (token, role, project_id, created_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, inv.Token, inv.Role, inv.ProjectID, inv.CreatedBy, inv.ExpiresAt, inv.CreatedAt)
	return err
}

// GetInvitationByToken retrieves an invitation by its token
func (r *PGAuthRepository) GetInvitationByToken(ctx context.Context, token string) (*port.Invitation, error) {
	var inv port.Invitation
	err := r.db.QueryRowContext(ctx, `
		SELECT id, token, role, project_id, created_by, expires_at, used_at, used_by, created_at
		FROM invitation 
		WHERE token = $1
	`, token).Scan(
		&inv.ID, &inv.Token, &inv.Role, &inv.ProjectID, &inv.CreatedBy,
		&inv.ExpiresAt, &inv.UsedAt, &inv.UsedBy, &inv.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// UseInvitation marks an invitation as used
func (r *PGAuthRepository) UseInvitation(ctx context.Context, token, userID string) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		UPDATE invitation 
		SET used_at = $1, used_by = $2 
		WHERE token = $3 AND used_at IS NULL
	`, now, userID, token)
	return err
}

// ListInvitations lists all invitations created by a user
func (r *PGAuthRepository) ListInvitations(ctx context.Context, userID string) ([]*port.Invitation, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, token, role, project_id, created_by, expires_at, used_at, used_by, created_at
		FROM invitation 
		WHERE created_by = $1 
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invitations []*port.Invitation
	for rows.Next() {
		var inv port.Invitation
		if err := rows.Scan(
			&inv.ID, &inv.Token, &inv.Role, &inv.ProjectID, &inv.CreatedBy,
			&inv.ExpiresAt, &inv.UsedAt, &inv.UsedBy, &inv.CreatedAt,
		); err != nil {
			return nil, err
		}
		invitations = append(invitations, &inv)
	}
	return invitations, nil
}

// DeleteInvitation deletes an invitation by its token
func (r *PGAuthRepository) DeleteInvitation(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM invitation WHERE token = $1`, token)
	return err
}
