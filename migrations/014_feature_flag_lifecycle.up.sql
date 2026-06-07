-- Feature flag lifecycle: ownership, expiration, status, rollout percentage.
-- All columns are added with sensible defaults so existing rows remain valid.

ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS owner TEXT NOT NULL DEFAULT 'system';
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT NOW();
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS status TEXT NOT NULL DEFAULT 'experimental'
    CHECK (status IN ('experimental', 'beta', 'stable', 'deprecated', 'expired'));
ALTER TABLE feature_flags ADD COLUMN IF NOT EXISTS rollout_pct INT NOT NULL DEFAULT 0
    CHECK (rollout_pct BETWEEN 0 AND 100);
CREATE INDEX IF NOT EXISTS idx_feature_flags_expires ON feature_flags(expires_at) WHERE status != 'expired';
