package domain

import (
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvitationValidation(t *testing.T) {
	// Test valid invitation
	inv := &Invitation{
		ID:        "test-id",
		Token:     "test-token-123",
		Role:      "dev",
		ProjectID: "project-1",
		CreatedBy: "user-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	err := inv.Validate()
	require.NoError(t, err)

	// Test invalid role
	inv.Role = "invalid-role"
	err = inv.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid role")

	// Test empty token
	inv.Role = "dev"
	inv.Token = ""
	err = inv.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "token is required")

	// Test expired invitation
	inv.Token = "test-token-123"
	inv.ExpiresAt = time.Now().Add(-1 * time.Hour)
	err = inv.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invitation has expired")
}

func TestInvitationIsExpired(t *testing.T) {
	// Test expired invitation
	inv := &Invitation{
		ExpiresAt: time.Now().Add(-1 * time.Hour),
	}
	assert.True(t, inv.IsExpired())

	// Test valid invitation
	inv.ExpiresAt = time.Now().Add(1 * time.Hour)
	assert.False(t, inv.IsExpired())
}

func TestInvitationIsValid(t *testing.T) {
	// Test valid invitation
	inv := &Invitation{
		Token:     "test-token-123",
		Role:      "dev",
		ExpiresAt: time.Now().Add(1 * time.Hour),
		UsedAt:    nil,
	}
	assert.True(t, inv.IsValid())

	// Test used invitation
	usedAt := time.Now()
	inv.UsedAt = &usedAt
	assert.False(t, inv.IsValid())

	// Test expired invitation
	inv.UsedAt = nil
	inv.ExpiresAt = time.Now().Add(-1 * time.Hour)
	assert.False(t, inv.IsValid())
}