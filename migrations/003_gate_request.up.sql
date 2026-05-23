-- 003_gate_request.up.sql — Gate request tracking table

CREATE TABLE IF NOT EXISTS gate_request (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id TEXT NOT NULL REFERENCES pipeline(id),
    stage VARCHAR(8) NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'approved', 'rejected', 'timeout', 'cancelled')),
    requested_by TEXT NOT NULL,
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    result JSONB,
    timeout_at TIMESTAMPTZ NOT NULL DEFAULT (NOW() + INTERVAL '5 minutes'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gate_request_pipeline ON gate_request(pipeline_id, stage);
CREATE INDEX IF NOT EXISTS idx_gate_request_pending ON gate_request(status) WHERE status = 'pending';
