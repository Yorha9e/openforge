package compliance

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"openforge/internal/shared/kernel"
)

// Scheduler runs the four monthly compliance reports on the 1st of
// each month at 02:00 (local time) and sends the resulting report as
// a notification via the configured Notifier.
type Scheduler struct {
	gen      *ReportGenerator
	notifier kernel.Notifier
	target   kernel.Target
	stopCh   chan struct{}
	done     chan struct{}
}

// NewScheduler creates a Scheduler bound to the given generator and
// notifier. The target is the address (e.g. webhook URL) that will
// receive notifications.
func NewScheduler(db *sql.DB, n kernel.Notifier, target kernel.Target) *Scheduler {
	return &Scheduler{
		gen:      NewReportGenerator(db, nil),
		notifier: n,
		target:   target,
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// NewSchedulerWithGenerator is a test seam allowing a pre-built
// ReportGenerator to be injected (e.g. with a fake querier).
func NewSchedulerWithGenerator(gen *ReportGenerator, n kernel.Notifier, target kernel.Target) *Scheduler {
	return &Scheduler{
		gen:      gen,
		notifier: n,
		target:   target,
		stopCh:   make(chan struct{}),
		done:     make(chan struct{}),
	}
}

// Start launches the background loop. It first sleeps until the next
// 02:00 on the 1st of a month, then runs every report type and
// repeats.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		defer close(s.done)
		for {
			next := nextRun(time.Now())
			wait := time.Until(next)
			slog.Info("compliance scheduler: next run", "at", next.Format(time.RFC3339), "in", wait.String())

			timer := time.NewTimer(wait)
			select {
			case <-s.stopCh:
				timer.Stop()
				slog.Info("compliance scheduler stopped")
				return
			case <-ctx.Done():
				timer.Stop()
				slog.Info("compliance scheduler cancelled by context")
				return
			case <-timer.C:
				s.runOnce(ctx, next)
			}
		}
	}()
}

// Stop signals the scheduler to stop and waits for the goroutine to
// exit.
func (s *Scheduler) Stop() {
	close(s.stopCh)
	<-s.done
}

// runOnce generates every report for the month ending at `at` and
// notifies the configured target. Errors are logged but do not stop
// the loop.
func (s *Scheduler) runOnce(ctx context.Context, at time.Time) {
	// The month we are reporting on is the calendar month that just
	// ended. If the run fires at 02:00 on the 1st, the previous
	// month is the correct window.
	month := previousMonth(at)

	for _, t := range AllReportTypes {
		rep, err := s.gen.Generate(ctx, t, month)
		if err != nil {
			slog.Error("compliance report generation failed",
				"type", string(t), "month", month, "error", err)
			continue
		}
		slog.Info("compliance report generated",
			"type", string(t), "month", month, "sections", len(rep.Sections))

		if s.notifier == nil {
			continue
		}
		msg := kernel.Notification{
			Title: fmt.Sprintf("[%s] %s — %s", rep.Month, rep.Title, string(rep.Type)),
			Body:  formatReportBody(rep),
		}
		if err := s.notifier.Send(ctx, s.target, msg); err != nil {
			slog.Error("compliance notify failed",
				"type", string(t), "month", month, "error", err)
		}
	}
}

// RunNow triggers a single monthly cycle immediately. Useful for
// tests and for first-run warm-up.
func (s *Scheduler) RunNow(ctx context.Context, month string) error {
	for _, t := range AllReportTypes {
		rep, err := s.gen.Generate(ctx, t, month)
		if err != nil {
			return fmt.Errorf("compliance: run now %s/%s: %w", month, t, err)
		}
		slog.Info("compliance report generated (manual)",
			"type", string(t), "month", month, "sections", len(rep.Sections))
	}
	return nil
}

// nextRun returns the next 02:00 on the 1st of a month at or after
// `from`. If `from` is already past 02:00 on the 1st, it returns the
// following month.
func nextRun(from time.Time) time.Time {
	year, month, _ := from.Date()
	first := time.Date(year, month, 1, 2, 0, 0, 0, from.Location())
	if !first.After(from) {
		first = first.AddDate(0, 1, 0)
	}
	return first
}

// previousMonth returns the YYYY-MM string for the calendar month
// preceding `at`.
func previousMonth(at time.Time) string {
	first := time.Date(at.Year(), at.Month(), 1, 0, 0, 0, 0, at.Location())
	prev := first.AddDate(0, 0, -1)
	return prev.Format("2006-01")
}

// formatReportBody renders a short text summary of a report.
func formatReportBody(r *Report) string {
	if r == nil {
		return ""
	}
	out := ""
	for _, sec := range r.Sections {
		out += sec.Title + ":\n"
		for _, row := range sec.Rows {
			out += fmt.Sprintf("  - %s: %d\n", row.Label, row.Count)
		}
	}
	return out
}
