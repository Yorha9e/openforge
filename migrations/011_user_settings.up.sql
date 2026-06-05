-- User settings persistence (replaces in-memory settingsStore).
CREATE TABLE IF NOT EXISTS user_settings (
    user_id        VARCHAR(320) PRIMARY KEY REFERENCES "user"(id) ON DELETE CASCADE,
    notifications  JSONB NOT NULL DEFAULT '{"email_enabled": true, "webhook_url": "", "channels": ["email"]}'::jsonb,
    layout         JSONB NOT NULL DEFAULT '{"editor_font_size": 14, "theme": "dark", "default_view_mode": "pro"}'::jsonb,
    language       JSONB NOT NULL DEFAULT '{"locale": "en", "timezone": "UTC"}'::jsonb,
    project        JSONB NOT NULL DEFAULT '{"work_dir": ""}'::jsonb,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
