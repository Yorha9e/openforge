-- 010_gate_stage_width.down.sql

ALTER TABLE gate_request ALTER COLUMN stage TYPE VARCHAR(8);
ALTER TABLE checkpoint ALTER COLUMN stage TYPE VARCHAR(8);
ALTER TABLE gate_event ALTER COLUMN stage TYPE VARCHAR(8);
