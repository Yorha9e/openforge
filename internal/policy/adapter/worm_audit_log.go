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

func (l *AuditLogger) Log(ctx context.Context, entry AuditEntry) error {
	content := fmt.Sprintf("%s|%s|%s|%s|%s", entry.Actor, entry.Action, entry.Resource, entry.Result, time.Now().UTC())
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
	prevHash := l.getLastHash(ctx)

	// Write through the dedicated writer connection (of_audit_writer role).
	// The app user openforge no longer has INSERT on audit_log; this is the
	// only path that can append to the chain.
	_, err := l.writer.ExecContext(ctx, `
        INSERT INTO audit_log (event, actor, action, resource, result, project_id, source_ip, user_agent, artifact_hash, prev_hash, content_hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, entry.Event, entry.Actor, entry.Action, entry.Resource, entry.Result,
		entry.ProjectID, entry.SourceIP, entry.UserAgent, entry.ArtifactHash, prevHash, contentHash)
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
