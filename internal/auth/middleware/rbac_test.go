package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"openforge/internal/auth/domain"
)

func TestCan_Alignment(t *testing.T) {
	tests := []struct {
		name   string
		roles  []string
		action string
		want   bool
	}{
		{"admin all", []string{"admin"}, "admin", true},        // admin bypasses action check (early return)
		{"admin read", []string{"admin"}, "read", true},
		{"admin execute", []string{"admin"}, "execute", true},
		{"pm execute", []string{"pm"}, "execute", true},
		{"pm read", []string{"pm"}, "read", true},
		{"pm cannot approve self", []string{"pm"}, "approve", true}, // pm CAN approve
		{"dev read", []string{"dev"}, "read", true},
		{"dev cannot approve", []string{"dev"}, "approve", false},
		{"dev_lead approve", []string{"dev_lead"}, "approve", true},
		{"observer read", []string{"observer"}, "read", true},
		{"observer cannot execute", []string{"observer"}, "execute", false},
		{"empty roles cannot execute", []string{}, "execute", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.Can(tt.roles, tt.action, "")
			if got != tt.want {
				t.Errorf("Can(%v, %q, \"\") = %v, want %v", tt.roles, tt.action, got, tt.want)
			}
		})
	}
}

func TestRequireRole(t *testing.T) {
	tests := []struct {
		name       string
		userRole   string
		required   string
		wantAccess bool
	}{
		{"admin passes all", "admin", "admin", true},
		{"admin passes pm route", "admin", "pm", true},
		{"pm passes pm route", "pm", "pm", true},
		{"pm fails admin route", "pm", "admin", false},
		{"dev fails pm route", "dev", "pm", false},
		{"dev passes dev route", "dev", "dev", true},
		{"empty role fails", "", "pm", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), domain.UserRoleContextKey, tt.userRole)
			got := RequireRole(ctx, tt.required)
			if got != tt.wantAccess {
				t.Errorf("RequireRole(%q, %q) = %v, want %v", tt.userRole, tt.required, got, tt.wantAccess)
			}
		})
	}
}

func TestRequireRoleMiddleware(t *testing.T) {
	handler := RequireRoleMiddleware("pm", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	})

	t.Run("allowed role", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), domain.UserRoleContextKey, "pm"))
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != 200 {
			t.Errorf("expected 200, got %d", w.Code)
		}
	})

	t.Run("forbidden role", func(t *testing.T) {
		r := httptest.NewRequest("GET", "/", nil)
		r = r.WithContext(context.WithValue(r.Context(), domain.UserRoleContextKey, "dev"))
		w := httptest.NewRecorder()
		handler(w, r)
		if w.Code != 403 {
			t.Errorf("expected 403, got %d", w.Code)
		}
	})
}
