-- Revert feature flag lifecycle columns.
DROP INDEX IF EXISTS idx_feature_flags_expires;
ALTER TABLE feature_flags DROP COLUMN IF EXISTS rollout_pct;
ALTER TABLE feature_flags DROP COLUMN IF EXISTS status;
ALTER TABLE feature_flags DROP COLUMN IF EXISTS expires_at;
ALTER TABLE feature_flags DROP COLUMN IF EXISTS created_at;
ALTER TABLE feature_flags DROP COLUMN IF EXISTS owner;
