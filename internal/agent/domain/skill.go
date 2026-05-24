package domain

import "time"

// Skill represents a loaded Skill with YAML frontmatter and Markdown body content.
type Skill struct {
	Name         string        `yaml:"name"`
	Version      string        `yaml:"version"`
	Stages       []string      `yaml:"stages"`
	Complexity   []string      `yaml:"complexity,omitempty"`
	Permission   []string      `yaml:"permission,omitempty"`
	Keywords     []string      `yaml:"keywords,omitempty"`
	Triggers     SkillTriggers `yaml:"triggers,omitempty"`
	BasePriority int           `yaml:"base_priority"`

	// Runtime fields merged from skill_config.yaml
	CurrentPriority float64   `yaml:"current_priority"`
	Enabled         bool      `yaml:"enabled"`
	Deprecated      bool      `yaml:"deprecated"`
	IsLatest        bool      `yaml:"is_latest"`
	PublishedAt     time.Time `yaml:"published_at"`
	CreatedAt       time.Time `yaml:"created_at"`

	// Body content parsed from Markdown
	Prompt   string   `yaml:"-"`
	Workflow []string `yaml:"-"`

	// Metadata
	Source   string `yaml:"-"` // "global" | "team" | "project"
	FilePath string `yaml:"-"` // source file path
}

// SkillTriggers defines trigger conditions for a Skill.
type SkillTriggers struct {
	FilePatterns []string `yaml:"file_patterns,omitempty"`
	UserIntent   []string `yaml:"user_intent,omitempty"`
}

// SkillConfig is the runtime state index entry in skill_config.yaml.
type SkillConfig struct {
	Name            string    `yaml:"name"`
	Version         string    `yaml:"version"`
	File            string    `yaml:"file"`
	BasePriority    int       `yaml:"base_priority"`
	CurrentPriority float64   `yaml:"current_priority"`
	Enabled         bool      `yaml:"enabled"`
	IsLatest        bool      `yaml:"is_latest"`
	Deprecated      bool      `yaml:"deprecated"`
	PublishedAt     time.Time `yaml:"published_at"`
}

// skillSnapshot is an immutable CoW snapshot of all loaded skills and configs.
type skillSnapshot struct {
	skills    []Skill
	config    map[string]SkillConfig // key: "name@version"
	buildTime time.Time
}

// SkillInjectionRecord records a skill that was injected into a prompt.
type SkillInjectionRecord struct {
	Name            string  `json:"name"`
	Version         string  `json:"version"`
	Source          string  `json:"source"`           // "project" | "team" | "global"
	TriggerScore    float64 `json:"trigger_score"`
	CurrentPriority float64 `json:"current_priority"`
	TriggeredBy     string  `json:"triggered_by"`     // "stage_filter" | "user_command" | "keyword_match" | "file_pattern"
	TokenCost       int     `json:"token_cost"`
}

// Default skill priority constants
const (
	DefaultBasePriority = 70
	MaxBasePriority     = 100
	MinBasePriority     = 0
	DefaultMaxInject    = 5
	DefaultAvgSkillTokens = 500
)
