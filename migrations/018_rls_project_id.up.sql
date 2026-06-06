-- migrations/018_rls_project_id.up.sql
-- X3 T4 #18: Row-Level Security — isolate every project_id-scoped table.
-- Policies read the connection-local GUC `app.current_project_id` (set by the
-- Go-side RLSConn wrapper before each query). The `true` second arg of
-- current_setting makes a missing setting return NULL instead of erroring,
-- which we treat as "no project filter applied" (rows hidden unless
-- project_id is also NULL — pipelines, file_locks, etc. have NOT NULL).
ALTER TABLE pipeline ENABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_message ENABLE ROW LEVEL SECURITY;
ALTER TABLE file_lock ENABLE ROW LEVEL SECURITY;
ALTER TABLE gate_event ENABLE ROW LEVEL SECURITY;

CREATE POLICY pipeline_project_isolation ON pipeline
    USING (project_id = current_setting('app.current_project_id', true)::uuid);

CREATE POLICY conversation_message_project_isolation ON conversation_message
    USING (pipeline_id IN (SELECT id FROM pipeline WHERE project_id = current_setting('app.current_project_id', true)::uuid));

CREATE POLICY file_lock_project_isolation ON file_lock
    USING (project_id = current_setting('app.current_project_id', true)::uuid);

CREATE POLICY gate_event_project_isolation ON gate_event
    USING (pipeline_id IN (SELECT id FROM pipeline WHERE project_id = current_setting('app.current_project_id', true)::uuid));
