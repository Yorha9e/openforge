-- migrations/016_audit_log_project_id.up.sql
-- Item #16: audit_log 加 project_id + 复合索引
-- 用于按 project 维度查询审计日志（合规/取证/排错）
ALTER TABLE audit_log ADD COLUMN IF NOT EXISTS project_id UUID;
CREATE INDEX IF NOT EXISTS idx_audit_log_project_time ON audit_log(project_id, created_at DESC) WHERE project_id IS NOT NULL;
