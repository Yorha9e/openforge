-- migrations/020_encrypted_backup.up.sql
-- 仅标记：备份加密由 DR adapter (T6 Step 2) 实现
COMMENT ON EXTENSION pgcrypto IS 'Used for encrypted backups (DESIGN §19.4 #21)';
