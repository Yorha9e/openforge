package server

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// SoftDeleteCleaner periodically hard-deletes soft-deleted projects
// and their cascaded pipelines after the retention window (30 days).
type SoftDeleteCleaner struct {
	db     *sql.DB
	stopCh chan struct{}
	done   chan struct{}
}

// NewSoftDeleteCleaner creates a new SoftDeleteCleaner.
func NewSoftDeleteCleaner(db *sql.DB) *SoftDeleteCleaner {
	return &SoftDeleteCleaner{
		db:     db,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the background cleanup goroutine.
// It runs every 24 hours and hard-deletes projects where
// deleted_at < NOW() - 30 days, along with their cascaded pipelines.
func (c *SoftDeleteCleaner) Start() {
	go func() {
		defer close(c.done)

		// Run cleanup immediately on start
		c.cleanup()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.cleanup()
			case <-c.stopCh:
				slog.Info("soft delete cleaner stopped")
				return
			}
		}
	}()

	slog.Info("soft delete cleaner started",
		"interval", "24h",
		"retention", "30 days",
	)
}

// Stop signals the background goroutine to stop and waits for it to finish.
func (c *SoftDeleteCleaner) Stop() {
	close(c.stopCh)
	<-c.done
}

// cleanup hard-deletes projects and their cascaded data older than 30 days.
func (c *SoftDeleteCleaner) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	tx, err := c.db.BeginTx(ctx, nil)
	if err != nil {
		slog.Error("soft delete cleanup: failed to begin transaction", "error", err)
		return
	}
	defer tx.Rollback()

	// Find projects past retention window
	rows, err := tx.QueryContext(ctx,
		`SELECT id FROM project
		 WHERE deleted_at IS NOT NULL
		   AND deleted_at < NOW() - INTERVAL '30 days'`)
	if err != nil {
		slog.Error("soft delete cleanup: query failed", "error", err)
		return
	}
	defer rows.Close()

	var projectIDs []string
	for rows.Next() {
		var pid string
		if err := rows.Scan(&pid); err != nil {
			slog.Error("soft delete cleanup: scan failed", "error", err)
			return
		}
		projectIDs = append(projectIDs, pid)
	}

	if len(projectIDs) == 0 {
		return
	}

	// Hard-delete cascaded data for each expired project
	for _, pid := range projectIDs {
		// Delete conversation data linked through pipelines
		_, err = tx.ExecContext(ctx, `
			DELETE FROM conversation_message
			WHERE pipeline_id IN (
				SELECT id FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL
			)`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: conversation_message", "project_id", pid, "error", err)
			return
		}

		_, err = tx.ExecContext(ctx, `
			DELETE FROM conversation_branch
			WHERE pipeline_id IN (
				SELECT id FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL
			)`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: conversation_branch", "project_id", pid, "error", err)
			return
		}

		_, err = tx.ExecContext(ctx, `
			DELETE FROM checkpoint
			WHERE pipeline_id IN (
				SELECT id FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL
			)`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: checkpoint", "project_id", pid, "error", err)
			return
		}

		_, err = tx.ExecContext(ctx, `
			DELETE FROM pipeline_stage
			WHERE pipeline_id IN (
				SELECT id FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL
			)`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: pipeline_stage", "project_id", pid, "error", err)
			return
		}

		_, err = tx.ExecContext(ctx, `
			DELETE FROM gate_event
			WHERE pipeline_id IN (
				SELECT id FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL
			)`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: gate_event", "project_id", pid, "error", err)
			return
		}

		// Delete soft-deleted pipelines for this project
		_, err = tx.ExecContext(ctx,
			`DELETE FROM pipeline WHERE project_id = $1 AND deleted_at IS NOT NULL`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: pipeline", "project_id", pid, "error", err)
			return
		}

		// Delete file locks
		_, err = tx.ExecContext(ctx,
			`DELETE FROM file_lock WHERE project_id = $1`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: file_lock", "project_id", pid, "error", err)
			return
		}

		// Delete the project itself
		_, err = tx.ExecContext(ctx,
			`DELETE FROM project WHERE id = $1 AND deleted_at IS NOT NULL`, pid)
		if err != nil {
			slog.Error("soft delete cleanup: project", "project_id", pid, "error", err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("soft delete cleanup: commit failed", "error", err)
		return
	}

	slog.Info("soft delete cleanup completed",
		"projects_hard_deleted", len(projectIDs),
	)
}

// StartSoftDeleteCleaner creates and starts a SoftDeleteCleaner using the given DB.
// It is called from RegisterRoutes at server startup.
func StartSoftDeleteCleaner(db *sql.DB) {
	cleaner := NewSoftDeleteCleaner(db)
	cleaner.Start()
}
