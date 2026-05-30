package domain

import (
	"errors"
	"time"
)

// Valid roles for invitations
var validRoles = map[string]bool{
	"admin":    true,
	"pm":       true,
	"dev_lead": true,
	"dev":      true,
	"observer": true,
}

// Invitation represents an invitation link in the system
type Invitation struct {
	ID        string     `json:"id"`
	Token     string     `json:"token"`
	Role      string     `json:"role"`
	ProjectID string     `json:"project_id,omitempty"`
	CreatedBy string     `json:"created_by"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	UsedBy    string     `json:"used_by,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Validate checks if the invitation is valid
func (i *Invitation) Validate() error {
	if i.Token == "" {
		return errors.New("token is required")
	}

	if !validRoles[i.Role] {
		return errors.New("invalid role")
	}

	if i.IsExpired() {
		return errors.New("invitation has expired")
	}

	return nil
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsValid checks if the invitation is valid and can be used
func (i *Invitation) IsValid() bool {
	return !i.IsExpired() && i.UsedAt == nil && i.Token != ""
}