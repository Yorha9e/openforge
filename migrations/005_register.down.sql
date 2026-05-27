-- 005_register.down.sql
ALTER TABLE "user" DROP COLUMN IF EXISTS password_hash;
