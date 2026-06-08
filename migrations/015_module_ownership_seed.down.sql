-- Migration 015 down: remove the _default module-ownership seed rows.
-- Per-project overrides (project_id != '_default') are preserved.

DELETE FROM module_ownership WHERE project_id = '_default';
