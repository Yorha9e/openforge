package compliance

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Package is a single artifact discovered by syft. It is a minimal
// projection of syft's own JSON shape: we keep only the fields the
// license report needs (name, version, license string).
type Package struct {
	Name     string `json:"name"`
	Version  string `json:"version"`
	Licenses string `json:"licenses"`
}

// SyftDocument is the slice of the syft JSON document we care about.
// syft emits a richer schema (sources, source, distro, etc.) but the
// license report only needs the artifact list.
type SyftDocument struct {
	Artifacts []Package `json:"artifacts"`
}

// SBOMScanner is the interface the license report depends on. It is
// defined here (rather than in report_generator.go) so the concrete
// syft-backed implementation and the test doubles live next to the
// types they consume.
type SBOMScanner interface {
	// Generate invokes syft and returns the full artifact list.
	Generate(ctx context.Context) ([]Package, error)
	// ScanForGplAgpl returns one ReportRow per GPL/AGPL-licensed
	// package discovered. The label encodes the package identity and
	// license so the report can show a single glance summary.
	ScanForGplAgpl(ctx context.Context) ([]ReportRow, error)
}

// gplLicenseTokens is the set of substrings we treat as "copyleft
// strong" — any license string containing one of these tokens is
// considered a copyleft hit. The tokens are matched case-insensitively.
var gplLicenseTokens = []string{
	"GPL",
	"AGPL",
	"SSPL",
}

// SyftSBOMScanner is the production SBOMScanner. It shells out to the
// `syft` CLI installed on the host. If syft is not on PATH, Generate
// returns a wrapped error rather than panicking so callers (including
// the monthly scheduler) can degrade gracefully.
type SyftSBOMScanner struct {
	workdir string
}

// NewSBOMScanner constructs a scanner rooted at workdir. An empty
// workdir is allowed and is forwarded to syft verbatim; syft treats
// "." as the current directory.
func NewSBOMScanner(workdir string) *SyftSBOMScanner {
	return &SyftSBOMScanner{workdir: workdir}
}

// Generate runs `syft scan <workdir> -o syft-json` and parses the JSON
// payload. The process is bounded by ctx; cancellation propagates to
// the subprocess via exec.CommandContext.
//
// If syft is not on PATH the lookup failure is wrapped with a stable
// sentinel substring ("syft: executable not found") so callers can
// match on it if they want to degrade to a no-SBOM report.
func (s *SyftSBOMScanner) Generate(ctx context.Context) ([]Package, error) {
	bin, err := exec.LookPath("syft")
	if err != nil {
		return nil, fmt.Errorf("syft: executable not found in PATH: %w", err)
	}

	workdir := s.workdir
	if workdir == "" {
		workdir = "."
	}

	args := []string{"scan", workdir, "-o", "syft-json"}
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("syft scan %q: %w", workdir, err)
	}

	var doc SyftDocument
	if err := json.Unmarshal(out, &doc); err != nil {
		return nil, fmt.Errorf("syft scan: decode JSON: %w", err)
	}
	return doc.Artifacts, nil
}

// ScanForGplAgpl calls Generate, then filters the artifact list down
// to packages whose Licenses field mentions a copyleft token
// (GPL/AGPL/SSPL). Each hit becomes one ReportRow whose label encodes
// the package coordinate and the offending license so reviewers can
// see at a glance what needs review.
func (s *SyftSBOMScanner) ScanForGplAgpl(ctx context.Context) ([]ReportRow, error) {
	pkgs, err := s.Generate(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]ReportRow, 0)
	for _, p := range pkgs {
		lic := strings.TrimSpace(p.Licenses)
		if lic == "" {
			continue
		}
		if !isCopyleftLicense(lic) {
			continue
		}
		label := fmt.Sprintf("%s@%s (%s)", p.Name, p.Version, lic)
		rows = append(rows, ReportRow{Label: label, Count: 1})
	}
	return rows, nil
}

// isCopyleftLicense returns true if lic contains any of the
// gplLicenseTokens (case-insensitive). Syft returns license strings
// as space- or comma-separated names ("GPL-3.0 OR MIT"), so substring
// matching is sufficient and avoids pulling in a full SPDX parser.
func isCopyleftLicense(lic string) bool {
	upper := strings.ToUpper(lic)
	for _, tok := range gplLicenseTokens {
		if strings.Contains(upper, tok) {
			return true
		}
	}
	return false
}
