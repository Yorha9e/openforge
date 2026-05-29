-- Add changed_files column to pipeline table for Pro Mode
ALTER TABLE pipeline ADD COLUMN IF NOT EXISTS changed_files JSONB DEFAULT '[]';