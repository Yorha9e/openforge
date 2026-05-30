-- 009_user_role_and_invitation.up.sql
-- Add global role column to user table and create invitation table

-- Add role column for global user role (used during registration)
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS role VARCHAR(16) DEFAULT 'dev';

-- Create invitation table
CREATE TABLE IF NOT EXISTS invitation (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    token       VARCHAR(64) NOT NULL UNIQUE,
    role        VARCHAR(16) NOT NULL CHECK (role IN ('admin','pm','dev_lead','dev','observer')),
    project_id  TEXT REFERENCES project(id),
    created_by  VARCHAR(320) NOT NULL REFERENCES "user"(id),
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    used_by     VARCHAR(320) REFERENCES "user"(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_invitation_token ON invitation(token);
CREATE INDEX IF NOT EXISTS idx_invitation_created_by ON invitation(created_by);
