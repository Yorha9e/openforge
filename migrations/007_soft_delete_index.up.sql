-- Index for efficient soft-delete cleanup queries
CREATE INDEX IF NOT EXISTS idx_project_deleted_at ON project(deleted_at) WHERE deleted_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pipeline_deleted_at ON pipeline(deleted_at) WHERE deleted_at IS NOT NULL;
