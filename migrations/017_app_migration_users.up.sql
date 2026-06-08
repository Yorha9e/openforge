-- 017_app_migration_users.up.sql
-- Separate application user (of) from migration user (of_migration).
-- The application user keeps DML rights; the migration user owns DDL rights.

DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'of_migration') THEN
        CREATE ROLE of_migration LOGIN PASSWORD 'of_migration_dev';
    END IF;
END$$;

-- of_migration 拥有 DDL 权限
GRANT CREATE, DROP, ALTER, TRUNCATE ON DATABASE openforge TO of_migration;
GRANT ALL ON ALL TABLES IN SCHEMA public TO of_migration;
GRANT ALL ON ALL SEQUENCES IN SCHEMA public TO of_migration;

-- 应用用户 of 失去 DDL 权限（保留 DML）
REVOKE CREATE, DROP, ALTER, TRUNCATE ON DATABASE openforge FROM of;
REVOKE CREATE ON SCHEMA public FROM of;
