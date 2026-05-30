package middleware

import (
	"context"
	"testing"

	"openforge/internal/auth/domain"
)

func TestCanCreateInvitation(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		expected bool
	}{
		{"admin can create", "admin", true},
		{"pm can create", "pm", true},
		{"dev_lead can create", "dev_lead", true},
		{"dev cannot create", "dev", false},
		{"observer cannot create", "observer", false},
		{"empty role cannot create", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), domain.UserRoleContextKey, tt.role)
			result := CanCreateInvitation(ctx)
			if result != tt.expected {
				t.Errorf("CanCreateInvitation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCanInviteToProject(t *testing.T) {
	tests := []struct {
		name     string
		role     string
		isLead   bool
		expected bool
	}{
		{"admin can invite to any project", "admin", false, true},
		{"admin can invite even if lead", "admin", true, true},
		{"pm can invite to own project", "pm", true, true},
		{"pm cannot invite to other project", "pm", false, false},
		{"dev_lead can invite to own project", "dev_lead", true, true},
		{"dev_lead cannot invite to other project", "dev_lead", false, false},
		{"dev cannot invite", "dev", false, false},
		{"dev cannot invite even if lead", "dev", true, false},
		{"observer cannot invite", "observer", false, false},
		{"empty role cannot invite", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.WithValue(context.Background(), domain.UserRoleContextKey, tt.role)
			result := CanInviteToProject(ctx, tt.isLead)
			if result != tt.expected {
				t.Errorf("CanInviteToProject() = %v, want %v", result, tt.expected)
			}
		})
	}
}
