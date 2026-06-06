package adapter

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestAuditEntry_ProjectIDField verifies the AuditEntry struct exposes a ProjectID
// field so the audit_log.project_id column can be populated (item #16).
func TestAuditEntry_ProjectIDField(t *testing.T) {
	entry := AuditEntry{
		Event:     "gate.request",
		Actor:     "user@example.com",
		Action:    "approve",
		Resource:  "project:abc",
		Result:    "success",
		ProjectID: "11111111-2222-3333-4444-555555555555",
	}
	if entry.ProjectID == "" {
		t.Fatal("ProjectID field is empty — would be skipped by INSERT")
	}
	if got := entry.ProjectID; got != "11111111-2222-3333-4444-555555555555" {
		t.Errorf("ProjectID = %q, want UUID", got)
	}
}

// TestAuditEntry_HasProjectIDField ensures the struct field name didn't drift
// away from "ProjectID" — the Log() INSERT references it by name.
func TestAuditEntry_HasProjectIDField(t *testing.T) {
	typ := reflect.TypeOf(AuditEntry{})
	field, ok := typ.FieldByName("ProjectID")
	if !ok {
		t.Fatal("AuditEntry is missing ProjectID field")
	}
	if field.Type.Kind() != reflect.String {
		t.Errorf("AuditEntry.ProjectID kind = %s, want string", field.Type.Kind())
	}
}

// TestLog_IncludesProjectIDInInsert is a structural test that ensures the Log
// SQL statement contains both the project_id column and the corresponding
// positional parameter binding. This guards against future refactors silently
// dropping project_id from the INSERT (item #16).
func TestLog_IncludesProjectIDInInsert(t *testing.T) {
	// Locate the worm_audit_log.go source via runtime.Caller.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src, err := os.ReadFile(filepath.Join(filepath.Dir(thisFile), "worm_audit_log.go"))
	if err != nil {
		t.Fatalf("read worm_audit_log.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "INSERT INTO audit_log") {
		t.Error("expected INSERT INTO audit_log in worm_audit_log.go")
	}
	if !strings.Contains(text, "project_id") {
		t.Error("Log INSERT must reference project_id column")
	}
	if !strings.Contains(text, "entry.ProjectID") {
		t.Error("Log INSERT must bind entry.ProjectID")
	}
}

func TestHashChain_Links(t *testing.T) {
	var dbHashes []string
	record := func(content string) string {
		prev := ""
		if len(dbHashes) > 0 {
			prev = dbHashes[len(dbHashes)-1]
		}
		h := fmt.Sprintf("%x", sha256.Sum256([]byte(prev+content)))
		dbHashes = append(dbHashes, h)
		return h
	}

	h1 := record("event-1")
	h2 := record("event-2")
	h3 := record("event-3")

	expectedPrevForH2 := h1
	expectedH2 := fmt.Sprintf("%x", sha256.Sum256([]byte(h1+"event-2")))
	if h2 != expectedH2 {
		t.Errorf("h2 = %s, want %s", h2, expectedH2)
	}

	expectedH3 := fmt.Sprintf("%x", sha256.Sum256([]byte(h2+"event-3")))
	if h3 != expectedH3 {
		t.Errorf("h3 = %s, want %s\nh2 was: %s", h3, expectedH3, h2)
	}

	// Verify chaining
	if h1 == h2 || h2 == h3 {
		t.Fatal("all hashes identical — chain not working")
	}
	t.Logf("prev for h2 should be: %s", expectedPrevForH2)
}

func TestHashChain_Verify(t *testing.T) {
	chain := NewHashChain()
	h1 := chain.Next("e1")
	h2 := chain.Next("e2")

	if !chain.Verify("e1", "", h1) {
		t.Error("verify e1 failed")
	}
	if !chain.Verify("e2", h1, h2) {
		t.Error("verify e2 failed")
	}
	if chain.Verify("e2", "wrong-prev", h2) {
		t.Error("verify should reject wrong prevHash")
	}
}

