// Package domain provides stage-specific templates for OpenForge Pipeline stages.
// This file contains templates for each stage (Clarify, Decompose, Implement, Test, Deploy, Verify)
// and complexity level (L1-L4).
package domain

// StageTemplates contains all stage-specific templates
var StageTemplates = map[string]map[string]*StageTemplate{
	"clarify":    clarifyTemplates,
	"decompose":  decomposeTemplates,
	"implement":  implementTemplates,
	"test":       testTemplates,
	"deploy":     deployTemplates,
	"verify":     verifyTemplates,
}

// Clarify stage templates
var clarifyTemplates = map[string]*StageTemplate{
	"L1": {
		Stage:      "clarify",
		Complexity: "L1",
		Template: `<stage_instructions stage="clarify" level="L1">
<objective>Analyze requirements and estimate complexity for atomic change.</objective>
<tasks>
<task order="1">Understand the requirement</task>
<task order="2">Identify affected files</task>
<task order="3">Estimate complexity (L1)</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files" />
<tool name="search_content" purpose="Search related code" />
</tools>
<output_format>
<requirement name="summary">Brief requirement summary (100-200 tokens)</requirement>
<requirement name="files">List of affected files</requirement>
<requirement name="complexity">L1 complexity estimate</requirement>
</output_format>
<constraints>
<constraint>Read-only operations only</constraint>
<constraint>Complexity must be L1</constraint>
</constraints>
</stage_instructions>`,
	},
	"L2": {
		Stage:      "clarify",
		Complexity: "L2",
		Template: `<stage_instructions stage="clarify" level="L2">
<objective>Analyze requirements and estimate complexity for simple modification.</objective>
<tasks>
<task order="1">Understand the requirement</task>
<task order="2">Identify affected modules</task>
<task order="3">Estimate complexity (L2)</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files" />
<tool name="search_content" purpose="Search related code" />
<tool name="analyze_topology" purpose="Analyze module dependencies" />
</tools>
<output_format>
<requirement name="summary">Requirement summary (200-300 tokens)</requirement>
<requirement name="modules">Affected modules list</requirement>
<requirement name="complexity">L2 complexity estimate</requirement>
</output_format>
<constraints>
<constraint>Read-only operations only</constraint>
<constraint>Complexity must be L2</constraint>
</constraints>
</stage_instructions>`,
	},
	"L3": {
		Stage:      "clarify",
		Complexity: "L3",
		Template: `<stage_instructions stage="clarify" level="L3">
<objective>Analyze requirements, understand context, ask clarifying questions, estimate complexity.</objective>
<tasks>
<task order="1">Analyze project structure and existing code</task>
<task order="2">Understand requirement context and constraints</task>
<task order="3">Identify potential issues and risks</task>
<task order="4">Ask clarifying questions (max 5)</task>
<task order="5">Estimate complexity level (L1-L4)</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files to understand context" />
<tool name="search_content" purpose="Search related code and configuration" />
<tool name="analyze_topology" purpose="Analyze project topology structure" />
<tool name="lsp_symbols" purpose="Get file symbol information" />
</tools>
<output_format>
<requirement name="requirement_summary">Requirement summary (200-300 tokens)</requirement>
<requirement name="constraints">Constraints list</requirement>
<requirement name="questions">Clarifying questions (max 5)</requirement>
<requirement name="complexity_estimate">Complexity estimate JSON</requirement>
<requirement name="affected_modules">Affected modules list</requirement>
</output_format>
<complexity_estimate_format>
{
  "level": "L1|L2|L3|L4",
  "reasoning": "Estimation reasoning",
  "estimated_files": 5,
  "estimated_modules": ["module1", "module2"],
  "estimated_tokens": 10000,
  "estimated_duration": "2h"
}
</complexity_estimate_format>
<constraints>
<constraint>Read-only operations, do not modify any files</constraint>
<constraint>Must ask at least one clarifying question</constraint>
<constraint>Complexity estimate must be based on actual analysis</constraint>
<constraint>Output must match specified format</constraint>
</constraints>
</stage_instructions>`,
	},
	"L4": {
		Stage:      "clarify",
		Complexity: "L4",
		Template: `<stage_instructions stage="clarify" level="L4">
<objective>Deep analysis for architecture change, comprehensive risk assessment.</objective>
<tasks>
<task order="1">Comprehensive project structure analysis</task>
<task order="2">Identify architectural impact and dependencies</task>
<task order="3">Risk assessment and mitigation strategies</task>
<task order="4">Ask clarifying questions</task>
<task order="5">Estimate complexity (L4)</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files" />
<tool name="search_content" purpose="Search related code" />
<tool name="analyze_topology" purpose="Analyze full project topology" />
<tool name="lsp_symbols" purpose="Get file symbols" />
<tool name="lsp_references" purpose="Find all references" />
</tools>
<output_format>
<requirement name="requirement_summary">Detailed requirement summary</requirement>
<requirement name="architecture_impact">Architecture impact analysis</requirement>
<requirement name="risks">Risk assessment with mitigation</requirement>
<requirement name="questions">Clarifying questions</requirement>
<requirement name="complexity_estimate">L4 complexity estimate JSON</requirement>
<requirement name="affected_modules">All affected modules</requirement>
</output_format>
<constraints>
<constraint>Read-only operations only</constraint>
<constraint>Must identify all architectural impacts</constraint>
<constraint>Risk assessment is mandatory</constraint>
<constraint>Complexity must be L4</constraint>
</constraints>
</stage_instructions>`,
	},
}

// Decompose stage templates
var decomposeTemplates = map[string]*StageTemplate{
	"L3": {
		Stage:      "decompose",
		Complexity: "L3",
		Template: `<stage_instructions stage="decompose" level="L3">
<objective>Break down requirements into sub-tasks, map affected modules, topology analysis.</objective>
<tasks>
<task order="1">Analyze requirement dependencies</task>
<task order="2">Break down into sub-tasks</task>
<task order="3">Map affected modules</task>
<task order="4">Create task dependency graph</task>
<task order="5">Estimate effort for each task</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files" />
<tool name="search_content" purpose="Search related code" />
<tool name="analyze_topology" purpose="Analyze module dependencies" />
</tools>
<output_format>
<requirement name="task_breakdown">List of sub-tasks with dependencies</requirement>
<requirement name="module_map">Module dependency map</requirement>
<requirement name="effort_estimates">Effort estimates per task</requirement>
<requirement name="risk_factors">Risk factors for each task</requirement>
</output_format>
<constraints>
<constraint>Read-only operations only</constraint>
<constraint>Tasks must be actionable and measurable</constraint>
<constraint>Dependencies must be clearly defined</constraint>
</constraints>
</stage_instructions>`,
	},
	"L4": {
		Stage:      "decompose",
		Complexity: "L4",
		Template: `<stage_instructions stage="decompose" level="L4">
<objective>Comprehensive decomposition for architecture changes with detailed risk analysis.</objective>
<tasks>
<task order="1">Deep architectural analysis</task>
<task order="2">Identify all impacted components</task>
<task order="3">Create detailed task breakdown</task>
<task order="4">Map cross-cutting concerns</task>
<task order="5">Define rollback strategies</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read relevant files" />
<tool name="search_content" purpose="Search related code" />
<tool name="analyze_topology" purpose="Full topology analysis" />
<tool name="lsp_symbols" purpose="Get file symbols" />
<tool name="lsp_references" purpose="Find all references" />
</tools>
<output_format>
<requirement name="task_breakdown">Detailed sub-tasks with dependencies</requirement>
<requirement name="architecture_impact">Full architecture impact map</requirement>
<requirement name="rollback_strategies">Rollback strategies per task</requirement>
<requirement name="risk_matrix">Risk matrix with mitigations</requirement>
</output_format>
<constraints>
<constraint>Read-only operations only</constraint>
<constraint>Must include rollback strategies</constraint>
<constraint>Architecture review required</constraint>
</constraints>
</stage_instructions>`,
	},
}

// Implement stage templates
var implementTemplates = map[string]*StageTemplate{
	"L1": {
		Stage:      "implement",
		Complexity: "L1",
		Template: `<stage_instructions stage="implement" level="L1">
<objective>Implement atomic change with minimal impact.</objective>
<tasks>
<task order="1">Acquire file lock</task>
<task order="2">Make targeted change</task>
<task order="3">Verify change</task>
<task order="4">Request Gate approval</task>
</tasks>
<tools>
<tool name="acquire_file_lock" purpose="Acquire file lock before modification" required="true" />
<tool name="read_file" purpose="Read existing code" />
<tool name="edit_file" purpose="Edit existing file" />
<tool name="bash" purpose="Run tests" />
</tools>
<code_conventions>
<convention>NO COMMENTS unless asked</convention>
<convention>Follow existing code style</convention>
<convention>Minimal changes only</convention>
</code_conventions>
<gate_approval>
<required_artifacts>
<artifact>Changed files list</artifact>
<artifact>Diff preview</artifact>
<artifact>Test results</artifact>
</required_artifacts>
</gate_approval>
<constraints>
<constraint>Must acquire file lock before modification</constraint>
<constraint>Changes must pass tests</constraint>
<constraint>Must request Gate approval</constraint>
</constraints>
</stage_instructions>`,
	},
	"L2": {
		Stage:      "implement",
		Complexity: "L2",
		Template: `<stage_instructions stage="implement" level="L2">
<objective>Implement simple modification with validation.</objective>
<tasks>
<task order="1">Acquire file locks</task>
<task order="2">Implement changes</task>
<task order="3">Add validation</task>
<task order="4">Run tests</task>
<task order="5">Request Gate approval</task>
</tasks>
<tools>
<tool name="acquire_file_lock" purpose="Acquire file locks" required="true" />
<tool name="read_file" purpose="Read existing code" />
<tool name="edit_file" purpose="Edit existing file" />
<tool name="write_file" purpose="Create new file if needed" />
<tool name="bash" purpose="Run build and tests" />
</tools>
<code_conventions>
<convention>NO COMMENTS unless asked</convention>
<convention>Follow existing code style</convention>
<convention>Check dependencies before using libraries</convention>
<convention>Prefer editing existing files</convention>
</code_conventions>
<constraints>
<constraint>Must acquire file locks before modification</constraint>
<constraint>Must add validation for user input</constraint>
<constraint>Changes must pass tests</constraint>
<constraint>Must request Gate approval</constraint>
</constraints>
</stage_instructions>`,
	},
	"L3": {
		Stage:      "implement",
		Complexity: "L3",
		Template: `<stage_instructions stage="implement" level="L3">
<objective>Implement feature development with comprehensive testing.</objective>
<tasks>
<task order="1">Analyze impact range and dependencies</task>
<task order="2">Acquire file locks</task>
<task order="3">Implement feature</task>
<task order="4">Write tests</task>
<task order="5">Run full test suite</task>
<task order="6">Request Gate approval</task>
</tasks>
<tools>
<tool name="acquire_file_lock" purpose="Acquire file locks" required="true" />
<tool name="read_file" purpose="Read existing code" />
<tool name="edit_file" purpose="Edit existing file" />
<tool name="write_file" purpose="Create new file" />
<tool name="bash" purpose="Run build, tests, and lint" />
<tool name="lsp_hover" purpose="Get symbol information" />
<tool name="lsp_definition" purpose="Go to definition" />
<tool name="lsp_references" purpose="Find all references" />
</tools>
<code_conventions>
<convention>NO COMMENTS unless asked</convention>
<convention>Follow existing code style</convention>
<convention>Check dependencies before using libraries</convention>
<convention>Prefer editing existing files over creating new ones</convention>
<convention>No backwards-compatibility hacks</convention>
<convention>Only validate at system boundaries</convention>
</code_conventions>
<security_rules>
<rule>Never expose or log secrets/keys</rule>
<rule>Never commit secrets</rule>
<rule>Follow security best practices</rule>
</security_rules>
<gate_approval>
<trigger>After code changes are complete</trigger>
<required_artifacts>
<artifact>Changed files list</artifact>
<artifact>Diff preview</artifact>
<artifact>Test results</artifact>
<artifact>Summary of changes</artifact>
</required_artifacts>
</gate_approval>
<constraints>
<constraint>Must acquire file locks before modification</constraint>
<constraint>Code changes must pass all tests</constraint>
<constraint>Must write tests for new functionality</constraint>
<constraint>Must request Gate approval after changes</constraint>
<constraint>Follow code conventions strictly</constraint>
</constraints>
</stage_instructions>`,
	},
	"L4": {
		Stage:      "implement",
		Complexity: "L4",
		Template: `<stage_instructions stage="implement" level="L4">
<objective>Implement architecture changes with comprehensive testing and documentation.</objective>
<tasks>
<task order="1">Deep impact analysis</task>
<task order="2">Acquire all necessary file locks</task>
<task order="3">Implement architecture changes</task>
<task order="4">Update documentation</task>
<task order="5">Write comprehensive tests</task>
<task order="6">Run full test suite with coverage</task>
<task order="7">Request Gate approval</task>
</tasks>
<tools>
<tool name="acquire_file_lock" purpose="Acquire file locks" required="true" />
<tool name="read_file" purpose="Read existing code" />
<tool name="edit_file" purpose="Edit existing file" />
<tool name="write_file" purpose="Create new file" />
<tool name="bash" purpose="Run build, tests, lint, and coverage" />
<tool name="lsp_hover" purpose="Get symbol information" />
<tool name="lsp_definition" purpose="Go to definition" />
<tool name="lsp_references" purpose="Find all references" />
<tool name="lsp_symbols" purpose="Get document symbols" />
</tools>
<code_conventions>
<convention>NO COMMENTS unless asked</convention>
<convention>Follow existing code style</convention>
<convention>Check dependencies before using libraries</convention>
<convention>Prefer editing existing files over creating new ones</convention>
<convention>No backwards-compatibility hacks</convention>
<convention>Only validate at system boundaries</convention>
</code_conventions>
<security_rules>
<rule>Never expose or log secrets/keys</rule>
<rule>Never commit secrets</rule>
<rule>Follow security best practices</rule>
<rule>Validate at system boundaries only</rule>
</security_rules>
<documentation>
<requirement>Update README if needed</requirement>
<requirement>Update API documentation</requirement>
<requirement>Add inline comments for complex logic</requirement>
</documentation>
<gate_approval>
<trigger>After code changes and documentation are complete</trigger>
<required_artifacts>
<artifact>Changed files list</artifact>
<artifact>Diff preview</artifact>
<artifact>Test results with coverage</artifact>
<artifact>Documentation updates</artifact>
<artifact>Architecture impact summary</artifact>
</required_artifacts>
</gate_approval>
<constraints>
<constraint>Must acquire all file locks before modification</constraint>
<constraint>Code changes must pass all tests with coverage</constraint>
<constraint>Must update documentation</constraint>
<constraint>Must request Gate approval after changes</constraint>
<constraint>Architecture review required</constraint>
</constraints>
</stage_instructions>`,
	},
}

// Test stage templates
var testTemplates = map[string]*StageTemplate{
	"L1": {
		Stage:      "test",
		Complexity: "L1",
		Template: `<stage_instructions stage="test" level="L1">
<objective>Run tests for atomic change and fix failures.</objective>
<tasks>
<task order="1">Run existing tests</task>
<task order="2">Fix any failures</task>
<task order="3">Verify all tests pass</task>
</tasks>
<tools>
<tool name="bash" purpose="Run tests" />
<tool name="read_file" purpose="Read test files" />
<tool name="edit_file" purpose="Fix test failures" />
</tools>
<constraints>
<constraint>All tests must pass</constraint>
<constraint>Fix failures before proceeding</constraint>
</constraints>
</stage_instructions>`,
	},
	"L3": {
		Stage:      "test",
		Complexity: "L3",
		Template: `<stage_instructions stage="test" level="L3">
<objective>Run comprehensive tests, fix failures, and ensure quality.</objective>
<tasks>
<task order="1">Run unit tests</task>
<task order="2">Run integration tests</task>
<task order="3">Run linting</task>
<task order="4">Fix any failures</task>
<task order="5">Verify test coverage</task>
</tasks>
<tools>
<tool name="bash" purpose="Run tests, lint, and coverage" />
<tool name="read_file" purpose="Read test files" />
<tool name="edit_file" purpose="Fix test failures" />
<tool name="search_content" purpose="Search for test patterns" />
</tools>
<constraints>
<constraint>All tests must pass</constraint>
<constraint>Linting must pass</constraint>
<constraint>Test coverage must meet threshold</constraint>
<constraint>Fix all failures before proceeding</constraint>
</constraints>
</stage_instructions>`,
	},
}

// Deploy stage templates
var deployTemplates = map[string]*StageTemplate{
	"L1": {
		Stage:      "deploy",
		Complexity: "L1",
		Template: `<stage_instructions stage="deploy" level="L1">
<objective>Deploy atomic change to staging.</objective>
<tasks>
<task order="1">Run pre-deployment checks</task>
<task order="2">Deploy to staging</task>
<task order="3">Verify deployment</task>
</tasks>
<tools>
<tool name="bash" purpose="Run deployment commands" />
<tool name="read_file" purpose="Read deployment logs" />
</tools>
<constraints>
<constraint>Deployment must succeed</constraint>
<constraint>Rollback on failure</constraint>
</constraints>
</stage_instructions>`,
	},
	"L3": {
		Stage:      "deploy",
		Complexity: "L3",
		Template: `<stage_instructions stage="deploy" level="L3">
<objective>Deploy feature to staging with verification.</objective>
<tasks>
<task order="1">Run pre-deployment dry-run</task>
<task order="2">Deploy to staging</task>
<task order="3">Run post-deployment verification</task>
<task order="4">Run smoke tests</task>
<task order="5">Rollback on failure</task>
</tasks>
<tools>
<tool name="bash" purpose="Run deployment and verification" />
<tool name="read_file" purpose="Read deployment logs" />
<tool name="manage_sandbox" purpose="Manage deployment sandbox" />
</tools>
<deploy_steps>
<step name="dry-run">Simulate deployment</step>
<step name="apply">Actual deployment</step>
<step name="verify">Health check and smoke test</step>
<step name="rollback">Rollback on failure</step>
</deploy_steps>
<constraints>
<constraint>Must run dry-run first</constraint>
<constraint>Must verify deployment health</constraint>
<constraint>Must rollback on verification failure</constraint>
</constraints>
</stage_instructions>`,
	},
}

// Verify stage templates
var verifyTemplates = map[string]*StageTemplate{
	"L1": {
		Stage:      "verify",
		Complexity: "L1",
		Template: `<stage_instructions stage="verify" level="L1">
<objective>Verify atomic change meets requirements.</objective>
<tasks>
<task order="1">Review changes</task>
<task order="2">Verify requirements met</task>
<task order="3">Write verification report</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read changed files" />
<tool name="bash" purpose="Run verification scripts" />
</tools>
<constraints>
<constraint>Must verify requirements are met</constraint>
<constraint>Must write verification report</constraint>
</constraints>
</stage_instructions>`,
	},
	"L3": {
		Stage:      "verify",
		Complexity: "L3",
		Template: `<stage_instructions stage="verify" level="L3">
<objective>PM acceptance and knowledge writeback.</objective>
<tasks>
<task order="1">Review all changes</task>
<task order="2">Verify requirements met</task>
<task order="3">Run acceptance tests</task>
<task order="4">Write verification report</task>
<task order="5">Write knowledge delta</task>
</tasks>
<tools>
<tool name="read_file" purpose="Read changed files" />
<tool name="bash" purpose="Run acceptance tests" />
<tool name="write_knowledge_delta" purpose="Write learned preferences" />
</tools>
<verification_report>
<requirement name="requirements_met">List of requirements and verification status</requirement>
<requirement name="test_results">Acceptance test results</requirement>
<requirement name="quality_metrics">Quality metrics</requirement>
<requirement name="lessons_learned">Lessons learned</requirement>
</verification_report>
<knowledge_writeback>
<requirement name="preferences">Learned preferences</requirement>
<requirement name="trajectories">Successful trajectories</requirement>
<requirement name="anti_patterns">Anti-patterns to avoid</requirement>
</knowledge_writeback>
<constraints>
<constraint>Must verify all requirements</constraint>
<constraint>Must run acceptance tests</constraint>
<constraint>Must write verification report</constraint>
<constraint>Must write knowledge delta</constraint>
<constraint>Never auto-close verification</constraint>
</constraints>
</stage_instructions>`,
	},
}

// GetStageTemplate returns the template for a specific stage and complexity level
func GetStageTemplate(stage, level string) *StageTemplate {
	if stageTemplates, ok := StageTemplates[stage]; ok {
		if template, ok := stageTemplates[level]; ok {
			return template
		}
		// Fall back to L3 if level not found
		if template, ok := stageTemplates["L3"]; ok {
			return template
		}
	}
	return nil
}

// GetAllStageTemplates returns all available stage templates
func GetAllStageTemplates() map[string]map[string]*StageTemplate {
	return StageTemplates
}

// GetStageNames returns all available stage names
func GetStageNames() []string {
	stages := make([]string, 0, len(StageTemplates))
	for stage := range StageTemplates {
		stages = append(stages, stage)
	}
	return stages
}

// GetComplexityLevels returns all available complexity levels for a stage
func GetComplexityLevels(stage string) []string {
	levels := make([]string, 0)
	if stageTemplates, ok := StageTemplates[stage]; ok {
		for level := range stageTemplates {
			levels = append(levels, level)
		}
	}
	return levels
}