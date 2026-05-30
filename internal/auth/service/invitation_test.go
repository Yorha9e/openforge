package service

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	port "openforge/internal/auth/port"
)

func TestGenerateToken(t *testing.T) {
	svc := NewInvitationService()
	
	// Test token generation
	token1, err := svc.GenerateToken()
	require.NoError(t, err)
	assert.NotEmpty(t, token1)
	assert.Len(t, token1, 64) // 32 bytes = 64 hex characters
	
	// Test uniqueness
	token2, err := svc.GenerateToken()
	require.NoError(t, err)
	assert.NotEqual(t, token1, token2)
}

func TestCreateInvitation(t *testing.T) {
	svc := NewInvitationService()
	
	// Test valid invitation creation
	inv, err := svc.CreateInvitation("dev", "project-1", "user-1", 7)
	require.NoError(t, err)
	assert.NotNil(t, inv)
	assert.NotEmpty(t, inv.Token)
	assert.Equal(t, "dev", inv.Role)
	assert.Equal(t, "project-1", inv.ProjectID)
	assert.Equal(t, "user-1", inv.CreatedBy)
	assert.True(t, inv.ExpiresAt.After(time.Now()))
	assert.Nil(t, inv.UsedAt)
	
	// Test invalid role
	_, err = svc.CreateInvitation("invalid-role", "project-1", "user-1", 7)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")
	
	// Test invalid expiration days
	_, err = svc.CreateInvitation("dev", "project-1", "user-1", 0)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expiration days must be positive")
}

func TestValidateInvitation(t *testing.T) {
	svc := NewInvitationService()
	
	// Test valid invitation
	inv := &port.Invitation{
		Token:     "test-token-123",
		Role:      "dev",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    nil,
	}
	err := svc.ValidateInvitation(inv)
	require.NoError(t, err)
	
	// Test nil invitation
	err = svc.ValidateInvitation(nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitation is nil")
	
	// Test expired invitation
	inv.ExpiresAt = time.Now().Add(-1 * time.Hour)
	err = svc.ValidateInvitation(inv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitation has expired")
	
	// Test used invitation
	inv.ExpiresAt = time.Now().Add(1 * time.Hour)
	usedAt := time.Now()
	inv.UsedAt = &usedAt
	err = svc.ValidateInvitation(inv)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitation has already been used")
}