package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	authport "openforge/internal/auth/port"
	"openforge/internal/auth/service"
)

// mockAuthRepository is a mock implementation of authport.AuthRepository
type mockAuthRepository struct {
	users       map[string]*authport.User
	passwords   map[string]string
	invitations map[string]*authport.Invitation
	projects    map[string]*authport.Project
	roles       map[string]*authport.UserRole
}

func newMockAuthRepository() *mockAuthRepository {
	return &mockAuthRepository{
		users:       make(map[string]*authport.User),
		passwords:   make(map[string]string),
		invitations: make(map[string]*authport.Invitation),
		projects:    make(map[string]*authport.Project),
		roles:       make(map[string]*authport.UserRole),
	}
}

func (m *mockAuthRepository) CreateProject(ctx context.Context, p *authport.Project) error {
	m.projects[p.ID] = p
	return nil
}

func (m *mockAuthRepository) GetProject(ctx context.Context, id string) (*authport.Project, error) {
	p, ok := m.projects[id]
	if !ok {
		return nil, nil
	}
	return p, nil
}

func (m *mockAuthRepository) CreateUser(ctx context.Context, u *authport.User) error {
	m.users[u.ID] = u
	return nil
}

func (m *mockAuthRepository) GetUser(ctx context.Context, id string) (*authport.User, error) {
	u, ok := m.users[id]
	if !ok {
		return nil, nil
	}
	return u, nil
}

func (m *mockAuthRepository) AssignRole(ctx context.Context, r *authport.UserRole) error {
	key := r.UserID + ":" + r.ProjectID
	m.roles[key] = r
	return nil
}

func (m *mockAuthRepository) GetUserRole(ctx context.Context, userID, projectID string) (*authport.UserRole, error) {
	key := userID + ":" + projectID
	r, ok := m.roles[key]
	if !ok {
		return nil, nil
	}
	return r, nil
}

func (m *mockAuthRepository) RegisterUser(ctx context.Context, id, displayName, passwordHash, role string) error {
	m.users[id] = &authport.User{
		ID:          id,
		DisplayName: displayName,
		Role:        role,
	}
	m.passwords[id] = passwordHash
	return nil
}

func (m *mockAuthRepository) GetUserPasswordHash(ctx context.Context, id string) (string, error) {
	hash, ok := m.passwords[id]
	if !ok {
		return "", nil
	}
	return hash, nil
}

func (m *mockAuthRepository) ListUserProjects(ctx context.Context, userID string) ([]*authport.Project, error) {
	var projects []*authport.Project
	for _, r := range m.roles {
		if r.UserID == userID {
			if p, ok := m.projects[r.ProjectID]; ok {
				projects = append(projects, p)
			}
		}
	}
	return projects, nil
}

func (m *mockAuthRepository) UserHasProjectAccess(ctx context.Context, userID, projectID string) (bool, error) {
	key := userID + ":" + projectID
	_, ok := m.roles[key]
	return ok, nil
}

func (m *mockAuthRepository) CreateInvitation(ctx context.Context, inv *authport.Invitation) error {
	m.invitations[inv.Token] = inv
	return nil
}

func (m *mockAuthRepository) GetInvitationByToken(ctx context.Context, token string) (*authport.Invitation, error) {
	inv, ok := m.invitations[token]
	if !ok {
		return nil, nil
	}
	return inv, nil
}

func (m *mockAuthRepository) UseInvitation(ctx context.Context, token, userID string) error {
	inv, ok := m.invitations[token]
	if !ok {
		return nil
	}
	now := time.Now()
	inv.UsedAt = &now
	inv.UsedBy = userID
	return nil
}

func (m *mockAuthRepository) ListInvitations(ctx context.Context, userID string) ([]*authport.Invitation, error) {
	var invitations []*authport.Invitation
	for _, inv := range m.invitations {
		if inv.CreatedBy == userID {
			invitations = append(invitations, inv)
		}
	}
	return invitations, nil
}

func (m *mockAuthRepository) DeleteInvitation(ctx context.Context, token string) error {
	delete(m.invitations, token)
	return nil
}

// setupTestServer creates a test server with routes registered
func setupTestServer(t *testing.T) (*http.ServeMux, *service.JWTService, *mockAuthRepository) {
	t.Helper()

	// Create mock repository
	authRepo := newMockAuthRepository()

	// Create JWT service
	jwtSvc := service.NewJWTService("test-secret", 15*time.Minute, 24*time.Hour)

	// Create invitation service
	invitationSvc := service.NewInvitationService()

	// Create mux and register routes
	mux := http.NewServeMux()

	// Register the routes that we're testing
	mux.HandleFunc("POST /api/auth/register", handleRegisterPersonalMode(jwtSvc, authRepo))
	mux.HandleFunc("POST /api/invitations", requireAuth(handleCreateInvitation(authRepo, invitationSvc), jwtSvc))
	mux.HandleFunc("GET /api/invitations/verify", handleVerifyInvitation(authRepo, invitationSvc))
	mux.HandleFunc("POST /api/auth/register/invitation", handleRegisterWithInvitation(jwtSvc, authRepo, invitationSvc))

	return mux, jwtSvc, authRepo
}

func TestHandleRegisterPersonalMode(t *testing.T) {
	mux, _, authRepo := setupTestServer(t)

	t.Run("successful registration", func(t *testing.T) {
		reqBody := map[string]string{
			"username":     "testuser",
			"email":        "test@example.com",
			"password":     "password123",
			"display_name": "Test User",
			"role":         "dev",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "testuser", data["user_id"])
		assert.Equal(t, "dev", data["role"])
		assert.NotEmpty(t, data["access_token"])

		// Verify user was created in mock repo
		user, _ := authRepo.GetUser(context.Background(), "testuser")
		assert.NotNil(t, user)
		assert.Equal(t, "Test User", user.DisplayName)
	})

	t.Run("duplicate username", func(t *testing.T) {
		// Register first user
		reqBody := map[string]string{
			"username":     "existinguser",
			"email":        "existing@example.com",
			"password":     "password123",
			"display_name": "Existing User",
			"role":         "dev",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Try to register with same username
		reqBody["email"] = "another@example.com"
		body, _ = json.Marshal(reqBody)
		req = httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})

	t.Run("invalid role", func(t *testing.T) {
		reqBody := map[string]string{
			"username":     "newuser",
			"email":        "new@example.com",
			"password":     "password123",
			"display_name": "New User",
			"role":         "invalid-role",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing required fields", func(t *testing.T) {
		reqBody := map[string]string{
			"username": "incomplete",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleCreateInvitation(t *testing.T) {
	mux, jwtSvc, authRepo := setupTestServer(t)

	// Create a test user and get token
	tokenPair, err := jwtSvc.Issue("admin-user", "admin", "")
	require.NoError(t, err)

	t.Run("successful invitation creation", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"role":            "dev",
			"project_id":      "project-1",
			"expires_in_days": 7,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/invitations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "dev", data["role"])
		assert.Equal(t, "project-1", data["project_id"])
		assert.NotEmpty(t, data["token"])
		assert.NotEmpty(t, data["invitation_url"])

		// Verify invitation was saved in mock repo
		token := data["token"].(string)
		inv, _ := authRepo.GetInvitationByToken(context.Background(), token)
		assert.NotNil(t, inv)
		assert.Equal(t, "admin-user", inv.CreatedBy)
	})

	t.Run("unauthorized request", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"role": "dev",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/invitations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// No Authorization header
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
	})

	t.Run("invalid role", func(t *testing.T) {
		reqBody := map[string]interface{}{
			"role":            "invalid-role",
			"expires_in_days": 7,
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/invitations", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+tokenPair.AccessToken)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleVerifyInvitation(t *testing.T) {
	mux, _, authRepo := setupTestServer(t)

	// Create a test invitation
	testToken := "test-invitation-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	authRepo.invitations[testToken] = &authport.Invitation{
		Token:     testToken,
		Role:      "dev",
		ProjectID: "project-1",
		CreatedBy: "admin-user",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	// Add a test project
	authRepo.projects["project-1"] = &authport.Project{
		ID:   "project-1",
		Name: "Test Project",
	}

	t.Run("valid invitation", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/invitations/verify?token="+testToken, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.True(t, data["valid"].(bool))
		assert.Equal(t, "dev", data["role"])
		assert.Equal(t, "project-1", data["project_id"])
		assert.Equal(t, "Test Project", data["project_name"])
	})

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/invitations/verify", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-existent token", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/invitations/verify?token=non-existent-token", nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("expired invitation", func(t *testing.T) {
		expiredToken := "expired-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		authRepo.invitations[expiredToken] = &authport.Invitation{
			Token:     expiredToken,
			Role:      "dev",
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		req := httptest.NewRequest("GET", "/api/invitations/verify?token="+expiredToken, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("used invitation", func(t *testing.T) {
		usedToken := "used-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		usedAt := time.Now().Add(-1 * time.Hour)
		authRepo.invitations[usedToken] = &authport.Invitation{
			Token:     usedToken,
			Role:      "dev",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			UsedAt:    &usedAt,
			UsedBy:    "some-user",
			CreatedAt: time.Now().Add(-2 * time.Hour),
		}

		req := httptest.NewRequest("GET", "/api/invitations/verify?token="+usedToken, nil)
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestHandleRegisterWithInvitation(t *testing.T) {
	mux, _, authRepo := setupTestServer(t)

	// Create a test invitation
	testToken := "valid-invitation-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	authRepo.invitations[testToken] = &authport.Invitation{
		Token:     testToken,
		Role:      "dev",
		ProjectID: "project-1",
		CreatedBy: "admin-user",
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		CreatedAt: time.Now(),
	}

	// Add a test project
	authRepo.projects["project-1"] = &authport.Project{
		ID:   "project-1",
		Name: "Test Project",
	}

	t.Run("successful invitation registration", func(t *testing.T) {
		reqBody := map[string]string{
			"token":        testToken,
			"username":     "inviteduser",
			"email":        "invited@example.com",
			"password":     "password123",
			"display_name": "Invited User",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/auth/register/invitation", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusCreated, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.True(t, response["success"].(bool))

		data := response["data"].(map[string]interface{})
		assert.Equal(t, "inviteduser", data["user_id"])
		assert.Equal(t, "dev", data["role"])
		assert.Equal(t, "project-1", data["project_id"])
		assert.Equal(t, "Test Project", data["project_name"])
		assert.NotEmpty(t, data["access_token"])

		// Verify user was created
		user, _ := authRepo.GetUser(context.Background(), "inviteduser")
		assert.NotNil(t, user)
		assert.Equal(t, "Invited User", user.DisplayName)

		// Verify invitation was marked as used
		inv, _ := authRepo.GetInvitationByToken(context.Background(), testToken)
		assert.NotNil(t, inv.UsedAt)
		assert.Equal(t, "inviteduser", inv.UsedBy)

		// Verify role was assigned
		key := "inviteduser:project-1"
		role, exists := authRepo.roles[key]
		assert.True(t, exists)
		assert.Equal(t, "dev", role.Role)
	})

	t.Run("missing required fields", func(t *testing.T) {
		// Create a fresh invitation for this test
		newToken := "new-invitation-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		authRepo.invitations[newToken] = &authport.Invitation{
			Token:     newToken,
			Role:      "dev",
			ProjectID: "project-1",
			CreatedBy: "admin-user",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}

		reqBody := map[string]string{
			"token":    newToken,
			"username": "incomplete",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/auth/register/invitation", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("invalid invitation token", func(t *testing.T) {
		reqBody := map[string]string{
			"token":        "non-existent-token",
			"username":     "newuser",
			"email":        "new@example.com",
			"password":     "password123",
			"display_name": "New User",
		}
		body, _ := json.Marshal(reqBody)

		req := httptest.NewRequest("POST", "/api/auth/register/invitation", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("duplicate username", func(t *testing.T) {
		// Create a fresh invitation
		dupToken := "dup-invitation-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
		authRepo.invitations[dupToken] = &authport.Invitation{
			Token:     dupToken,
			Role:      "dev",
			ProjectID: "project-1",
			CreatedBy: "admin-user",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}

		// First registration
		reqBody := map[string]string{
			"token":        dupToken,
			"username":     "dupuser",
			"email":        "dup@example.com",
			"password":     "password123",
			"display_name": "Dup User",
		}
		body, _ := json.Marshal(reqBody)
		req := httptest.NewRequest("POST", "/api/auth/register/invitation", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		assert.Equal(t, http.StatusCreated, w.Code)

		// Create another invitation for duplicate test
		dupToken2 := "dup2-invitation-token-1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcde"
		authRepo.invitations[dupToken2] = &authport.Invitation{
			Token:     dupToken2,
			Role:      "dev",
			ProjectID: "project-1",
			CreatedBy: "admin-user",
			ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
			CreatedAt: time.Now(),
		}

		// Try same username with different invitation
		reqBody["token"] = dupToken2
		reqBody["email"] = "dup2@example.com"
		body, _ = json.Marshal(reqBody)
		req = httptest.NewRequest("POST", "/api/auth/register/invitation", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w = httptest.NewRecorder()
		mux.ServeHTTP(w, req)

		assert.Equal(t, http.StatusConflict, w.Code)
	})
}
