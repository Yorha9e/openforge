DROP INDEX IF EXISTS idx_audit_log_project_time;
ALTER TABLE audit_log DROP COLUMN IF EXISTS project_id;
