-- migrations/018_rls_project_id.down.sql
DROP POLICY IF EXISTS gate_event_project_isolation ON gate_event;
DROP POLICY IF EXISTS file_lock_project_isolation ON file_lock;
DROP POLICY IF EXISTS conversation_message_project_isolation ON conversation_message;
DROP POLICY IF EXISTS pipeline_project_isolation ON pipeline;
ALTER TABLE gate_event DISABLE ROW LEVEL SECURITY;
ALTER TABLE file_lock DISABLE ROW LEVEL SECURITY;
ALTER TABLE conversation_message DISABLE ROW LEVEL SECURITY;
ALTER TABLE pipeline DISABLE ROW LEVEL SECURITY;
