-- 012_audit_log_revoke.down.sql
-- Restore openforge to a normal application role for audit_log and remove the
-- writer's INSERT grant. Dropping the role itself is left to the operator
-- (DROP ROLE of_audit_writer;) because doing it here would also fail inside
-- a transaction and may strand a live connection.

GRANT INSERT, SELECT ON audit_log TO openforge;
REVOKE INSERT ON audit_log FROM of_audit_writer;
