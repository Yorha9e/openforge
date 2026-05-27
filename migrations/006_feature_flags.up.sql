-- Feature flags: Admin-togglable enterprise capabilities.
-- Defaults are set by profile YAML; DB rows override on startup and at runtime.

CREATE TABLE IF NOT EXISTS feature_flags (
    flag_key    TEXT        PRIMARY KEY,
    enabled     BOOLEAN     NOT NULL DEFAULT false,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE feature_flags IS 'Runtime-overridable feature toggles for enterprise capabilities';
COMMENT ON COLUMN feature_flags.flag_key IS 'Flag identifier: enterprise_platform, compliance_suite, production_ops, distribution_artifacts';
COMMENT ON COLUMN feature_flags.enabled IS 'Whether the feature group is currently active';
