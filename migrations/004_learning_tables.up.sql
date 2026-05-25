-- 004_learning_tables.up.sql — Phase 7 Learning Engine tables

CREATE TABLE preference (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    key             VARCHAR(128) NOT NULL,
    value           TEXT NOT NULL,
    weight          DECIMAL(5,2) NOT NULL DEFAULT 0,
    source          VARCHAR(32) NOT NULL CHECK (source IN ('code_review','auto_detect','ab_experiment','manual','skill_usage','tool_success')),
    conflict_count  INT NOT NULL DEFAULT 0,
    last_activated  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, key, value)
);
CREATE INDEX idx_preference_project ON preference(project_id);

CREATE TABLE trajectory (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    pipeline_id     TEXT NOT NULL,
    stage_sequence  TEXT[] NOT NULL,
    total_chat_rounds INT NOT NULL,
    total_tokens    BIGINT NOT NULL,
    backtrack_count INT NOT NULL DEFAULT 0,
    rejection_count INT NOT NULL DEFAULT 0,
    failure_codes   TEXT[],
    successful_patterns TEXT[],
    tools_used      TEXT[],
    skills_matched  TEXT[],
    requirement_summary TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_trajectory_project ON trajectory(project_id);

CREATE TABLE knowledge_snapshot (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id      TEXT NOT NULL REFERENCES project(id),
    version         INT NOT NULL CHECK (version > 0),
    snapshot_data   JSONB NOT NULL,
    signature       VARCHAR(128),
    health_baseline BOOLEAN NOT NULL DEFAULT false,
    code_acceptance_rate DECIMAL(5,2) CHECK (code_acceptance_rate >= 0 AND code_acceptance_rate <= 100),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(project_id, version)
);

CREATE TABLE ab_experiment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    knowledge_id    TEXT NOT NULL,
    cohort_a_ratio  DECIMAL(3,2) NOT NULL DEFAULT 0.90
                    CHECK (cohort_a_ratio > 0 AND cohort_a_ratio < 1),
    status          VARCHAR(16) NOT NULL DEFAULT 'running'
                    CHECK (status IN ('running','completed','aborted')),
    verdict         VARCHAR(8) CHECK (verdict IN ('promoted','invalid','harmful')),
    p_value         DECIMAL(6,4) CHECK (p_value >= 0 AND p_value <= 1),
    effect_size     DECIMAL(6,4),
    started_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE TABLE ab_experiment_assignment (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    experiment_id   UUID NOT NULL REFERENCES ab_experiment(id),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    cohort          CHAR(1) NOT NULL CHECK (cohort IN ('A','B')),
    code_acceptance_rate DECIMAL(5,2),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE pipeline_retrospective (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pipeline_id     TEXT NOT NULL REFERENCES pipeline(id),
    project_id      TEXT NOT NULL REFERENCES project(id),
    duration_seconds INT,
    chat_rounds     INT,
    total_tokens    BIGINT,
    rejection_count INT,
    backtrack_count INT,
    lessons_learned TEXT[],
    improvement_actions TEXT[],
    knowledge_updates TEXT[],               -- knowledge IDs updated by this pipeline
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_retrospective_pipeline ON pipeline_retrospective(pipeline_id);
CREATE INDEX idx_retrospective_project ON pipeline_retrospective(project_id);
