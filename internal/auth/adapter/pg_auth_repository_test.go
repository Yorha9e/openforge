package adapter

import (
	"context"
	"database/sql"
	"testing"
	"time"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "github.com/lib/pq"
	port "openforge/internal/auth/port"
)

func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("postgres", "postgres://openforge:openforge_dev@localhost:5432/openforge?sslmode=disable")
	require.NoError(t, err)
	
	// Clean up test data
	_, _ = db.Exec(`DELETE FROM invitation WHERE token LIKE 'test-%'`)
	
	return db
}

func TestCreateInvitation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	repo := NewPGAuthRepository(db)
	ctx := context.Background()
	
	inv := &port.Invitation{
		Token:     "test-token-" + time.Now().Format("20060102150405"),
		Role:      "dev",
		ProjectID: "project-1",
		CreatedBy: "admin",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}
	
	err := repo.CreateInvitation(ctx, inv)
	require.NoError(t, err)
	
	// Verify invitation was created
	var count int
	err = db.QueryRow(`SELECT COUNT(*) FROM invitation WHERE token = $1`, inv.Token).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestGetInvitationByToken(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	repo := NewPGAuthRepository(db)
	ctx := context.Background()
	
	// Test non-existent invitation
	inv, err := repo.GetInvitationByToken(ctx, "non-existent-token")
	require.NoError(t, err)
	assert.Nil(t, inv)
	
	// Create test invitation
	token := "test-token-" + time.Now().Format("20060102150405")
	_, err = db.Exec(`
		INSERT INTO invitation (token, role, created_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token, "dev", "admin", time.Now().Add(24*time.Hour), time.Now())
	require.NoError(t, err)
	
	// Test existing invitation
	inv, err = repo.GetInvitationByToken(ctx, token)
	require.NoError(t, err)
	assert.NotNil(t, inv)
	assert.Equal(t, token, inv.Token)
	assert.Equal(t, "dev", inv.Role)
}

func TestUseInvitation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()
	
	repo := NewPGAuthRepository(db)
	ctx := context.Background()
	
	// Create test invitation
	token := "test-token-" + time.Now().Format("20060102150405")
	_, err := db.Exec(`
		INSERT INTO invitation (token, role, created_by, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, token, "dev", "admin", time.Now().Add(24*time.Hour), time.Now())
	require.NoError(t, err)
	
	// Use invitation
	err = repo.UseInvitation(ctx, token, "user-2")
	require.NoError(t, err)
	
	// Verify invitation was used
	var usedBy string
	var usedAt time.Time
	err = db.QueryRow(`SELECT used_by, used_at FROM invitation WHERE token = $1`, token).Scan(&usedBy, &usedAt)
	require.NoError(t, err)
	assert.Equal(t, "user-2", usedBy)
	assert.False(t, usedAt.IsZero())
}