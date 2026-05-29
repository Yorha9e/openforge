-- Remove changed_files column from pipeline table
ALTER TABLE pipeline DROP COLUMN IF EXISTS changed_files;