-- migrations/021_audit_noncore_ddl.up.sql
-- Item #22: schema_change_log 触发器
-- 触发器：捕获对非核心表 (除 pipeline / project / user / audit_log / feature_flags 外) 的 DDL
CREATE TABLE IF NOT EXISTS schema_change_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    table_name TEXT NOT NULL,
    operation TEXT NOT NULL,
    actor TEXT,
    sql_text TEXT,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE FUNCTION audit_ddl_event() RETURNS event_trigger AS $$
DECLARE
    r record;
BEGIN
    FOR r IN SELECT * FROM pg_event_trigger_ddl_commands() LOOP
        INSERT INTO schema_change_log (table_name, operation, sql_text)
        VALUES (r.object_identity, r.command_tag, current_query());
    END LOOP;
END $$ LANGUAGE plpgsql;

CREATE EVENT TRIGGER audit_ddl ON ddl_command_end WHEN TAG IN (
    'CREATE TABLE', 'ALTER TABLE', 'DROP TABLE', 'CREATE INDEX', 'DROP INDEX'
) EXECUTE FUNCTION audit_ddl_event();
