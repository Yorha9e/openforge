package server

import (
	"encoding/json"
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
	mux.HandleFunc("PATCH /api/admin/skills/{name}", adminMw(handleSkillDeprecateToggle(of)))
	mux.HandleFunc("PUT /api/admin/skills/priorities", adminMw(handleSkillPrioritiesUpdate(of)))
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

func handleSkillDeprecateToggle(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if of.SkillLoader == nil {
			writeError(w, http.StatusServiceUnavailable, "skill loader not available")
			return
		}

		name := r.PathValue("name")
		if name == "" {
			writeError(w, http.StatusBadRequest, "skill name required")
			return
		}

		var req struct {
			Deprecated bool `json:"deprecated"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if err := of.SkillLoader.UpdateSkillDeprecated(name, req.Deprecated); err != nil {
			writeError(w, http.StatusInternalServerError, sanitizeError(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"name":       name,
			"deprecated": req.Deprecated,
		})
	}
}

func handleSkillPrioritiesUpdate(of *profile.OpenForge) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if of.SkillLoader == nil {
			writeError(w, http.StatusServiceUnavailable, "skill loader not available")
			return
		}

		var req struct {
			Priorities map[string]float64 `json:"priorities"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}

		if len(req.Priorities) == 0 {
			writeError(w, http.StatusBadRequest, "priorities map required")
			return
		}

		if err := of.SkillLoader.UpdateSkillPriorities(req.Priorities); err != nil {
			writeError(w, http.StatusInternalServerError, sanitizeError(err))
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"updated": len(req.Priorities),
		})
	}
}

func truncateSkillStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
