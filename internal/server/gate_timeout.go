package server

import (
	"context"
	"log/slog"
	"time"

	pipelineport "openforge/internal/pipeline/port"
	pipelinesvc "openforge/internal/pipeline/service"
)

// StartGateTimeoutChecker runs a background goroutine that periodically scans
// the gate_request table for pending requests whose timeout_at has passed.
// Timed-out requests are marked as 'timeout' and their pipelines are cancelled
// to prevent indefinite goroutine deadlock.
func StartGateTimeoutChecker(gateRepo pipelineport.GateRequestRepository, pipelineSvc *pipelinesvc.PipelineService) {
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			ctx := context.Background()

			pipelineIDs, err := gateRepo.HandleTimeouts(ctx)
			if err != nil {
				slog.Error("gate timeout scanner: failed to handle timeouts", "error", err)
				continue
			}

			for _, pid := range pipelineIDs {
				if err := pipelineSvc.Cancel(ctx, pid); err != nil {
					slog.Error("gate timeout scanner: failed to cancel pipeline",
						"pipeline_id", pid,
						"error", err,
					)
					continue
				}
				slog.Warn("gate timeout: pipeline cancelled due to approval timeout",
					"pipeline_id", pid,
					"timeout", "5 minutes",
				)
			}

			if len(pipelineIDs) > 0 {
				slog.Info("gate timeout scan complete", "timeout_count", len(pipelineIDs))
			}
		}
	}()
}
