package profile

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadOwnership_ValidYAML verifies LoadOwnership parses a well-formed
// module-ownership YAML file into a non-empty Modules map.
func TestLoadOwnership_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	yamlPath := filepath.Join(dir, "module-ownership.yaml")
	contents := `modules:
  chat:
    prefix: src/features/chat/
    owners:
      - dev_lead_a
    fallback: dev_lead_b
    ooo_group: pm-on-call
    auto_bypass: false
  code_review:
    prefix: src/features/code-review/
    owners:
      - dev_lead_b
    fallback: dev_lead_a
    ooo_group: pm-on-call
    auto_bypass: true
`
	if err := os.WriteFile(yamlPath, []byte(contents), 0o644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}

	owner, err := LoadOwnership(yamlPath)
	if err != nil {
		t.Fatalf("LoadOwnership returned error: %v", err)
	}
	if owner == nil {
		t.Fatal("LoadOwnership returned nil")
	}
	if len(owner.Modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(owner.Modules))
	}
	chat, ok := owner.Modules["chat"]
	if !ok {
		t.Fatal("expected 'chat' key in Modules map")
	}
	if chat.Prefix != "src/features/chat/" {
		t.Errorf("chat.Prefix = %q, want %q", chat.Prefix, "src/features/chat/")
	}
	if len(chat.Owners) != 1 || chat.Owners[0] != "dev_lead_a" {
		t.Errorf("chat.Owners = %v, want [dev_lead_a]", chat.Owners)
	}
	if chat.Fallback != "dev_lead_b" {
		t.Errorf("chat.Fallback = %q, want dev_lead_b", chat.Fallback)
	}
	if chat.OOOGroup != "pm-on-call" {
		t.Errorf("chat.OOOGroup = %q, want pm-on-call", chat.OOOGroup)
	}
	if chat.AutoBypass {
		t.Errorf("chat.AutoBypass = true, want false")
	}

	cr, ok := owner.Modules["code_review"]
	if !ok {
		t.Fatal("expected 'code_review' key in Modules map")
	}
	if !cr.AutoBypass {
		t.Errorf("code_review.AutoBypass = false, want true")
	}
}

// TestLoadOwnership_InvalidYAML verifies LoadOwnership returns an error when
// the file is unreadable or the YAML is malformed.
func TestLoadOwnership_InvalidYAML(t *testing.T) {
	// Missing file
	if _, err := LoadOwnership("/nonexistent/path/module-ownership.yaml"); err == nil {
		t.Error("expected error for missing file, got nil")
	}

	// Malformed YAML: unclosed quote + bad indent.
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badPath, []byte("modules: [unterminated\n  :\n - oops"), 0o644); err != nil {
		t.Fatalf("write bad yaml: %v", err)
	}
	if _, err := LoadOwnership(badPath); err == nil {
		t.Error("expected error for malformed yaml, got nil")
	}
}

// TestLoadOwnership_EmptyFile verifies LoadOwnership handles an empty file
// (zero modules) without error.
func TestLoadOwnership_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	emptyPath := filepath.Join(dir, "empty.yaml")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatalf("write empty yaml: %v", err)
	}
	owner, err := LoadOwnership(emptyPath)
	if err != nil {
		t.Fatalf("LoadOwnership on empty file returned error: %v", err)
	}
	if owner == nil {
		t.Fatal("LoadOwnership returned nil for empty file")
	}
	if len(owner.Modules) != 0 {
		t.Errorf("expected 0 modules for empty file, got %d", len(owner.Modules))
	}
}
