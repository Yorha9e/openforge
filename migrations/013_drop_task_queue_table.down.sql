-- 013_drop_task_queue_table.down.sql
-- Restore task_queue to its Phase 1 schema (matches migrations/001_init.up.sql §9).
-- No data backfill — this is a structural rollback only.

CREATE TABLE task_queue (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL,
    project_id      TEXT NOT NULL,
    task_type       VARCHAR(32) NOT NULL CHECK (task_type IN ('llm_request','sandbox_run','notification')),
    priority        INT NOT NULL DEFAULT 2 CHECK (priority >= 0 AND priority <= 3),
    payload         JSONB NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','claimed','running','completed','failed')),
    claimed_by      VARCHAR(64),
    claimed_at      TIMESTAMPTZ,
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    retry_count     INT NOT NULL DEFAULT 0,
    max_retries     INT NOT NULL DEFAULT 3,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_task_queue_dequeue ON task_queue(status, priority DESC, created_at DESC)
    WHERE status = 'pending';
