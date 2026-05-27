package compliance

import (
	"context"
	"database/sql"
	"log/slog"
	"time"
)

// DataLifecycle manages the lifecycle of audit log data.
// It runs a background goroutine that periodically cleans up old audit logs.
type DataLifecycle struct {
	db     *sql.DB
	stopCh chan struct{}
	done   chan struct{}
}

// NewDataLifecycle creates a new DataLifecycle instance.
func NewDataLifecycle(db *sql.DB) *DataLifecycle {
	return &DataLifecycle{
		db:     db,
		stopCh: make(chan struct{}),
		done:   make(chan struct{}),
	}
}

// Start begins the background cleanup goroutine.
// It runs every 24 hours and deletes audit logs older than 365 days.
func (d *DataLifecycle) Start() {
	go func() {
		defer close(d.done)

		// Run cleanup immediately on start
		d.cleanup()

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				d.cleanup()
			case <-d.stopCh:
				slog.Info("data lifecycle stopped")
				return
			}
		}
	}()

	slog.Info("data lifecycle started", "interval", "24h", "retention", "365 days")
}

// Stop signals the background goroutine to stop and waits for it to finish.
func (d *DataLifecycle) Stop() {
	close(d.stopCh)
	<-d.done
}

// cleanup deletes audit logs older than 365 days.
func (d *DataLifecycle) cleanup() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	query := `DELETE FROM audit_log WHERE created_at < NOW() - INTERVAL '365 days'`

	result, err := d.db.ExecContext(ctx, query)
	if err != nil {
		slog.Error("failed to cleanup old audit logs", "error", err)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		slog.Error("failed to get rows affected", "error", err)
		return
	}

	if rowsAffected > 0 {
		slog.Info("cleaned up old audit logs", "rows_deleted", rowsAffected)
	}
}

// Stats returns statistics about the audit log data.
func (d *DataLifecycle) Stats(ctx context.Context) (DataLifecycleStats, error) {
	var stats DataLifecycleStats

	// Get total count
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log").Scan(&stats.TotalRecords)
	if err != nil {
		return stats, err
	}

	// Get oldest record
	err = d.db.QueryRowContext(ctx, "SELECT MIN(created_at) FROM audit_log").Scan(&stats.OldestRecord)
	if err != nil && err != sql.ErrNoRows {
		return stats, err
	}

	// Get newest record
	err = d.db.QueryRowContext(ctx, "SELECT MAX(created_at) FROM audit_log").Scan(&stats.NewestRecord)
	if err != nil && err != sql.ErrNoRows {
		return stats, err
	}

	// Get count of records older than 365 days
	err = d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM audit_log WHERE created_at < NOW() - INTERVAL '365 days'").Scan(&stats.RecordsOlderThan365Days)
	if err != nil {
		return stats, err
	}

	return stats, nil
}

// DataLifecycleStats contains statistics about audit log data.
type DataLifecycleStats struct {
	TotalRecords          int64      `json:"total_records"`
	OldestRecord          *time.Time `json:"oldest_record,omitempty"`
	NewestRecord          *time.Time `json:"newest_record,omitempty"`
	RecordsOlderThan365Days int64    `json:"records_older_than_365_days"`
}