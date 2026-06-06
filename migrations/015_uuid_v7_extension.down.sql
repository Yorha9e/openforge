-- 015_uuid_v7_extension.down.sql
DROP FUNCTION IF EXISTS uuid_generate_v7();
DROP EXTENSION IF EXISTS "uuid-ossp";
DROP EXTENSION IF EXISTS pgcrypto;
