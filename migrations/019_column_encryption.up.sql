-- migrations/019_column_encryption.up.sql
-- pgcrypto column-level encryption for user PII (item #20)
--
-- Strategy:
--   * Add email column (nullable) to "user" for OIDC-derived addresses.
--   * Mirror plaintext in email_encrypted BYTEA via BEFORE INSERT/UPDATE trigger
--     so reads can decrypt via go-side helpers when needed, while the indexable
--     plaintext remains available for login / lookup.
--   * The encryption key is sourced from the per-session GUC `app.encryption_key`.
--     Production deployments must SET this on the connection (e.g. from Vault)
--     before issuing any write that touches the user table.
--
-- Requires: pgcrypto extension (loaded by 015_uuid_v7_extension).

CREATE EXTENSION IF NOT EXISTS pgcrypto;

ALTER TABLE "user" ADD COLUMN IF NOT EXISTS email           VARCHAR(320);
ALTER TABLE "user" ADD COLUMN IF NOT EXISTS email_encrypted BYTEA;

-- Unique lookup index on the plaintext email (login / OIDC subject resolution).
CREATE UNIQUE INDEX IF NOT EXISTS user_email_unique
    ON "user"(email)
    WHERE email IS NOT NULL;

CREATE OR REPLACE FUNCTION encrypt_user_email() RETURNS trigger AS $$
BEGIN
    -- Only (re)encrypt when the plaintext is set and the ciphertext is stale.
    IF NEW.email IS NOT NULL THEN
        NEW.email_encrypted := pgp_sym_encrypt(
            NEW.email,
            current_setting('app.encryption_key', true)
        );
    ELSIF NEW.email IS NULL AND OLD.email_encrypted IS NOT NULL THEN
        -- Clear ciphertext if plaintext was wiped.
        NEW.email_encrypted := NULL;
    END IF;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS user_email_encrypt ON "user";
CREATE TRIGGER user_email_encrypt
    BEFORE INSERT OR UPDATE OF email ON "user"
    FOR EACH ROW EXECUTE FUNCTION encrypt_user_email();
