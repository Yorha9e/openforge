package service

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	port "openforge/internal/auth/port"
)

// Valid roles for invitations
var validRoles = map[string]bool{
	"admin":    true,
	"pm":       true,
	"dev_lead": true,
	"dev":      true,
	"observer": true,
}

// InvitationService provides invitation-related business logic
type InvitationService struct{}

// NewInvitationService creates a new InvitationService
func NewInvitationService() *InvitationService {
	return &InvitationService{}
}

// GenerateToken generates a cryptographically secure random token
func (s *InvitationService) GenerateToken() (string, error) {
	bytes := make([]byte, 32) // 32 bytes = 64 hex characters
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateInvitation creates a new invitation with the given parameters
func (s *InvitationService) CreateInvitation(role, projectID, createdBy string, expirationDays int) (*port.Invitation, error) {
	// Validate role
	if !validRoles[role] {
		return nil, errors.New("invalid role")
	}

	// Validate expiration days
	if expirationDays <= 0 {
		return nil, errors.New("expiration days must be positive")
	}

	// Generate token
	token, err := s.GenerateToken()
	if err != nil {
		return nil, err
	}

	// Create invitation
	inv := &port.Invitation{
		Token:     token,
		Role:      role,
		ProjectID: projectID,
		CreatedBy: createdBy,
		ExpiresAt: time.Now().Add(time.Duration(expirationDays) * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	return inv, nil
}

// ValidateInvitation validates an invitation
func (s *InvitationService) ValidateInvitation(inv *port.Invitation) error {
	if inv == nil {
		return errors.New("invitation is nil")
	}

	if time.Now().After(inv.ExpiresAt) {
		return errors.New("invitation has expired")
	}

	if inv.UsedAt != nil {
		return errors.New("invitation has already been used")
	}

	return nil
}