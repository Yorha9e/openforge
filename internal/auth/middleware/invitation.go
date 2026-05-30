package middleware

import (
	"context"

	"openforge/internal/auth/domain"
)

// CanCreateInvitation checks if the user can create invitations.
// Admin, PM, and dev_lead can create invitations.
func CanCreateInvitation(ctx context.Context) bool {
	role, ok := ctx.Value(domain.UserRoleContextKey).(string)
	if !ok || role == "" {
		return false
	}
	switch role {
	case "admin", "pm", "dev_lead":
		return true
	default:
		return false
	}
}

// CanInviteToProject checks if the user can invite to a specific project.
// Admin can invite to any project. PM and dev_lead can only invite to projects they lead.
func CanInviteToProject(ctx context.Context, isProjectLead bool) bool {
	role, ok := ctx.Value(domain.UserRoleContextKey).(string)
	if !ok || role == "" {
		return false
	}
	if role == "admin" {
		return true
	}
	if (role == "pm" || role == "dev_lead") && isProjectLead {
		return true
	}
	return false
}
