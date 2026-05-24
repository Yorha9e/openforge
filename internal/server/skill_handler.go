package server

import (
	"net/http"
	"strings"

	"openforge/internal/agent/domain"
	"openforge/internal/shared/profile"
)

// RegisterSkillRoutes registers skill admin REST endpoints.
// adminMw wraps handlers with auth + admin role requirement.
func RegisterSkillRoutes(mux *http.ServeMux, of *profile.OpenForge, adminMw func(http.HandlerFunc) http.HandlerFunc, authMw func(http.HandlerFunc) http.HandlerFunc) {
	// Admin-only routes
	mux.HandleFunc("GET /api/admin/skills", adminMw(handleSkillsList(of)))
	mux.HandleFunc("GET /api/admin/skills/{name}", adminMw(handleSkillDetail(of)))
	mux.HandleFunc("GET /api/admin/skills/pending-deprecations", adminMw(handlePendingDeprecations(of)))
	// Pipeline skills: any authenticated user
	mux.HandleFunc("GET /api/pipelines/{pid}/skills", authMw(handlePipelineSkills(of)))
}

func handleSkillsList(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if of.SkillLoader == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		skills := of.SkillLoader.GetAllSkills()
		result := make([]map[string]any, 0, len(skills))
		for _, s := range skills {
			result = append(result, skillToMap(s))
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleSkillDetail(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if name == "" {
			http.Error(w, "skill name required", http.StatusBadRequest)
			return
		}
		if strings.Contains(name, "/") {
			name = name[:strings.Index(name, "/")]
		}
		if of.SkillLoader == nil {
			http.Error(w, "skill loader not available", http.StatusServiceUnavailable)
			return
		}
		skill, err := of.SkillLoader.MatchByName(name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, skillToMap(*skill))
	}
}

func handlePendingDeprecations(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if of.SkillLoader == nil {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		var pending []map[string]any
		for _, s := range of.SkillLoader.GetAllSkills() {
			if s.Deprecated {
				pending = append(pending, skillToMap(s))
			}
		}
		writeJSON(w, http.StatusOK, pending)
	}
}

func handlePipelineSkills(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, []any{})
	}
}

func skillToMap(s domain.Skill) map[string]interface{} {
	return map[string]interface{}{
		"name":             s.Name,
		"version":          s.Version,
		"source":           s.Source,
		"stages":           s.Stages,
		"keywords":         s.Keywords,
		"complexity":       s.Complexity,
		"permission":       s.Permission,
		"base_priority":    s.BasePriority,
		"current_priority": s.CurrentPriority,
		"enabled":          s.Enabled,
		"deprecated":       s.Deprecated,
		"is_latest":        s.IsLatest,
		"prompt_preview":   truncateSkillStr(s.Prompt, 200),
		"workflow_steps":   len(s.Workflow),
	}
}

func truncateSkillStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
