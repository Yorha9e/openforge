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
	db *sql.DB
}

func NewAuditLogger(db *sql.DB) *AuditLogger {
	return &AuditLogger{db: db}
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

	_, err := l.db.ExecContext(ctx, `
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
	err := l.db.QueryRowContext(ctx, `SELECT content_hash FROM audit_log ORDER BY created_at DESC LIMIT 1`).Scan(&hash)
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
