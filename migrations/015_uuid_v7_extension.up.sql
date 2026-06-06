-- 015_uuid_v7_extension.up.sql
-- Item #15: UUID v7 全线统一
-- 启用 pgcrypto (gen_random_uuid) + uuid-ossp，并安装自定义 uuid_generate_v7()
-- uuid-ossp 默认不含 v7；用 plpgsql 实现与 RFC 9562 一致的 v7 时间戳+随机数

-- 1. 启用 pgcrypto (gen_random_uuid)
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- 2. 启用 uuid-ossp (uuid_generate_v5 等；为未来扩展预留)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- 3. 自定义 uuid_generate_v7() — 时间戳前 48 bit + 随机后 80 bit (含 RFC 9562 版本与变体)
CREATE OR REPLACE FUNCTION uuid_generate_v7() RETURNS uuid AS $$
DECLARE
    ts_ms bigint := (extract(epoch from clock_timestamp()) * 1000)::bigint - 12219292800000;
    ts_hex varchar := lpad(to_hex(ts_ms), 12, '0');
    rand_hex varchar := encode(gen_random_bytes(10), 'hex');
    uuid_v7 text := substr(ts_hex, 1, 8) || '-' ||
                   substr(ts_hex, 9, 4) || '-7' ||
                   substr(rand_hex, 1, 3) || '-' ||
                   substr('89ab', 1 + (random() * 3)::int, 1) ||
                   substr(rand_hex, 4, 3) || '-' ||
                   substr(rand_hex, 7, 12);
    RETURN uuid_v7::uuid;
END $$ LANGUAGE plpgsql VOLATILE;
