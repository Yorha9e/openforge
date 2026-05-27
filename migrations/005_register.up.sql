-- 005_register.up.sql — Add password_hash column for user registration
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
