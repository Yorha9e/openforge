-- 012_audit_log_revoke.up.sql
-- DB-level anti-tamper for audit_log: revoke DELETE/UPDATE/TRUNCATE from the
-- application user; route INSERTs through a dedicated low-privilege role.
-- See DESIGN.md §6.6 审计防篡改.
--
-- The of_audit_writer role MUST be created out-of-band (e.g. via the
-- docker-entrypoint-initdb.d/ script in deployments/, or by the operator) before
-- this migration runs. CREATE ROLE cannot be executed inside a transaction
-- block, and the in-process migration runner wraps every .up.sql in a tx.
-- This file only contains idempotent GRANT/REVOKE statements that are safe to
-- re-run.

-- 1. App user (openforge) keeps SELECT, loses everything else.
GRANT SELECT ON audit_log TO openforge;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON audit_log FROM openforge;
REVOKE INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON audit_log FROM PUBLIC;

-- 2. Dedicated writer role: INSERT only.
GRANT INSERT ON audit_log TO of_audit_writer;
REVOKE UPDATE, DELETE, TRUNCATE ON audit_log FROM of_audit_writer;
