-- 001_init.up.sql — Phase 1 tables

-- 1. Project & users
CREATE TABLE IF NOT EXISTS project (
    id          TEXT CONSTRAINT pk_project PRIMARY KEY,
    name        VARCHAR(255) NOT NULL,
    git_url     VARCHAR(512) NOT NULL,
    repo_type   VARCHAR(64) NOT NULL CHECK (repo_type IN ('monorepo-node-react','custom')),
    template    VARCHAR(64) NOT NULL DEFAULT 'custom',
    deleted_at  TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    config      JSONB NOT NULL DEFAULT '{}'
);

CREATE TABLE IF NOT EXISTS"user" (
    id          VARCHAR(320) PRIMARY KEY,
    display_name VARCHAR(128) NOT NULL,
    avatar_url  VARCHAR(512),
    disabled_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login  TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_role (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     VARCHAR(320) NOT NULL REFERENCES "user"(id),
    project_id  TEXT NOT NULL REFERENCES project(id),
    role        VARCHAR(16) NOT NULL CHECK (role IN ('admin','pm','dev_lead','dev','observer')),
    modules     TEXT[] NOT NULL DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, project_id)
);

CREATE TABLE IF NOT EXISTS module_ownership (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id  TEXT NOT NULL REFERENCES project(id),
    module_name VARCHAR(64) NOT NULL,
    paths       TEXT[] NOT NULL,
    team_name   VARCHAR(128) NOT NULL,
    reviewers   TEXT[] NOT NULL,
    fallback_reviewer VARCHAR(320) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, module_name)
);

-- 2. Pipeline core
CREATE TABLE IF NOT EXISTS pipeline (
    id              TEXT PRIMARY KEY,
    project_id      TEXT NOT NULL REFERENCES project(id),
    parent_pipeline_id TEXT REFERENCES pipeline(id),
    title           VARCHAR(512) NOT NULL,
    level           VARCHAR(4) NOT NULL CHECK (level IN ('L1','L2','L3','L4')),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','paused','awaiting_review',
                                      'completed','rejected','token_exceeded','cancelled','dormant')),
    current_stage   VARCHAR(10) CHECK (current_stage IN ('clarify','decompose','impl','test','deploy','verify')),
    created_by      VARCHAR(320) NOT NULL,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ,
    region          VARCHAR(16) NOT NULL DEFAULT 'bj',
    config          JSONB NOT NULL DEFAULT '{}',
    backtrack_count INT NOT NULL DEFAULT 0 CHECK (backtrack_count >= 0 AND backtrack_count <= 3),
    version         INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_pipeline_project_status ON pipeline(project_id, status);
CREATE INDEX IF NOT EXISTS idx_pipeline_parent ON pipeline(parent_pipeline_id);
CREATE INDEX IF NOT EXISTS idx_pipeline_created_by ON pipeline(created_by, created_at);

CREATE TABLE IF NOT EXISTS pipeline_stage (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(10) NOT NULL CHECK (stage IN ('clarify','decompose','impl','test','deploy','verify')),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending'
                    CHECK (status IN ('pending','running','awaiting_gate','passed','failed','skipped')),
    requirement_summary TEXT,
    constraints         TEXT[],
    preference_profile  TEXT,
    module_index_subset TEXT,
    summary         TEXT,
    artifact_ref    TEXT NOT NULL DEFAULT '',
    artifact_hash   VARCHAR(64),
    started_at      TIMESTAMPTZ,
    completed_at    TIMESTAMPTZ,
    schema_version  INT NOT NULL DEFAULT 1,
    version         INT NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS idx_pipeline_stage_pipeline ON pipeline_stage(pipeline_id);

CREATE TABLE IF NOT EXISTS gate_event (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(8) NOT NULL,
    event           VARCHAR(16) NOT NULL
                    CHECK (event IN ('awaiting','approved','rejected','claimed','timeout','auto_bypassed')),
    actor           VARCHAR(320) NOT NULL,
    decision        VARCHAR(8) CHECK (decision IN ('approve','reject','comment')),
    line_comments   JSONB,
    summary_feedback TEXT,
    checklist       JSONB,
    artifact_hash   VARCHAR(64),
    prev_hash       VARCHAR(64) NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_gate_event_pipeline ON gate_event(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_gate_event_actor ON gate_event(actor, event, created_at);

-- 3. Checkpoints
CREATE TABLE IF NOT EXISTS checkpoint (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    stage           VARCHAR(8) NOT NULL,
    seq             INT NOT NULL,
    data            JSONB NOT NULL,
    trigger         VARCHAR(8) NOT NULL CHECK (trigger IN ('auto','manual')),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_checkpoint_pipeline_stage ON checkpoint(pipeline_id, stage DESC);

-- 4. Conversation
CREATE TABLE IF NOT EXISTS conversation_message (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    branch_id       VARCHAR(32) NOT NULL DEFAULT 'main',
    msg_seq         INT NOT NULL,
    role            VARCHAR(8) NOT NULL CHECK (role IN ('user','agent','system')),
    msg_type        VARCHAR(16) NOT NULL DEFAULT 'text'
                    CHECK (msg_type IN ('text','code_card','topo_card','gate_card','error_card')),
    content         TEXT NOT NULL,
    token_count     INT,
    reply_to_seq    INT,
    deleted_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(pipeline_id, branch_id, msg_seq)
);

CREATE TABLE IF NOT EXISTS conversation_branch (
    id              VARCHAR(32) PRIMARY KEY,
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    parent_branch   VARCHAR(32) NOT NULL DEFAULT 'main',
    fork_msg_seq    INT NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','merged','abandoned')),
    created_by      VARCHAR(320) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- 5. File locks
CREATE TABLE IF NOT EXISTS file_lock (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    project_id      TEXT NOT NULL REFERENCES project(id),
    file_path       VARCHAR(512) NOT NULL,
    lock_type       VARCHAR(10) NOT NULL CHECK (lock_type IN ('write','read_only')),
    estimated_duration INT NOT NULL,
    locked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    UNIQUE(project_id, file_path)
);

CREATE INDEX IF NOT EXISTS idx_file_lock_project ON file_lock(project_id);

-- 6. Token usage (partitioned)
CREATE TABLE IF NOT EXISTS token_usage (
    id              BIGSERIAL NOT NULL,
    pipeline_id     TEXT NOT NULL,
    project_id      TEXT NOT NULL REFERENCES project(id),
    provider        VARCHAR(32) NOT NULL,
    model           VARCHAR(64) NOT NULL,
    prompt_tokens     BIGINT NOT NULL CHECK (prompt_tokens >= 0),
    completion_tokens BIGINT NOT NULL CHECK (completion_tokens >= 0),
    estimated_cost    DECIMAL(10,4) NOT NULL CHECK (estimated_cost >= 0),
    batch_id        UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS token_usage_2026_05 PARTITION OF token_usage
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS token_usage_2026_06 PARTITION OF token_usage
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_token_usage_pipeline ON token_usage(pipeline_id);
CREATE INDEX IF NOT EXISTS idx_token_usage_project ON token_usage(project_id, created_at);

CREATE TABLE IF NOT EXISTS cost_quota (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    month           VARCHAR(7) NOT NULL CHECK (month ~ '^\d{4}-\d{2}$'),
    token_limit     BIGINT NOT NULL CHECK (token_limit > 0),
    token_used      BIGINT NOT NULL DEFAULT 0,
    status          VARCHAR(16) NOT NULL DEFAULT 'active'
                    CHECK (status IN ('active','exceeded','special_approved')),
    UNIQUE(project_id, month)
);

-- 7. Audit log (WORM, partitioned)
CREATE TABLE IF NOT EXISTS audit_log (
    id              UUID DEFAULT gen_random_uuid() NOT NULL,
    event           VARCHAR(64) NOT NULL,
    actor           VARCHAR(320) NOT NULL,
    action          VARCHAR(128) NOT NULL,
    resource        VARCHAR(256) NOT NULL,
    result          VARCHAR(16) NOT NULL DEFAULT 'success'
                    CHECK (result IN ('success','failure')),
    error_code      VARCHAR(32),
    source_ip       INET,
    user_agent      VARCHAR(512),
    project_id      TEXT,
    region          VARCHAR(16) NOT NULL DEFAULT 'bj',
    artifact_hash   VARCHAR(64),
    prev_hash       VARCHAR(64) NOT NULL,
    content_hash    VARCHAR(64) NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

CREATE TABLE IF NOT EXISTS audit_log_2026_05 PARTITION OF audit_log
    FOR VALUES FROM ('2026-05-01') TO ('2026-06-01');
CREATE TABLE IF NOT EXISTS audit_log_2026_06 PARTITION OF audit_log
    FOR VALUES FROM ('2026-06-01') TO ('2026-07-01');

CREATE INDEX IF NOT EXISTS idx_audit_log_created ON audit_log(created_at);
CREATE INDEX IF NOT EXISTS idx_audit_log_actor ON audit_log(actor, created_at);

-- 8. Feature flags
CREATE TABLE IF NOT EXISTS feature_flag (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            VARCHAR(64) NOT NULL UNIQUE,
    owner           VARCHAR(128) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'experimental'
                    CHECK (status IN ('experimental','beta','stable','deprecated')),
    rollout_percent INT NOT NULL DEFAULT 0 CHECK (rollout_percent >= 0 AND rollout_percent <= 100),
    description     VARCHAR(512),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ,
    deprecated_at   TIMESTAMPTZ
);

-- 9. Task queue
CREATE TABLE IF NOT EXISTS task_queue (
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

CREATE INDEX IF NOT EXISTS idx_task_queue_dequeue ON task_queue(status, priority DESC, created_at DESC)
    WHERE status = 'pending';
