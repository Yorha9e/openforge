package application

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"openforge/internal/agent/domain"
	"openforge/internal/agent/port"
	"openforge/internal/llm"
)

// LearningService orchestrates the learning feedback loop: retrospective
// generation, LLM-driven lesson extraction, and A/B experiment assignment
// for every completed pipeline (§3.12, Phase 8.4).
type LearningService struct {
	trajStore  domain.TrajectoryStore
	retroStore domain.RetrospectiveStore
	prefStore  domain.PreferenceStore
	expStore   domain.ExperimentStore
	llmRouter  *llm.Router
	retroGen   *domain.RetrospectiveGenerator
}

// NewLearningService creates a LearningService with all required dependencies.
// llmRouter may be nil — the service falls back to rule-based lesson extraction.
func NewLearningService(
	trajStore domain.TrajectoryStore,
	retroStore domain.RetrospectiveStore,
	prefStore domain.PreferenceStore,
	expStore domain.ExperimentStore,
	llmRouter *llm.Router,
) *LearningService {
	return &LearningService{
		trajStore:  trajStore,
		retroStore: retroStore,
		prefStore:  prefStore,
		expStore:   expStore,
		llmRouter:  llmRouter,
		retroGen:   domain.NewRetrospectiveGenerator(retroStore, trajStore, prefStore),
	}
}

// HandlePipelineCompleted is the async callback triggered when a pipeline
// finishes execution (status = completed/rejected/cancelled). It spawns a
// goroutine to:
//  1. Retrieve the pipeline's trajectory record.
//  2. Generate a rule-based retrospective via RetrospectiveGenerator.
//  3. If an LLM router is available, enhance the lessons through LLM analysis.
//  4. Persist LLM-derived lessons as preferences for future pipelines.
func (s *LearningService) HandlePipelineCompleted(ctx context.Context, pipelineID string) {
	go func() {
		bgCtx := context.Background()

		traj, err := s.trajStore.GetByPipeline(bgCtx, pipelineID)
		if err != nil || traj == nil {
			return
		}

		// Step 1: Generate base retrospective using existing generator.
		_, err = s.retroGen.Generate(bgCtx, traj.ProjectID, pipelineID)
		if err != nil {
			return
		}

		// Step 2: Enhance with LLM-driven deep analysis when available.
		if s.llmRouter != nil && len(traj.FailureCodes) > 0 {
			llmLessons, llmActions := s.analyzeTrajectoryWithLLM(bgCtx, traj)
			// Record LLM-derived lessons as preferences for future pipelines.
			for _, lesson := range llmLessons {
				_ = s.prefStore.Upsert(bgCtx, domain.PreferenceRecord{
					ProjectID: traj.ProjectID,
					Key:       "llm_lesson",
					Value:     lesson,
					Weight:    0.1,
					Source:    "auto_detect",
				})
			}
			for _, action := range llmActions {
				_ = s.prefStore.Upsert(bgCtx, domain.PreferenceRecord{
					ProjectID: traj.ProjectID,
					Key:       "llm_action",
					Value:     action,
					Weight:    0.1,
					Source:    "auto_detect",
				})
			}
		}
	}()
}

// AssignCohort routes a new pipeline to Cohort A or B for all active A/B experiments.
// Returns the assigned cohorts keyed by experiment ID.
func (s *LearningService) AssignCohort(ctx context.Context, pipelineID string) (map[string]string, error) {
	active, err := s.expStore.ListActive(ctx)
	if err != nil || len(active) == 0 {
		return nil, err
	}

	assignments := make(map[string]string, len(active))
	for _, exp := range active {
		cohort := "A"
		// Randomize: Cohort A with probability cohort_a_ratio, Cohort B otherwise.
		if rand.Float64() >= exp.CohortARatio {
			cohort = "B"
		}
		a := &domain.ABExperimentAssignment{
			ExperimentID: exp.ID,
			PipelineID:   pipelineID,
			Cohort:       cohort,
		}
		if err := s.expStore.Assign(ctx, a); err != nil {
			return assignments, fmt.Errorf("assign cohort for experiment %s: %w", exp.ID, err)
		}
		assignments[exp.ID] = cohort
	}
	return assignments, nil
}

// EvaluateExperiment completes an active experiment by collecting cohort
// results and running a statistical t-test to determine the verdict.
func (s *LearningService) EvaluateExperiment(ctx context.Context, experimentID string) (*domain.ABExperiment, error) {
	exp, err := s.expStore.Get(ctx, experimentID)
	if err != nil || exp == nil {
		return nil, fmt.Errorf("experiment %s not found: %w", experimentID, err)
	}
	if exp.Status != "running" {
		return nil, fmt.Errorf("experiment %s is not running (status=%s)", experimentID, exp.Status)
	}

	// Collect CAR (code acceptance rate) per cohort.
	cohortA, cohortB, err := s.getCohortResults(ctx, experimentID)
	if err != nil {
		return nil, err
	}

	// Run t-test.
	pValue, effectSize := domain.SimpleTTest(cohortA, cohortB)
	verdict := domain.DetermineVerdict(pValue, effectSize)
	if verdict == "" {
		return exp, nil // insufficient data
	}

	if err := s.expStore.Complete(ctx, experimentID, verdict, pValue, effectSize); err != nil {
		return nil, err
	}
	exp.Verdict = verdict
	exp.PValue = pValue
	exp.EffectSize = effectSize
	exp.Status = "completed"
	return exp, nil
}

// getCohortResults collects code acceptance rates grouped by cohort.
func (s *LearningService) getCohortResults(ctx context.Context, experimentID string) ([]float64, []float64, error) {
	if pgStore, ok := s.expStore.(interface {
		GetCohortResults(ctx context.Context, experimentID string) ([]float64, []float64, error)
	}); ok {
		return pgStore.GetCohortResults(ctx, experimentID)
	}
	return nil, nil, nil
}

// analyzeTrajectoryWithLLM sends the trajectory summary to an LLM for
// deep analysis and lesson extraction. Returns lessons and actions.
func (s *LearningService) analyzeTrajectoryWithLLM(ctx context.Context, t *domain.TrajectoryRecord) ([]string, []string) {
	prompt := buildTrajectoryAnalysisPrompt(t)
	resp, err := s.llmRouter.Chat(ctx, port.ChatRequest{
		Messages: []port.Message{
			{Role: "user", Content: prompt},
		},
		SystemPrompt: "你是一个软件工程分析专家。请从 Pipeline 执行轨迹中提炼经验教训和改进行动。",
		Config: port.LLMConfig{
			Model:     "claude-sonnet-4",
			MaxTokens: 1024,
		},
	})
	if err != nil {
		return nil, nil
	}

	lessons, actions := parseLLMAnalysis(resp.Content)
	return lessons, actions
}

// buildTrajectoryAnalysisPrompt constructs a prompt that summarises the
// trajectory for LLM analysis.
func buildTrajectoryAnalysisPrompt(t *domain.TrajectoryRecord) string {
	var sb strings.Builder
	sb.WriteString("请分析以下 Pipeline 执行轨迹，提炼出经验教训和改进行动：\n\n")
	sb.WriteString(fmt.Sprintf("- 项目: %s\n", t.ProjectID))
	sb.WriteString(fmt.Sprintf("- Pipeline: %s\n", t.PipelineID))
	sb.WriteString(fmt.Sprintf("- 阶段序列: %s\n", strings.Join(t.StageSequence, " -> ")))
	sb.WriteString(fmt.Sprintf("- 对话轮次: %d\n", t.TotalChatRounds))
	sb.WriteString(fmt.Sprintf("- Token 总量: %d\n", t.TotalTokens))
	sb.WriteString(fmt.Sprintf("- 回溯次数: %d\n", t.BacktrackCount))
	sb.WriteString(fmt.Sprintf("- 拒绝次数: %d\n", t.RejectionCount))

	if len(t.FailureCodes) > 0 {
		sb.WriteString(fmt.Sprintf("- 失败码: %s\n", strings.Join(t.FailureCodes, ", ")))
	}
	if len(t.SuccessfulPatterns) > 0 {
		sb.WriteString(fmt.Sprintf("- 成功模式: %s\n", strings.Join(t.SuccessfulPatterns, ", ")))
	}
	if len(t.ToolsUsed) > 0 {
		sb.WriteString(fmt.Sprintf("- 使用工具: %s\n", strings.Join(t.ToolsUsed, ", ")))
	}

	sb.WriteString("\n请返回 JSON 格式：\n")
	sb.WriteString(`{"lessons_learned": ["教训1", "教训2"], "improvement_actions": ["行动1", "行动2"]}`)
	sb.WriteString("\n请只返回 JSON，不要包含其他文本。")
	return sb.String()
}

// parseLLMAnalysis extracts lessons and actions from an LLM response.
// It handles both valid JSON and plain-text fallback.
func parseLLMAnalysis(content string) (lessons []string, actions []string) {
	// Strip markdown code fences if present.
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```json")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	// Simple line-based fallback: treat non-structural lines as lessons.
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "{" || line == "}" {
			continue
		}
		line = strings.Trim(line, `",`)
		if strings.Contains(line, "lessons_learned") || strings.Contains(line, "improvement_actions") {
			continue
		}
		if strings.HasPrefix(line, "{") || strings.HasPrefix(line, "}") {
			continue
		}
		line = strings.TrimPrefix(line, `"`)
		line = strings.TrimSuffix(line, `"`)
		line = strings.TrimSuffix(line, `",`)
		line = strings.Trim(line, `"`)
		if len(line) > 3 {
			lessons = append(lessons, line)
		}
	}
	return lessons, actions
}
