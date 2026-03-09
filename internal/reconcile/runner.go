// Package reconcile defines scheduled reconciliation and audit tasks.
package reconcile

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"imsub/internal/events"
)

// ErrInvalidInterval indicates that a schedule interval is not strictly positive.
var ErrInvalidInterval = errors.New("reconcile: invalid interval")

// Task is a named reconciliation or audit unit.
type Task interface {
	Name() string
	Run(ctx context.Context) error
	Classify(err error) string
}

// Schedule configures how a task should be run.
type Schedule struct {
	Task         Task
	InitialDelay time.Duration
	Interval     time.Duration
}

// Runner executes scheduled reconciliation tasks and emits shared job events.
type Runner struct {
	logger *slog.Logger
	events events.EventSink
}

// NewRunner creates a reconciliation runner.
func NewRunner(logger *slog.Logger, sink events.EventSink) *Runner {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runner{logger: logger, events: sink}
}

// RunScheduled runs a scheduled task until ctx is done.
func (r *Runner) RunScheduled(ctx context.Context, schedule Schedule) error {
	if schedule.Interval <= 0 {
		r.logger.Warn("reconciliation task not started: non-positive interval", "task", taskName(schedule.Task), "interval", schedule.Interval)
		return ErrInvalidInterval
	}
	if schedule.InitialDelay > 0 {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(schedule.InitialDelay):
		}
	}

	r.runTask(ctx, schedule.Task)

	ticker := time.NewTicker(schedule.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			r.runTask(ctx, schedule.Task)
		}
	}
}

func (r *Runner) runTask(ctx context.Context, task Task) {
	if task == nil {
		return
	}
	start := time.Now()
	result := "ok"
	err := task.Run(ctx)
	if err != nil {
		result = task.Classify(err)
		r.logger.Warn("reconciliation task failed", "task", task.Name(), "error", err, "result", result)
	}
	if r.events != nil {
		r.events.Emit(ctx, events.Event{
			Name:     events.NameBackgroundJob,
			Outcome:  result,
			Fields:   map[string]string{"job": task.Name()},
			Duration: time.Since(start),
		})
	}
}

func taskName(task Task) string {
	if task == nil {
		return "unknown"
	}
	return task.Name()
}
