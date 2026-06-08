-- 017_app_migration_users.down.sql
-- Reverts app/migration user separation.

-- of 恢复 DDL
GRANT CREATE, DROP, ALTER, TRUNCATE ON DATABASE openforge TO of;
GRANT CREATE ON SCHEMA public TO of;
-- 移除 of_migration 权限
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM of_migration;
REVOKE ALL ON ALL SEQUENCES IN SCHEMA public FROM of_migration;
-- 不删除 role（避免级联），但去掉登录
ALTER ROLE of_migration NOLOGIN;
