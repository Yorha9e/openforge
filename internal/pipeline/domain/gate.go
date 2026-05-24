package domain

import "time"

type GateChecklist struct {
	CodeReviewed      bool `json:"code_reviewed"`
	SecurityChecked   bool `json:"security_checked"`
	LicenseCleared    bool `json:"license_cleared"`
	CodingStandardMet bool `json:"coding_standard_met"`
}

type LineComment struct {
	FilePath string `json:"file_path"`
	Line     int    `json:"line"`
	Comment  string `json:"comment"`
	Mark     string `json:"mark"`
}

type GateEvent struct {
	PipelineID      string         `json:"pipeline_id"`
	Stage           string         `json:"stage"`
	Event           string         `json:"event"`
	Actor           string         `json:"actor"`
	Decision        string         `json:"decision"`
	LineComments    []LineComment  `json:"line_comments"`
	SummaryFeedback string         `json:"summary_feedback"`
	Checklist       GateChecklist  `json:"checklist"`
	ArtifactHash    string         `json:"artifact_hash"`
	PrevHash        string         `json:"prev_hash"`
	ContentHash     string         `json:"content_hash"`
	CreatedAt       time.Time      `json:"created_at"`
}
