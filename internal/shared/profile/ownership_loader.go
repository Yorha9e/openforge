package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ModuleOwnershipYAML is the in-memory representation of a module-ownership
// YAML file. Each entry maps a logical module name to its path prefix,
// responsible owners, fallback owner, optional OOO (out-of-office) group,
// and an auto-bypass flag for trusted review paths.
//
// This file is loaded once at startup (see LoadOwnership) and the canonical
// data is then seeded into the `module_ownership` PG table by migration 015.
// At runtime, the PG-backed PGOwnershipRepository is the source of truth.
type ModuleOwnershipYAML struct {
	Modules map[string]ModuleEntry `yaml:"modules"`
}

// ModuleEntry describes a single module's ownership configuration.
type ModuleEntry struct {
	Prefix     string   `yaml:"prefix"`     // file path prefix (e.g. "src/features/chat/")
	Owners     []string `yaml:"owners"`     // primary owner user IDs / emails
	Fallback   string   `yaml:"fallback"`   // fallback reviewer when no owner is on-call
	OOOGroup   string   `yaml:"ooo_group"`  // group consulted when all owners are OOO
	AutoBypass bool     `yaml:"auto_bypass"` // trusted enough to skip gate review entirely
}

// LoadOwnership reads a module-ownership YAML file from disk and parses it
// into a ModuleOwnershipYAML value. The file is expected to follow the
// `modules: { <name>: { prefix, owners, fallback, ooo_group, auto_bypass } }`
// schema. An error is returned if the file cannot be read or if the YAML
// is malformed. Missing fields default to their zero values.
func LoadOwnership(path string) (*ModuleOwnershipYAML, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read module-ownership %s: %w", path, err)
	}
	var m ModuleOwnershipYAML
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse module-ownership %s: %w", path, err)
	}
	return &m, nil
}
