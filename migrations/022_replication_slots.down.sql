-- migrations/022_replication_slots.down.sql
-- Note: pg_drop_replication_slot will fail while the slot is still being consumed.
-- We swallow the error so the migration is reversible when no consumer is attached.
DO $$
BEGIN
    PERFORM pg_drop_replication_slot('of_audit_cdc');
EXCEPTION WHEN OTHERS THEN
    RAISE NOTICE 'slot of_audit_cdc not dropped: %', SQLERRM;
END$$;

DROP PUBLICATION IF EXISTS of_core_publication;
