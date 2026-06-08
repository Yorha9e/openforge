package domain

type MetricName string

const (
	MetricPipelineCreated    MetricName = "of_pipeline_created_total"
	MetricPipelineCompleted  MetricName = "of_pipeline_completed_total"
	MetricPipelineDuration   MetricName = "of_pipeline_duration_seconds"
	MetricLLMCallDuration    MetricName = "of_agent_llm_call_duration_seconds"
	MetricLLMCallErrors      MetricName = "of_agent_llm_call_errors_total"
	MetricTokenUsage         MetricName = "of_agent_llm_token_usage_total"
	MetricBacktrackTotal     MetricName = "of_agent_backtrack_total"
	MetricGateApproveTotal     MetricName = "of_gate_approve_total"
	MetricGateRejectTotal      MetricName = "of_gate_reject_total"
	MetricCodeAcceptanceRate   MetricName = "of_code_acceptance_rate"
	MetricGoroutineCount       MetricName = "of_coordinator_goroutine_count"
	MetricCircuitBreaker       MetricName = "of_circuit_breaker_state"
	MetricMigrationGateRefused MetricName = "openforge_migration_gate_refused_total"
	// Added in Path-D T1: 3 missing metric names to round out the 13-metric
	// observability set. Each must be registered in the exporter and wired
	// up at an actual call site.
	MetricSandboxPool         MetricName = "of_sandbox_pool_size"
	MetricLearningFallback    MetricName = "of_learning_fallback_total"
	MetricTokenQuotaRemaining MetricName = "of_token_quota_remaining"
)
