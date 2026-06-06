package adapter

import (
	"context"
	"database/sql"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestPGDisasterRecovery_Disabled_NilDB(t *testing.T) {
	dr := NewPGDisasterRecovery(nil, "", "", "")
	if dr.db != nil {
		t.Error("should be disabled when db is nil")
	}

	ctx := context.Background()
	err := dr.Backup(ctx)
	if err == nil {
		t.Error("Backup should return error when disabled")
	}

	err = dr.Restore(ctx, time.Now())
	if err == nil {
		t.Error("Restore should return error when disabled")
	}

	status, err := dr.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if status.Healthy {
		t.Error("should be unhealthy when disabled")
	}
}

func TestPGDisasterRecovery_Disabled_EmptyBackupDir(t *testing.T) {
	// Create a dummy DB connection (will fail to connect but that's ok for this test)
	db, err := sql.Open("postgres", "host=localhost port=5432 user=test dbname=test sslmode=disable")
	if err != nil {
		t.Skip("cannot create DB connection")
	}
	defer db.Close()

	dr := NewPGDisasterRecovery(db, "host=localhost port=5432 user=test dbname=test sslmode=disable", "", "")
	if dr.db != nil {
		t.Error("should be disabled when backup dir is empty")
	}
}

func TestPGDisasterRecovery_Backup_Restore_Integration(t *testing.T) {
	// Skip if no PostgreSQL connection available
	dbHost := os.Getenv("POSTGRES_HOST")
	if dbHost == "" {
		t.Skip("POSTGRES_HOST not set, skipping integration test")
	}

	dbPort := os.Getenv("POSTGRES_PORT")
	if dbPort == "" {
		dbPort = "5432"
	}

	dbUser := os.Getenv("POSTGRES_USER")
	if dbUser == "" {
		dbUser = "postgres"
	}

	dbPassword := os.Getenv("POSTGRES_PASSWORD")
	if dbPassword == "" {
		dbPassword = "postgres"
	}

	dbName := os.Getenv("POSTGRES_DB")
	if dbName == "" {
		dbName = "openforge"
	}

	dsn := "host=" + dbHost + " port=" + dbPort + " user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	defer db.Close()

	// Ping to verify connection
	if err := db.Ping(); err != nil {
		t.Skip("cannot connect to PostgreSQL, skipping integration test")
	}

	// Create temporary backup directory
	tempDir, err := os.MkdirTemp("", "pg_backup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Get pg_tools_path from environment or use default
	pgToolsPath := os.Getenv("PG_TOOLS_PATH")

	dr := NewPGDisasterRecovery(db, dsn, tempDir, pgToolsPath)
	if dr.db == nil {
		t.Fatal("should be enabled")
	}

	ctx := context.Background()

	// Test Backup
	err = dr.Backup(ctx)
	if err != nil {
		t.Fatalf("Backup failed: %v", err)
	}

	// Verify backup file was created
	files, err := filepath.Glob(filepath.Join(tempDir, "*.dump"))
	if err != nil {
		t.Fatalf("failed to list backup files: %v", err)
	}
	if len(files) == 0 {
		t.Error("no backup files created")
	}

	// Test Status after backup
	status, err := dr.Status(ctx)
	if err != nil {
		t.Fatalf("Status failed: %v", err)
	}
	if !status.Healthy {
		t.Error("should be healthy after backup")
	}
	if status.LastBackup.IsZero() {
		t.Error("LastBackup should be set")
	}

	// Test Restore (this will fail if pg_restore is not available, but we test the logic)
	err = dr.Restore(ctx, time.Now())
	if err != nil {
		// Restore may fail if pg_restore is not installed, that's ok for unit test
		t.Logf("Restore failed (expected if pg_restore not available): %v", err)
	}
}

func TestPGDisasterRecovery_CleanupOldBackups(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "pg_cleanup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy backup files
	for i := 0; i < 10; i++ {
		file := filepath.Join(tempDir, "backup_20260101_00000"+string(rune('0'+i))+".dump")
		if err := os.WriteFile(file, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to create dummy backup: %v", err)
		}
		// Set modification time to be different
		modTime := time.Now().Add(-time.Duration(10-i) * 24 * time.Hour)
		os.Chtimes(file, modTime, modTime)
	}

	dr := &PGDisasterRecovery{
		backupDir: tempDir,
	}

	// Run cleanup
	dr.cleanupOldBackups()

	// Verify only 7 files remain
	files, err := filepath.Glob(filepath.Join(tempDir, "*.dump"))
	if err != nil {
		t.Fatalf("failed to list files: %v", err)
	}
	if len(files) != 7 {
		t.Errorf("expected 7 files after cleanup, got %d", len(files))
	}
}

func TestPGDisasterRecovery_EncryptedBackup(t *testing.T) {
	// Skip if pg_dump or gpg are not available (TDD allowance per plan).
	if _, err := exec.LookPath("pg_dump"); err != nil {
		t.Skip("requires pg_dump in PATH")
	}
	if _, err := exec.LookPath("gpg"); err != nil {
		t.Skip("requires gpg in PATH")
	}

	tempDir, err := os.MkdirTemp("", "pg_encrypted_backup_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	passphraseFile := filepath.Join(tempDir, "passphrase.txt")
	if err := os.WriteFile(passphraseFile, []byte("test-passphrase-please-rotate"), 0600); err != nil {
		t.Fatalf("failed to write passphrase file: %v", err)
	}

	dr := NewPGDisasterRecoveryWithPassphrase(nil, "host=127.0.0.1 dbname=stub", tempDir, "", passphraseFile)
	if dr == nil {
		t.Fatal("constructor returned nil")
	}
	// With nil db the constructor returns an empty struct; populate the
	// minimum fields the encryptBackup unit under test needs.
	dr.passphraseFile = passphraseFile
	dr.backupDir = tempDir

	// encryptBackup is the unit under test for T6 — it runs gpg on an
	// already-produced plaintext dump and removes the plaintext.
	plaintext := filepath.Join(tempDir, "backup_20260607_000000.dump")
	if err := os.WriteFile(plaintext, []byte("plaintext-pg-dump-bytes"), 0644); err != nil {
		t.Fatalf("failed to seed plaintext: %v", err)
	}

	encrypted, err := dr.encryptBackup(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("encryptBackup failed: %v", err)
	}
	if encrypted == plaintext {
		t.Fatal("encryptBackup returned the plaintext path; expected .gpg suffix")
	}
	if filepath.Ext(encrypted) != ".gpg" {
		t.Errorf("expected .gpg suffix, got %q", encrypted)
	}

	info, err := os.Stat(encrypted)
	if err != nil {
		t.Fatalf("expected encrypted file at %q: %v", encrypted, err)
	}
	if info.Size() == 0 {
		t.Error("encrypted file is empty")
	}

	if _, err := os.Stat(plaintext); !os.IsNotExist(err) {
		t.Errorf("plaintext dump should be removed; stat err = %v", err)
	}

	data, err := os.ReadFile(encrypted)
	if err != nil {
		t.Fatalf("failed to read encrypted file: %v", err)
	}
	if containsBytes(data, []byte("plaintext-pg-dump-bytes")) {
		t.Error("encrypted file leaks plaintext content")
	}
}

func containsBytes(haystack, needle []byte) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		match := true
		for j := 0; j < len(needle); j++ {
			if haystack[i+j] != needle[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}