package server

import (
	"net/http"
	"strings"

	"openforge/internal/agent/domain"
	"openforge/internal/shared/profile"
)

// RegisterSkillRoutes registers skill admin REST endpoints.
func RegisterSkillRoutes(mux *http.ServeMux, of *profile.OpenForge) {
	mux.HandleFunc("/api/admin/skills", func(w http.ResponseWriter, r *http.Request) {
		handleSkillsList(of, w, r)
	})
	mux.HandleFunc("/api/admin/skills/", func(w http.ResponseWriter, r *http.Request) {
		handleSkillDetail(of, w, r)
	})
	mux.HandleFunc("/api/pipelines/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/skills") {
			handlePipelineSkills(of, w, r)
			return
		}
		http.NotFound(w, r)
	})
}

func handleSkillsList(of *profile.OpenForge, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if of.SkillLoader == nil {
		writeJSON(w, http.StatusOK, []interface{}{})
		return
	}

	skills := of.SkillLoader.GetAllSkills()
	result := make([]map[string]interface{}, 0, len(skills))
	for _, s := range skills {
		result = append(result, skillToMap(s))
	}
	writeJSON(w, http.StatusOK, result)
}

func handleSkillDetail(of *profile.OpenForge, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimPrefix(r.URL.Path, "/api/admin/skills/")
	if name == "" {
		http.Error(w, "skill name required", http.StatusBadRequest)
		return
	}
	name = strings.TrimSuffix(name, "/")
	if idx := strings.Index(name, "/"); idx >= 0 {
		name = name[:idx]
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

func handlePipelineSkills(of *profile.OpenForge, w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, []interface{}{})
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
