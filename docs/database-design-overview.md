i# 数据库设计总览

> 基于 `migrations/` 目录下所有迁移文件整理

---

## 迁移文件列表

| 文件 | 表数量 | 说明 |
|------|--------|------|
| `migrations/001_init.up.sql` | 15 | Phase 1 — 核心表 (project / pipeline / gate / checkpoint / conversation / file_lock / token / audit / feature_flag / task_queue) |
| `migrations/003_gate_request.up.sql` | 1 | Phase 3 — 审批挂起门禁表 |
| `migrations/004_learning_tables.up.sql` | 6 | Phase 7 — 学习引擎表 (preference / trajectory / knowledge_snapshot / ab_experiment / retrospective) |

---

## 001_init.up.sql — Phase 1 核心表

### 1.1 项目 & 用户

#### `project`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | |
| name | VARCHAR(255) | 项目名称 |
| git_url | VARCHAR(512) | Git 仓库地址 |
| repo_type | VARCHAR(64) | `monorepo-node-react` / `custom` |
| template | VARCHAR(64) | 模板 |
| deleted_at | TIMESTAMPTZ | 软删除 |
| config | JSONB | 配置 |

- **实现状态**: ❌ 未实现 — 表已创建，无 Go 代码 CRUD
- **行号**: 001_init.up.sql 第 3-14 行

#### `"user"`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(320) PK | 邮箱/用户 ID |
| display_name | VARCHAR(128) | 显示名称 |
| avatar_url | VARCHAR(512) | 头像 URL |
| disabled_at | TIMESTAMPTZ | 禁用时间 |
| last_login | TIMESTAMPTZ | 最后登录 |

- **实现状态**: ❌ 未实现
- **行号**: 001_init.up.sql 第 16-23 行

#### `user_role`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | |
| user_id | VARCHAR(320) → user(id) | |
| project_id | TEXT → project(id) | |
| role | VARCHAR(16) | `admin` / `pm` / `dev_lead` / `dev` / `observer` |
| modules | TEXT[] | 负责模块列表 |

- **实现状态**: ❌ 未实现
- **行号**: 001_init.up.sql 第 25-33 行

#### `module_ownership`
| 字段 | 类型 | 说明 |
|------|------|------|
| module_name | VARCHAR(64) | 模块名 |
| paths | TEXT[] | 文件路径列表 |
| team_name | VARCHAR(128) | 团队名 |
| reviewers | TEXT[] | 审查人列表 |
| fallback_reviewer | VARCHAR(320) | 兜底审查人 |

- **实现状态**: ❌ 未实现（仅「通」字段引用）
- **行号**: 001_init.up.sql 第 35-45 行

### 1.2 Pipeline 核心

#### `pipeline`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | TEXT PK | |
| project_id | TEXT → project(id) | |
| title | VARCHAR(512) | |
| level | VARCHAR(4) | `L1` / `L2` / `L3` / `L4` |
| status | VARCHAR(16) | `pending` / `running` / `paused` / `awaiting_review` / `completed` / `rejected` / `token_exceeded` / `cancelled` / `dormant` |
| current_stage | VARCHAR(10) | `clarify` / `decompose` / `impl` / `test` / `deploy` / `verify` |
| created_by | VARCHAR(320) | |
| region | VARCHAR(16) | 默认 `bj` |
| config | JSONB | |
| backtrack_count | INT 0-3 | 回溯计数 |
| version | INT | 乐观锁版本号 |

- **实现状态**: ✅ 完全实现
- **Go 代码**: `internal/pipeline/adapter/pg_repository.go` 第 29-103 行
  - `Create()`: INSERT pipeline (行 29-36)
  - `GetByID()`: SELECT by id (行 38-56)
  - `ListByProject()`: SELECT by project (行 58-80)
  - `UpdateStatus()`: 乐观锁更新 status + version (行 82-95)
  - `IncrementBacktrack()`: 递增 backtrack_count (行 97-103)
- **行号**: 001_init.up.sql 第 48-67 行
- **索引**: project+status / parent_pipeline_id / created_by+created_at

#### `pipeline_stage`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| stage | VARCHAR(10) | 同上 6 个阶段 |
| status | VARCHAR(16) | `pending` / `running` / `awaiting_gate` / `passed` / `failed` / `skipped` |
| requirement_summary | TEXT | |
| constraints | TEXT[] | |
| summary | TEXT | |
| artifact_ref | TEXT | |
| artifact_hash | VARCHAR(64) | |
| schema_version | INT | |

- **实现状态**: ❌ 未实现
- **行号**: 001_init.up.sql 第 73-90 行

#### `gate_event`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| stage | VARCHAR(8) | |
| event | VARCHAR(16) | `awaiting` / `approved` / `rejected` / `claimed` / `timeout` / `auto_bypassed` |
| actor | VARCHAR(320) | |
| decision | VARCHAR(8) | `approve` / `reject` / `comment` |
| line_comments | JSONB | 行级评论 |
| summary_feedback | TEXT | |
| checklist | JSONB | 审批清单 |
| prev_hash | VARCHAR(64) | 哈希链前驱 |
| content_hash | VARCHAR(64) | 哈希链内容 |

- **实现状态**: ✅ 完全实现
- **Go 代码**: `internal/pipeline/adapter/pg_repository.go` 第 107-202 行
  - `CreateEvent()`: INSERT gate_event (行 120-131)
  - `GetLatestHash()`: 查询最新 content_hash (行 107-118)
  - `ListByPipeline()`: 按 pipeline 查询 (行 133-158)
  - `ListPending()`: 查询所有 `awaiting` 事件 (行 160-186)
  - `Claim()`: 认领审批 (行 188-194)
  - `ReleaseClaim()`: 释放认领 (行 196-202)
- **行号**: 001_init.up.sql 第 94-109 行

#### `checkpoint`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| stage | VARCHAR(8) | |
| seq | INT | 序列号 |
| data | JSONB | 序列化消息数据 |
| trigger | VARCHAR(8) | `auto` / `manual` |

- **实现状态**: ⚠️ 接口已定义，触发逻辑存在，**但无 PG 适配器**
- **Go 接口**: `internal/agent/domain/query_engine.go` 第 45-46 行
- **调用代码**: `query_engine.go` `saveCheckpoint()` (第 586-607 行)
  - 仅在**上下文溢出**或**达到 50 轮限制**时触发
  - 异步保存 `go func() { _ = qe.checkpointRepo.Save(...) }()`
  - **错误被静默丢弃**
- **行号**: 001_init.up.sql 第 115-123 行

### 1.3 对话（Conversation）

#### `conversation_message` ⚠️
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| branch_id | VARCHAR(32) | 分支 ID，默认 `main` |
| msg_seq | INT | 消息序号 |
| role | VARCHAR(8) | `user` / `agent` / `system` |
| msg_type | VARCHAR(16) | `text` / `code_card` / `topo_card` / `gate_card` / `error_card` |
| content | TEXT | |
| token_count | INT | |
| reply_to_seq | INT | 回复的目标消息序号 |
| deleted_at | TIMESTAMPTZ | |
| UNIQUE(pipeline_id, branch_id, msg_seq) | |

- **实现状态**: ❌ **完全未实现** ⚠️⚠️⚠️
- **Go 代码**: 无写入逻辑，仅有 `routes.go` 第 583-589 行硬编码返回 `[]`
  ```go
  // Phase 7: read from conversation_message table
  // Phase 6.5: return empty — messages are ephemeral (WS only)
  writeJSON(w, 200, []any{})
  ```
- **行号**: 001_init.up.sql 第 128-142 行

#### `conversation_branch`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | VARCHAR(32) PK | 分支 ID |
| pipeline_id | TEXT → pipeline(id) | |
| parent_branch | VARCHAR(32) | 父分支 ID |
| fork_msg_seq | INT | 分叉点消息序号 |
| status | VARCHAR(16) | `active` / `merged` / `abandoned` |
| created_by | VARCHAR(320) | |

- **实现状态**: ❌ **完全未实现**
- **行号**: 001_init.up.sql 第 144-153 行

### 1.4 文件锁

#### `file_lock`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| project_id | TEXT → project(id) | |
| file_path | VARCHAR(512) | 文件路径 |
| lock_type | VARCHAR(10) | `write` / `read_only` |
| estimated_duration | INT | 预估秒数 |
| expires_at | TIMESTAMPTZ | 过期时间 |
| UNIQUE(project_id, file_path) | |

- **实现状态**: ✅ 完全实现
- **Go 代码**: `internal/pipeline/adapter/pg_file_lock.go` 第 23-115 行
  - `Acquire()`: INSERT ON CONFLICT DO NOTHING (行 23-41)
  - `Release()`: DELETE by project+file (行 44-50)
  - `ListByProject()`: 查询项目所有锁 (行 53-77)
  - `DetectDeadlock()`: DFS 死锁检测 (行 81-115)
- **行号**: 001_init.up.sql 第 156-166 行

### 1.5 Token 用量 & 预算

#### `token_usage`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | BIGSERIAL | |
| pipeline_id | TEXT | |
| project_id | TEXT → project(id) | |
| provider | VARCHAR(32) | |
| model | VARCHAR(64) | |
| prompt_tokens | BIGINT | |
| completion_tokens | BIGINT | |
| estimated_cost | DECIMAL(10,4) | |
| created_at | TIMESTAMPTZ | 月分区键 |
| PRIMARY KEY (id, created_at) | | RANGE 分区 |

- **分区**: `token_usage_2026_05` / `token_usage_2026_06`
- **实现状态**: ⚠️ 仅实现只读聚合查询
- **Go 代码**: `internal/pipeline/adapter/pg_repository.go` 第 206-275 行
  - `AggregateByDay()`: 按天聚合 (行 206-224)
  - `AggregateByModel()`: 按模型聚合 (行 226-244)
  - `GetCurrentMonthUsage()`: 当月用量统计 (行 265-275)
- **行号**: 001_init.up.sql 第 171-183 行

#### `cost_quota`
| 字段 | 类型 | 说明 |
|------|------|------|
| project_id | TEXT → project(id) | |
| month | VARCHAR(7) | `YYYY-MM` |
| token_limit | BIGINT | 月度上限 |
| token_used | BIGINT | 已用 |
| status | VARCHAR(16) | `active` / `exceeded` / `special_approved` |
| UNIQUE(project_id, month) | |

- **实现状态**: ⚠️ 仅实现只读查询
- **Go 代码**: `pg_repository.go` 第 246-263 行 `GetProjectBudget()`（无记录时返回默认值）
- **行号**: 001_init.up.sql 第 193-202 行

### 1.6 审计日志

#### `audit_log`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID | |
| event | VARCHAR(64) | |
| actor | VARCHAR(320) | |
| action | VARCHAR(128) | |
| resource | VARCHAR(256) | |
| result | VARCHAR(16) | `success` / `failure` |
| source_ip | INET | |
| project_id | TEXT | |
| prev_hash | VARCHAR(64) | 哈希链前驱 |
| content_hash | VARCHAR(64) | 哈希链内容 |
| PRIMARY KEY (id, created_at) | | RANGE 分区 |

- **分区**: `audit_log_2026_05` / `audit_log_2026_06`
- **实现状态**: ✅ 完全实现（WORM 审计链）
- **Go 代码**: `internal/policy/adapter/worm_audit_log.go` 行 32-54
  - `Log()`: INSERT audit_log + SHA256 哈希链
- **行号**: 001_init.up.sql 第 205-223 行

### 1.7 Feature Flag

#### `feature_flag`
| 字段 | 类型 | 说明 |
|------|------|------|
| name | VARCHAR(64) UNIQUE | 特性名称 |
| owner | VARCHAR(128) | |
| status | VARCHAR(16) | `experimental` / `beta` / `stable` / `deprecated` |
| rollout_percent | INT 0-100 | 灰度百分比 |
| expires_at | TIMESTAMPTZ | |

- **实现状态**: ❌ 未实现 — 仅有 `llm/registry.go` 中字段名引用
- **行号**: 001_init.up.sql 第 234-245 行

### 1.8 任务队列

#### `task_queue`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT | |
| project_id | TEXT | |
| task_type | VARCHAR(32) | `llm_request` / `sandbox_run` / `notification` |
| priority | INT 0-3 | |
| payload | JSONB | |
| status | VARCHAR(16) | `pending` / `claimed` / `running` / `completed` / `failed` |
| retry_count | INT | |
| max_retries | INT | 默认 3 |
| 条件索引 | | `WHERE status = 'pending'` |

- **实现状态**: ❌ **未使用** — Go 代码使用 Redis 实现 (`redis_task_queue.go`), 此 PG 表空置
- **行号**: 001_init.up.sql 第 248-268 行

---

## 003_gate_request.up.sql — Phase 3 审批挂起

#### `gate_request`
| 字段 | 类型 | 说明 |
|------|------|------|
| id | UUID PK | |
| pipeline_id | TEXT → pipeline(id) | |
| stage | VARCHAR(8) | |
| status | VARCHAR(16) | `pending` / `approved` / `rejected` / `timeout` / `cancelled` |
| requested_by | TEXT | |
| approved_by | TEXT | |
| approved_at | TIMESTAMPTZ | |
| result | JSONB | |
| timeout_at | TIMESTAMPTZ | 超时时间（5 分钟） |

- **实现状态**: ❌ 未实现（有详细 spec 文档但无 Go 代码）
- **Spec 文档**: `docs/superpowers/specs/2026-05-23-qe-03-gaterepo.md`
- **计划在**: Phase 3 / Phase 9-10 实现
- **行号**: 003_gate_request.up.sql 第 3-16 行

---

## 004_learning_tables.up.sql — Phase 7 学习引擎

#### `preference`
| 字段 | 类型 | 说明 |
|------|------|------|
| project_id | TEXT → project(id) | |
| key | VARCHAR(128) | 偏好键名 |
| value | TEXT | 偏好值 |
| weight | DECIMAL(5,2) | 权重 |
| source | VARCHAR(32) | `code_review` / `auto_detect` / `ab_experiment` / `manual` / `skill_usage` / `tool_success` |
| conflict_count | INT | 冲突计数 |
| last_activated | TIMESTAMPTZ | |
| UNIQUE(project_id, key, value) | |

- **实现状态**: ✅ 完全实现
- **Go 代码**: `internal/agent/adapter/pg_preference_store.go` 行 18-78
  - `Upsert()`: INSERT ON CONFLICT DO UPDATE (行 18-28)
  - `ListByProject()`: 按项目查询 (行 30-49)
  - `Get()`: 按项目+键查询 (行 51-70)
  - `ResolveConflict()`: 冲突解决 (行 72-78)
- **行号**: 004_learning_tables.up.sql 第 3-14 行

#### `trajectory`
| 字段 | 类型 | 说明 |
|------|------|------|
| project_id | TEXT → project(id) | |
| pipeline_id | TEXT | |
| stage_sequence | TEXT[] | 阶段序列 |
| total_chat_rounds | INT | 总对话轮次 |
| total_tokens | BIGINT | |
| backtrack_count | INT | |
| rejection_count | INT | |
| failure_codes | TEXT[] | 失败码 |
| successful_patterns | TEXT[] | 成功模式 |
| tools_used | TEXT[] | 使用的工具 |
| skills_matched | TEXT[] | 匹配的技能 |

- **实现状态**: ✅ 完全实现
- **Go 代码**: `internal/agent/adapter/pg_trajectory_store.go` 行 20-137
  - `Record()`: INSERT trajectory (行 20-31)
  - `ListByProject()`: 按项目查询 (行 33-45)
  - `GetByPipeline()`: 按 pipeline 查询 (行 47-63)
  - `SimilarPatterns()`: 相似失败模式搜索 (行 65-78)
  - `SuccessfulTools()`: 成功工具统计 (行 80-99)
  - `MatchedSkills()`: 匹配技能统计 (行 101-120)
- **行号**: 004_learning_tables.up.sql 第 17-32 行

#### `knowledge_snapshot`
| 字段 | 类型 | 说明 |
|------|------|------|
| project_id | TEXT → project(id) | |
| version | INT | 版本号 |
| snapshot_data | JSONB | 快照数据 |
| signature | VARCHAR(128) | 签名 |
| health_baseline | BOOLEAN | 健康基线 |
| code_acceptance_rate | DECIMAL(5,2) | 代码接受率 |
| UNIQUE(project_id, version) | |

- **实现状态**: ❌ 未实现（仅有领域接口定义）
- **Go 接口**: `internal/agent/domain/knowledge_snapshot.go` 第 28 行 `KnowledgeSnapshotStore`
- **行号**: 004_learning_tables.up.sql 第 35-45 行

#### `ab_experiment`
| 字段 | 类型 | 说明 |
|------|------|------|
| knowledge_id | TEXT | |
| cohort_a_ratio | DECIMAL(3,2) | 分组比例 |
| status | VARCHAR(16) | `running` / `completed` / `aborted` |
| verdict | VARCHAR(8) | `promoted` / `invalid` / `harmful` |
| p_value | DECIMAL(6,4) | 统计显著性 |
| effect_size | DECIMAL(6,4) | 效应量 |

- **实现状态**: ❌ 未实现（仅有领域接口定义）
- **Go 接口**: `internal/agent/domain/ab_experiment.go` 第 32 行 `ExperimentStore`
- **行号**: 004_learning_tables.up.sql 第 47-59 行

#### `ab_experiment_assignment`
- **实现状态**: ❌ 未实现
- **行号**: 004_learning_tables.up.sql 第 61-68 行

#### `pipeline_retrospective`
| 字段 | 类型 | 说明 |
|------|------|------|
| pipeline_id | TEXT → pipeline(id) | |
| project_id | TEXT → project(id) | |
| duration_seconds | INT | |
| chat_rounds | INT | |
| total_tokens | BIGINT | |
| rejection_count | INT | |
| backtrack_count | INT | |
| lessons_learned | TEXT[] | 经验教训 |
| improvement_actions | TEXT[] | 改进行动 |
| knowledge_updates | TEXT[] | 更新的知识 ID |

- **实现状态**: ❌ 未实现（仅有领域接口定义）
- **Go 接口**: `internal/agent/domain/retrospective.go` 第 28 行 `RetrospectiveStore`
- **行号**: 004_learning_tables.up.sql 第 70-84 行

---

## 汇总

### 按实现状态

| 状态 | 数量 | 表名 |
|------|------|------|
| ✅ 完全实现 | 6 | pipeline, gate_event, file_lock, audit_log, preference, trajectory |
| ⚠️ 部分实现 | 4 | checkpoint(仅有接口), token_usage(只读聚合), cost_quota(只读), task_queue(改用Redis) |
| ❌ 未实现 | 12 | project, `"user"`, user_role, module_ownership, pipeline_stage, **conversation_message**, **conversation_branch**, gate_request, knowledge_snapshot, ab_experiment, ab_experiment_assignment, pipeline_retrospective |

### 按功能域

| 功能域 | 表 | 实现状态 |
|--------|-----|---------|
| **项目管理** | project, "user", user_role, module_ownership | ❌ 全部未实现 |
| **Pipeline 执行** | pipeline, pipeline_stage, checkpoint | ✅ pipeline; ❌ stage; ⚠️ checkpoint |
| **审批门禁** | gate_event, gate_request | ✅ gate_event; ❌ gate_request |
| **🗣️ 对话历史** | **conversation_message**, **conversation_branch** | ❌ **全部未实现** |
| **文件锁** | file_lock | ✅ 完全实现 |
| **Token & 预算** | token_usage, cost_quota | ⚠️ 只读聚合 |
| **审计** | audit_log | ✅ WORM 哈希链 |
| **任务队列** | task_queue | ⚠️ 表空置，实际用 Redis |
| **特性开关** | feature_flag | ❌ 未实现 |
| **学习引擎** | preference, trajectory, knowledge_snapshot, ab_experiment, ab_experiment_assignment, pipeline_retrospective | ✅ preference+trajectory; ❌ 其余 |
