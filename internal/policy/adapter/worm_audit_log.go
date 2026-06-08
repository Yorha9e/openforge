package adapter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

type AuditLogger struct {
	// reader is used for read-only queries (e.g. VerifyChain/ScanFullChain in
	// T5). It is also the connection used by getLastHash. T4 splits the
	// writer to enforce that the app user can no longer mutate audit_log.
	reader *sql.DB
	// writer is the dedicated INSERT-only connection (uses the
	// of_audit_writer role). For backward compatibility it falls back to
	// reader when constructed via NewAuditLogger.
	writer *sql.DB
}

// NewAuditLogger constructs a single-connection audit logger. Both the read
// and write sides share the same *sql.DB. This is the legacy path; production
// should prefer NewWithWriter so the writer runs as of_audit_writer and the
// app user (openforge) cannot mutate audit_log even if compromised.
func NewAuditLogger(db *sql.DB) *AuditLogger {
	return &AuditLogger{reader: db, writer: db}
}

// NewWithWriter constructs an audit logger with separated reader and writer
// connections. The writer is expected to be connected as a role that holds
// only INSERT on audit_log (see migrations/012_audit_log_revoke.up.sql). When
// writer is nil it falls back to reader, preserving single-DSN behavior.
func NewWithWriter(reader, writer *sql.DB) *AuditLogger {
	if writer == nil {
		writer = reader
	}
	return &AuditLogger{reader: reader, writer: writer}
}

type AuditEntry struct {
	Event        string
	Actor        string
	Action       string
	Resource     string
	Result       string
	ProjectID    string
	SourceIP     string
	UserAgent    string
	ArtifactHash string
}

// computeContentHash returns the SHA256 hash of prevHash concatenated with the
// content string. Including prevHash means tampering with any prior row breaks
// the chain at that point — the hash of row N+1 is no longer reproducible.
//
// This is the single source of truth for chain hashing: both Log (write) and
// VerifyChain (read) must call it. Splitting the two paths would allow silent
// drift.
func (l *AuditLogger) computeContentHash(prevHash, content string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content)))
}

// contentString is the canonical, reproducible representation of a row's
// chainable fields. VerifyChain rebuilds this from the DB columns; Log must
// build it from the same fields using the same time that ends up stored in
// created_at (so the hash can be reproduced from the row alone).
//
// The timestamp is rounded to microsecond precision before formatting
// because PostgreSQL's TIMESTAMPTZ stores microseconds, not nanoseconds. If
// we used time.Now()'s full nanosecond precision in Log, the value embedded
// in the content hash would have ~3 extra digits that the round-tripped
// created_at (read back from PG) does not, and the recomputed hash would
// never match.
func contentString(actor, action, resource, result string, t time.Time) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", actor, action, resource, result, t.UTC().Round(time.Microsecond))
}

func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error {
	prevHash := l.getLastHash(ctx)
	// Use a single time value for both the content hash and the stored
	// created_at. This is what makes VerifyChain reproducible: the only way
	// to recompute content_hash from a DB row is to read created_at back and
	// feed it through the same contentString() function. Round to
	// microsecond precision to match PG's TIMESTAMPTZ storage.
	now := time.Now().UTC().Round(time.Microsecond)
	content := contentString(entry.Actor, entry.Action, entry.Resource, entry.Result, now)
	contentHash := l.computeContentHash(prevHash, content)

	// Write through the dedicated writer connection (of_audit_writer role).
	// The app user openforge no longer has INSERT on audit_log after T4's
	// REVOKE migration; this is the only path that can append to the chain.
	// We pass created_at explicitly so the stored timestamp matches the one
	// baked into content_hash byte-for-byte.
	_, err := l.writer.ExecContext(ctx, `
        INSERT INTO audit_log (event, actor, action, resource, result, project_id, source_ip, user_agent, artifact_hash, prev_hash, content_hash, created_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
    `, entry.Event, entry.Actor, entry.Action, entry.Resource, entry.Result,
		entry.ProjectID, entry.SourceIP, entry.UserAgent, entry.ArtifactHash, prevHash, contentHash, now)
	return err
}

// getLastHash returns the most recent content_hash from the audit log,
// or an empty string for the first entry in the chain.
func (l *AuditLogger) getLastHash(ctx context.Context) string {
	var hash string
	err := l.reader.QueryRowContext(ctx, `SELECT content_hash FROM audit_log ORDER BY created_at DESC LIMIT 1`).Scan(&hash)
	if err != nil {
		return ""
	}
	return hash
}

// VerifyChain re-walks the prev_hash/content_hash chain for the audit_log
// rows that fall inside the given year-month partition (format "2006-01").
// Any mismatch in either prev_hash linkage (for rows past the first) or
// content_hash recomputation is returned as an error and identifies the
// offending event/row.
//
// The first row's prev_hash is treated as a black-box genesis pointer — we
// accept whatever it points to (a prior partition's head, or the empty
// string for a fresh chain) and use it as the running prevHash. This lets
// verification succeed when the partition contains rows that link to a
// predecessor outside the partition, which is the common case for monthly
// partitions of a continuously-growing chain.
//
// Uses the T4-split reader connection (which runs as the openforge app user
// holding only SELECT on audit_log). This is exactly the read path that the
// production deployment should use — no privilege escalation needed to
// verify the chain.
func (l *AuditLogger) VerifyChain(ctx context.Context, yearMonth string) error {
	start, err := time.Parse("2006-01", yearMonth)
	if err != nil {
		return fmt.Errorf("VerifyChain: parse %q: %w", yearMonth, err)
	}
	const q = `
        SELECT event, actor, action, resource, result, prev_hash, content_hash, created_at
        FROM audit_log
        WHERE created_at >= $1 AND created_at < $1::date + INTERVAL '1 month'
        ORDER BY created_at ASC, id ASC
    `
	rows, err := l.reader.QueryContext(ctx, q, start)
	if err != nil {
		return fmt.Errorf("VerifyChain: query %s: %w", yearMonth, err)
	}
	defer rows.Close()

	// prevHash starts as the empty-string genesis value. We seed it with
	// the first row's stored prev_hash so the chain check is relative to
	// that row's own predecessor, not to a hard-coded empty string.
	var prevHash string
	first := true
	for rows.Next() {
		var (
			event, actor, action, resource, result string
			storedPrev, storedHash                 string
			createdAt                              time.Time
		)
		if err := rows.Scan(&event, &actor, &action, &resource, &result, &storedPrev, &storedHash, &createdAt); err != nil {
			return fmt.Errorf("VerifyChain: scan: %w", err)
		}
		if first {
			prevHash = storedPrev
			first = false
		} else if storedPrev != prevHash {
			return fmt.Errorf("VerifyChain: %s: prev_hash mismatch (expected %q got %q)", event, prevHash, storedPrev)
		}
		content := contentString(actor, action, resource, result, createdAt)
		recomputed := l.computeContentHash(prevHash, content)
		if recomputed != storedHash {
			return fmt.Errorf("VerifyChain: %s: content_hash mismatch (expected %q got %q)", event, recomputed, storedHash)
		}
		prevHash = storedHash
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("VerifyChain: rows: %w", err)
	}
	return nil
}

// ScanFullChain walks the last three calendar months (current, prev, prev-1)
// and aggregates any partition that fails VerifyChain into a single error.
// This is what the hourly ticker in bootstrap.go calls.
func (l *AuditLogger) ScanFullChain(ctx context.Context) error {
	now := time.Now().UTC()
	var errs []string
	for i := 0; i < 3; i++ {
		ym := now.AddDate(0, -i, 0).Format("2006-01")
		if err := l.VerifyChain(ctx, ym); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", ym, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("ScanFullChain: %d partition(s) failed: %v", len(errs), errs)
	}
	return nil
}

// HashChain provides an in-memory content-addressable chain for testing.
type HashChain struct {
	mu       sync.Mutex
	prevHash string
}

func NewHashChain() *HashChain { return &HashChain{} }

func (c *HashChain) Next(content string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	h := fmt.Sprintf("%x", sha256.Sum256([]byte(c.prevHash+content)))
	c.prevHash = h
	return h
}

func (c *HashChain) Verify(content, prevHash, expectedHash string) bool {
	computed := fmt.Sprintf("%x", sha256.Sum256([]byte(prevHash+content)))
	return computed == expectedHash
}

func (c *HashChain) CurrentPrev() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.prevHash
}
