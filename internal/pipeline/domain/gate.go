package domain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

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

// ArtifactVerifyError is returned by GateEvent.VerifyArtifactHash when the
// recomputed sha256 of the artifact contents does not match the hash recorded
// on the gate event. This guards against TOCTOU tampering of an artifact
// between submission and downstream-stage advancement.
type ArtifactVerifyError struct {
	PipelineID   string
	Stage        string
	ExpectedHash string
	ActualHash   string
}

func (e *ArtifactVerifyError) Error() string {
	return fmt.Sprintf("artifact hash mismatch for %s/%s: stored=%s computed=%s",
		e.PipelineID, e.Stage, e.ExpectedHash, e.ActualHash)
}

// VerifyArtifactHash recomputes the sha256 of contentBytes and compares it to
// the ArtifactHash recorded on the gate event. An empty stored hash is treated
// as a legacy event (no hash to verify) and returns nil so old flows are not
// regressed.
func (g *GateEvent) VerifyArtifactHash(contentBytes []byte) error {
	if g.ArtifactHash == "" {
		return nil
	}
	sum := sha256.Sum256(contentBytes)
	computed := hex.EncodeToString(sum[:])
	if computed != g.ArtifactHash {
		return &ArtifactVerifyError{
			PipelineID:   g.PipelineID,
			Stage:        g.Stage,
			ExpectedHash: g.ArtifactHash,
			ActualHash:   computed,
		}
	}
	return nil
}
