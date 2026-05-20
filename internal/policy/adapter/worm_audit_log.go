package adapter

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
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
	prevHash := "genesis"

	_, err := l.db.ExecContext(ctx, `
        INSERT INTO audit_log (event, actor, action, resource, result, project_id, source_ip, user_agent, artifact_hash, prev_hash, content_hash)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
    `, entry.Event, entry.Actor, entry.Action, entry.Resource, entry.Result,
		entry.ProjectID, entry.SourceIP, entry.UserAgent, entry.ArtifactHash, prevHash, contentHash)
	return err
}
