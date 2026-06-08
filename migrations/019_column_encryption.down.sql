-- migrations/019_column_encryption.down.sql
DROP TRIGGER IF EXISTS user_email_encrypt ON "user";
DROP FUNCTION IF EXISTS encrypt_user_email();
DROP INDEX IF EXISTS user_email_unique;
ALTER TABLE "user" DROP COLUMN IF EXISTS email_encrypted;
ALTER TABLE "user" DROP COLUMN IF EXISTS email;
