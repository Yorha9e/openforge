-- Migration 015: module-ownership seed data
--
-- Seeds the module_ownership table with a `_default` project containing the
-- minimal-profile reviewer routing rules. The canonical column set
-- (project_id, module_name, paths, team_name, reviewers, fallback_reviewer)
-- was created in 001_init.up.sql. This migration only inserts the
-- dev-team default mappings; production teams override these rows at
-- runtime via PGOwnershipRepository.Upsert (or by editing this file).
--
-- The `_default` project_id is a sentinel used when no per-project rules
-- apply; OwnershipService resolves it as a fallback after per-project
-- ownership lookups miss.

INSERT INTO module_ownership
    (project_id, module_name, paths, team_name, reviewers, fallback_reviewer)
VALUES
    ('_default', 'src/features/chat',
     ARRAY['src/features/chat/']::TEXT[],
     'team-chat',
     ARRAY['dev_lead_a']::TEXT[],
     'dev_lead_b'),
    ('_default', 'src/features/code-review',
     ARRAY['src/features/code-review/']::TEXT[],
     'team-review',
     ARRAY['dev_lead_b']::TEXT[],
     'dev_lead_a'),
    ('_default', 'internal/agent',
     ARRAY['internal/agent/']::TEXT[],
     'platform',
     ARRAY['dev_lead_a', 'dev_lead_b']::TEXT[],
     'pm')
ON CONFLICT (project_id, module_name) DO NOTHING;
