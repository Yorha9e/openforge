package port

import (
	"context"
	"errors"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockRepository implements AuthRepository for testing
type MockRepository struct {
	invitations map[string]*Invitation
	users       map[string]*User
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		invitations: make(map[string]*Invitation),
		users:       make(map[string]*User),
	}
}

func (m *MockRepository) CreateInvitation(ctx context.Context, inv *Invitation) error {
	m.invitations[inv.Token] = inv
	return nil
}

func (m *MockRepository) GetInvitationByToken(ctx context.Context, token string) (*Invitation, error) {
	inv, exists := m.invitations[token]
	if !exists {
		return nil, nil
	}
	return inv, nil
}

func (m *MockRepository) UseInvitation(ctx context.Context, token, userID string) error {
	inv, exists := m.invitations[token]
	if !exists {
		return errors.New("invitation not found")
	}
	now := time.Now()
	inv.UsedAt = &now
	inv.UsedBy = userID
	return nil
}

func (m *MockRepository) ListInvitations(ctx context.Context, userID string) ([]*Invitation, error) {
	var result []*Invitation
	for _, inv := range m.invitations {
		if inv.CreatedBy == userID {
			result = append(result, inv)
		}
	}
	return result, nil
}

func (m *MockRepository) DeleteInvitation(ctx context.Context, token string) error {
	delete(m.invitations, token)
	return nil
}

func TestCreateInvitation(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	inv := &Invitation{
		Token:     "test-token-123",
		Role:      "dev",
		ProjectID: "project-1",
		CreatedBy: "user-1",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	err := repo.CreateInvitation(ctx, inv)
	require.NoError(t, err)

	// Verify invitation was created
	saved, err := repo.GetInvitationByToken(ctx, "test-token-123")
	require.NoError(t, err)
	assert.Equal(t, inv.Token, saved.Token)
	assert.Equal(t, inv.Role, saved.Role)
}

func TestGetInvitationByToken(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Test non-existent invitation
	inv, err := repo.GetInvitationByToken(ctx, "non-existent")
	require.NoError(t, err)
	assert.Nil(t, inv)

	// Create invitation
	expected := &Invitation{
		Token:     "test-token-123",
		Role:      "dev",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	repo.invitations["test-token-123"] = expected

	// Test existing invitation
	inv, err = repo.GetInvitationByToken(ctx, "test-token-123")
	require.NoError(t, err)
	assert.Equal(t, expected.Token, inv.Token)
}

func TestUseInvitation(t *testing.T) {
	repo := NewMockRepository()
	ctx := context.Background()

	// Create invitation
	inv := &Invitation{
		Token:     "test-token-123",
		Role:      "dev",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	repo.invitations["test-token-123"] = inv

	// Use invitation
	err := repo.UseInvitation(ctx, "test-token-123", "user-2")
	require.NoError(t, err)

	// Verify invitation was used
	saved, _ := repo.GetInvitationByToken(ctx, "test-token-123")
	assert.NotNil(t, saved.UsedAt)
	assert.Equal(t, "user-2", saved.UsedBy)
}