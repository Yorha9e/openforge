package adapter

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"openforge/internal/shared/kernel"
)

// PGDisasterRecovery implements kernel.DisasterRecovery using pg_dump/pg_restore.
type PGDisasterRecovery struct {
	db          *sql.DB
	dsn         string
	backupDir   string
	pgToolsPath string // G13: configurable pg_dump path
	lastBackup  time.Time
	lastRestore time.Time
	mu          sync.RWMutex
}

// NewPGDisasterRecovery creates a new PostgreSQL disaster recovery handler.
// pgToolsPath is the directory containing pg_dump and pg_restore binaries.
// If empty, assumes they are in PATH.
func NewPGDisasterRecovery(db *sql.DB, dsn, backupDir, pgToolsPath string) *PGDisasterRecovery {
	if db == nil {
		slog.Warn("pg disaster recovery disabled: nil db")
		return &PGDisasterRecovery{}
	}
	if backupDir == "" {
		slog.Warn("pg disaster recovery disabled: empty backup directory")
		return &PGDisasterRecovery{}
	}

	// Ensure backup directory exists
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		slog.Warn("pg disaster recovery: failed to create backup directory", "error", err)
		return &PGDisasterRecovery{}
	}

	slog.Info("pg disaster recovery enabled", "backup_dir", backupDir, "pg_tools_path", pgToolsPath)
	return &PGDisasterRecovery{
		db:          db,
		dsn:         dsn,
		backupDir:   backupDir,
		pgToolsPath: pgToolsPath,
	}
}

// Backup creates a new PostgreSQL backup using pg_dump.
func (p *PGDisasterRecovery) Backup(ctx context.Context) error {
	if p.db == nil {
		return fmt.Errorf("pg disaster recovery is disabled")
	}

	pgDump := p.pgToolsPath + "/pg_dump"
	if p.pgToolsPath == "" {
		pgDump = "pg_dump"
	}

	// Generate timestamp-based filename
	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(p.backupDir, fmt.Sprintf("backup_%s.dump", timestamp))

	// Create backup using pg_dump -Fc (custom format)
	cmd := exec.CommandContext(ctx, pgDump, "-Fc", "-f", backupFile, p.dsn)
	cmd.Env = append(os.Environ(), "PGSSLMODE=disable") // Simplify for dev

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("pg_dump failed", "error", err, "output", string(output))
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	p.mu.Lock()
	p.lastBackup = time.Now()
	p.mu.Unlock()

	slog.Info("pg backup completed", "file", backupFile, "size", getFileSize(backupFile))

	// Cleanup old backups (keep last 7)
	p.cleanupOldBackups()

	return nil
}

// Restore restores from the most recent backup.
func (p *PGDisasterRecovery) Restore(ctx context.Context, point time.Time) error {
	if p.db == nil {
		return fmt.Errorf("pg disaster recovery is disabled")
	}

	// Find the most recent backup file
	backupFile, err := p.findLatestBackup()
	if err != nil {
		return fmt.Errorf("no backup found: %w", err)
	}

	pgRestore := p.pgToolsPath + "/pg_restore"
	if p.pgToolsPath == "" {
		pgRestore = "pg_restore"
	}

	// Restore using pg_restore
	cmd := exec.CommandContext(ctx, pgRestore, "-d", p.dsn, backupFile)
	cmd.Env = append(os.Environ(), "PGSSLMODE=disable")

	output, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("pg_restore failed", "error", err, "output", string(output))
		return fmt.Errorf("pg_restore failed: %w", err)
	}

	p.mu.Lock()
	p.lastRestore = time.Now()
	p.mu.Unlock()

	slog.Info("pg restore completed", "file", backupFile)
	return nil
}

// Status returns the current disaster recovery status.
func (p *PGDisasterRecovery) Status(ctx context.Context) (kernel.DRStatus, error) {
	if p.db == nil {
		return kernel.DRStatus{Healthy: false}, nil
	}

	p.mu.RLock()
	lastBackup := p.lastBackup
	lastRestore := p.lastRestore
	p.mu.RUnlock()

	// Check if we have a recent backup (within 24 hours)
	healthy := false
	if !lastBackup.IsZero() {
		healthy = time.Since(lastBackup) < 24*time.Hour
	}

	// Also check if backup files exist
	if !healthy {
		backupFiles, _ := p.listBackupFiles()
		if len(backupFiles) > 0 {
			// Check if latest backup is within 24 hours
			latest := backupFiles[len(backupFiles)-1]
			info, err := os.Stat(latest)
			if err == nil {
				healthy = time.Since(info.ModTime()) < 24*time.Hour
				if healthy && lastBackup.IsZero() {
					lastBackup = info.ModTime()
				}
			}
		}
	}

	return kernel.DRStatus{
		Healthy:     healthy,
		LastBackup:  lastBackup,
		LastRestore: lastRestore,
	}, nil
}

// findLatestBackup finds the most recent backup file in the backup directory.
func (p *PGDisasterRecovery) findLatestBackup() (string, error) {
	files, err := p.listBackupFiles()
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no backup files found in %s", p.backupDir)
	}
	return files[len(files)-1], nil
}

// listBackupFiles returns all .dump files sorted by modification time.
func (p *PGDisasterRecovery) listBackupFiles() ([]string, error) {
	entries, err := os.ReadDir(p.backupDir)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".dump") {
			files = append(files, filepath.Join(p.backupDir, entry.Name()))
		}
	}

	// Sort by modification time
	sort.Slice(files, func(i, j int) bool {
		iInfo, _ := os.Stat(files[i])
		jInfo, _ := os.Stat(files[j])
		if iInfo == nil || jInfo == nil {
			return false
		}
		return iInfo.ModTime().Before(jInfo.ModTime())
	})

	return files, nil
}

// cleanupOldBackups keeps only the last 7 backup files.
func (p *PGDisasterRecovery) cleanupOldBackups() {
	files, err := p.listBackupFiles()
	if err != nil {
		return
	}

	// Keep only last 7 files
	if len(files) > 7 {
		for _, file := range files[:len(files)-7] {
			if err := os.Remove(file); err != nil {
				slog.Warn("failed to remove old backup", "file", file, "error", err)
			} else {
				slog.Info("removed old backup", "file", file)
			}
		}
	}
}

// getFileSize returns the size of a file in bytes.
func getFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Close closes the database connection (if owned by this instance).
func (p *PGDisasterRecovery) Close() error {
	// We don't close the DB here as it's shared with the main application
	return nil
}

// Verify that PGDisasterRecovery implements kernel.DisasterRecovery.
var _ kernel.DisasterRecovery = (*PGDisasterRecovery)(nil)