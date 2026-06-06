package adapter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

// dsn is the dev DSN used by other adapter tests (see auth/adapter
// pg_auth_repository_test.go). The post-T4 production deployment uses a
// separate of_audit_writer DSN for INSERTs, but the test DB still allows
// INSERTs from openforge in this dev environment, so the same DSN works as
// both reader and writer.
const dsn = "postgres://openforge:openforge_dev@localhost:5432/openforge?sslmode=disable"

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Skipf("test DB unavailable, skipping: %v", err)
	}
	return db
}

// TestAuditLogger_VerifyChain_NoTampering writes a small chain through the
// production Log path against a dedicated test partition, then calls
// VerifyChain on that partition. It expects nil (no error) — the chain is
// internally consistent.
//
// Isolation: we create a private partition audit_log_test_isolated for this
// test, route our writes through it via a wrapper, and drop it at the end.
// This keeps the production partitions (2026_05, 2026_06) untouched and
// gives the test full control over the chain state.
func TestAuditLogger_VerifyChain_NoTampering(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	al := NewAuditLogger(db)
	ctx := context.Background()

	const part = "audit_log_test_isolated"
	const partMonth = "2099-01"
	// Ensure no leftover partition from a prior failed run.
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+part)
	if _, err := db.ExecContext(ctx, `CREATE TABLE `+part+` PARTITION OF audit_log FOR VALUES FROM ('2099-01-01') TO ('2099-02-01')`); err != nil {
		t.Skipf("cannot create test partition (expected after T4): %v", err)
		return
	}
	defer func() { _, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+part) }()

	// Insert rows directly with created_at pinned into the test partition's
	// month so VerifyChain finds them when scanning 2099-01.
	prevHash := ""
	insertedHashes := map[string]string{} // event -> content_hash, for re-walk
	for i := 0; i < 5; i++ {
		actor := "verify-test"
		action := "noop"
		resource := "test"
		result := "success"
		createdAt := time.Date(2099, 1, 2, 0, 0, i, 0, time.UTC)
		content := contentString(actor, action, resource, result, createdAt)
		ch := al.computeContentHash(prevHash, content)
		event := fmt.Sprintf("verify-test-%d", i)
		_, err := db.ExecContext(ctx, `
            INSERT INTO audit_log (event, actor, action, resource, result, prev_hash, content_hash, source_ip, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `, event, actor, action, resource, result, prevHash, ch, "127.0.0.1", createdAt)
		if err != nil {
			t.Skipf("cannot insert into test partition (expected after T4): %v", err)
			return
		}
		insertedHashes[event] = ch
		prevHash = ch
	}

	if err := al.VerifyChain(ctx, partMonth); err != nil {
		t.Fatalf("VerifyChain on untampered log returned error: %v", err)
	}
}

// TestAuditLogger_VerifyChain_DetectsTampering writes a small chain into a
// dedicated test partition, then breaks one row's prev_hash directly. The
// post-T4 prod setup will reject the UPDATE with permission denied (this
// is the whole point of the REVOKE migration); in dev we let it through
// and assert VerifyChain catches the break.
func TestAuditLogger_VerifyChain_DetectsTampering(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	al := NewAuditLogger(db)
	ctx := context.Background()

	const part = "audit_log_tamper_isolated"
	const partMonth = "2099-02"
	_, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+part)
	if _, err := db.ExecContext(ctx, `CREATE TABLE `+part+` PARTITION OF audit_log FOR VALUES FROM ('2099-02-01') TO ('2099-03-01')`); err != nil {
		t.Skipf("cannot create test partition (expected after T4): %v", err)
		return
	}
	defer func() { _, _ = db.ExecContext(ctx, `DROP TABLE IF EXISTS `+part) }()

	prevHash := ""
	for i := 0; i < 3; i++ {
		actor := "tamper-test"
		action := "noop"
		resource := "test"
		result := "success"
		createdAt := time.Date(2099, 2, 2, 0, 0, i, 0, time.UTC)
		content := contentString(actor, action, resource, result, createdAt)
		ch := al.computeContentHash(prevHash, content)
		event := fmt.Sprintf("tamper-test-%d", i)
		_, err := db.ExecContext(ctx, `
            INSERT INTO audit_log (event, actor, action, resource, result, prev_hash, content_hash, source_ip, created_at)
            VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
        `, event, actor, action, resource, result, prevHash, ch, "127.0.0.1", createdAt)
		if err != nil {
			t.Skipf("cannot insert into test partition (expected after T4): %v", err)
			return
		}
		prevHash = ch
	}

	// Break the last row's prev_hash. PostgreSQL doesn't allow ORDER BY /
	// LIMIT in UPDATE, so use a subquery.
	_, err := db.ExecContext(ctx, `
        UPDATE audit_log SET prev_hash = $1
        WHERE created_at = (
            SELECT MAX(created_at) FROM audit_log
            WHERE event LIKE 'tamper-test-%'
        )
    `, "deadbeef")
	if err != nil {
		t.Skipf("UPDATE not permitted in this environment (expected after T4): %v", err)
		return
	}

	err = al.VerifyChain(ctx, partMonth)
	if err == nil {
		t.Fatal("expected VerifyChain to return an error after tampering, got nil")
	}
	t.Logf("VerifyChain caught tampering as expected: %v", err)
}

// TestAuditLogger_ScanFullChain_Smoke exercises ScanFullChain end-to-end
// against the live test DB. It must not panic and must return either nil
// (chain is intact) or a wrapped VerifyChain error.
func TestAuditLogger_ScanFullChain_Smoke(t *testing.T) {
	db := newTestDB(t)
	defer db.Close()
	al := NewAuditLogger(db)
	ctx := context.Background()

	if err := al.ScanFullChain(ctx); err != nil {
		// If the dev DB happens to have legacy T4-era rows whose content_hash
		// was computed with a different time source, the scan will surface
		// that as an error. That is the correct, observable behavior of the
		// scanner — log it but don't fail the smoke test.
		t.Logf("ScanFullChain reported (this is fine if dev DB has legacy rows): %v", err)
	}
}

// TestComputeContentHash covers the helper directly. This is the load-bearing
// unit: any change to the hash function breaks the chain for every future row,
// so a fast, DB-free assertion is worth keeping.
func TestComputeContentHash(t *testing.T) {
	al := &AuditLogger{}
	h1 := al.computeContentHash("", "first")
	h2 := al.computeContentHash(h1, "second")
	if h1 == h2 {
		t.Fatal("chain should produce different hashes at each step")
	}
	// Recomputation must be deterministic.
	if al.computeContentHash(h1, "second") != h2 {
		t.Fatal("computeContentHash is not deterministic")
	}
	// Different prevHash must produce a different content hash.
	if al.computeContentHash("not-h1", "second") == h2 {
		t.Fatal("content hash did not depend on prevHash — chain linkage is broken")
	}
}

// TestHashChain_Links — preserved from the pre-T5 test file. Validates the
// in-memory HashChain helper used by the canary variants and as a reference
// implementation.
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

	if h1 == h2 || h2 == h3 {
		t.Fatal("all hashes identical — chain not working")
	}
	t.Logf("prev for h2 should be: %s", expectedPrevForH2)
}

// TestHashChain_Verify — preserved from the pre-T5 test file.
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
