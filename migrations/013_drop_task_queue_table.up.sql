-- 013_drop_task_queue_table.up.sql
-- 目的: task_queue 表在 Phase 5 后由 Redis Streams 替代 (pg-skip-locked 已被新方案取代)。
-- 应用代码已不再使用此表 (newTaskQueue 对非 redis-streams 配置返回 noop)。
-- T8 已确认 0 rows, 安全 drop。
-- CASCADE 用于同时移除 idx_task_queue_dequeue 部分索引。

DROP TABLE IF EXISTS task_queue CASCADE;
