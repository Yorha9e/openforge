-- 010_gate_stage_width.up.sql
-- The stage value "decompose" is 9 characters; gate tables need room for it.

ALTER TABLE gate_event ALTER COLUMN stage TYPE VARCHAR(10);
ALTER TABLE checkpoint ALTER COLUMN stage TYPE VARCHAR(10);
ALTER TABLE gate_request ALTER COLUMN stage TYPE VARCHAR(10);
