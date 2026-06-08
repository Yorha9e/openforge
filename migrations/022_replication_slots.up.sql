-- migrations/022_replication_slots.up.sql
-- Item #23: logical replication slot for change data capture (CDC).
-- Publication exposes pipeline / conversation_message / file_lock / gate_event deltas.

-- pg_create_logical_replication_slot is NOT idempotent.
-- Wrap in DO $$ ... EXCEPTION WHEN duplicate_object THEN NULL $$ to make re-runs safe.
DO $$
BEGIN
    PERFORM pg_create_logical_replication_slot('of_audit_cdc', 'pgoutput');
EXCEPTION WHEN duplicate_object THEN
    -- Slot already exists; nothing to do.
    NULL;
END$$;

CREATE PUBLICATION of_core_publication FOR TABLE
    pipeline, conversation_message, file_lock, gate_event;
