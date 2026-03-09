package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"imsub/internal/core"
	"imsub/internal/events"
)

type store interface {
	ListCreators(ctx context.Context) ([]core.Creator, error)
	ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []core.Creator) (int, error)
	RepairTrackedGroupReverseIndex(ctx context.Context) (indexUsers, repairedUsers, missingLinks, staleLinks int, err error)
}

type reconciler interface {
	ReconcileSubscribersOnce(ctx context.Context) error
}

type eventSubReconciler interface {
	ReconcileEventSubsOnce(ctx context.Context) error
}

// ErrInvalidInterval indicates that a job loop interval is not strictly positive.
var ErrInvalidInterval = errors.New("jobs: invalid interval")

// Service runs periodic background maintenance and reconciliation jobs.
type Service struct {
	store         store
	reconcile     reconciler
	log           *slog.Logger
	obs           events.EventSink
	eventSubRecon eventSubReconciler
}

// New creates a background jobs Service.
func New(store store, reconcile reconciler, logger *slog.Logger, obs events.EventSink) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{store: store, reconcile: reconcile, log: logger, obs: obs}
}

// SetEventSubReconciler wires an EventSub reconciler into the jobs service.
func (s *Service) SetEventSubReconciler(r eventSubReconciler) {
	s.eventSubRecon = r
}

func (s *Service) logger() *slog.Logger {
	if s == nil || s.log == nil {
		return slog.Default()
	}
	return s.log
}

// RunSubscriberReconciler runs subscriber reconciliation on each tick until ctx is done.
// The first run happens after the first tick (not immediately).
//
// RunSubscriberReconciler returns ErrInvalidInterval if interval <= 0.
// If ctx is canceled, RunSubscriberReconciler returns nil.
func (s *Service) RunSubscriberReconciler(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		if s != nil {
			s.logger().Warn("subscriber reconciler not started: non-positive interval", "interval", interval)
		}
		return ErrInvalidInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.ReconcileSubscribersOnce(ctx); err != nil {
				// Reconciliation failures are expected to be transient.
				// ReconcileSubscribersOnce records metrics and logs details.
				continue
			}
		}
	}
}

// ReconcileSubscribersOnce runs one reconciliation pass and records metrics.
func (s *Service) ReconcileSubscribersOnce(ctx context.Context) error {
	if s == nil || s.reconcile == nil {
		return nil
	}
	start := time.Now()
	result := "ok"
	defer func() {
		if s.obs != nil {
			s.obs.Emit(ctx, events.Event{
				Name:     events.NameBackgroundJob,
				Outcome:  result,
				Fields:   map[string]string{"job": "reconcile_subscribers"},
				Duration: time.Since(start),
			})
		}
	}()
	if err := s.reconcile.ReconcileSubscribersOnce(ctx); err != nil {
		switch {
		case errors.Is(err, core.ErrListActiveCreators):
			result = "list_active_creators_failed"
		case errors.Is(err, core.ErrPartialReconcile):
			result = "partial_failure"
		default:
			result = "failed"
		}
		s.logger().Warn("reconcile subscribers failed", "error", err)
		return fmt.Errorf("reconcile subscribers: %w", err)
	}
	return nil
}

// RunIntegrityAudits runs integrity audits on each tick until ctx is done.
// The first run happens after the first tick (not immediately).
//
// RunIntegrityAudits returns ErrInvalidInterval if interval <= 0.
// If ctx is canceled, RunIntegrityAudits returns nil.
func (s *Service) RunIntegrityAudits(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		if s != nil {
			s.logger().Warn("integrity audits not started: non-positive interval", "interval", interval)
		}
		return ErrInvalidInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.RunIntegrityAuditOnce(ctx); err != nil {
				// Audit failures are expected to be transient.
				// RunIntegrityAuditOnce records metrics and logs details.
				continue
			}
		}
	}
}

// RunIntegrityAuditOnce runs one integrity audit pass and records metrics.
func (s *Service) RunIntegrityAuditOnce(ctx context.Context) error {
	if s == nil || s.store == nil {
		return nil
	}
	start := time.Now()
	result := "ok"
	defer func() {
		if s.obs != nil {
			s.obs.Emit(ctx, events.Event{
				Name:     events.NameBackgroundJob,
				Outcome:  result,
				Fields:   map[string]string{"job": "integrity_audit"},
				Duration: time.Since(start),
			})
		}
	}()

	creators, err := s.store.ListCreators(ctx)
	if err != nil {
		result = "list_creators_failed"
		s.logger().Warn("integrity audit ListCreators failed", "error", err)
		return fmt.Errorf("list creators: %w", err)
	}

	activeNoGroup, err := s.store.ActiveCreatorIDsWithoutGroup(ctx, creators)
	if err != nil {
		result = "active_set_read_failed"
		s.logger().Warn("integrity audit read active creators failed", "error", err)
		return fmt.Errorf("read active creator set: %w", err)
	}

	indexUsers, repairedUsers, missingLinks, staleLinks, err := s.store.RepairTrackedGroupReverseIndex(ctx)
	if err != nil {
		result = "tracked_reverse_index_repair_failed"
		s.logger().Warn("integrity audit tracked reverse index repair failed", "error", err)
		return fmt.Errorf("repair tracked reverse index: %w", err)
	}
	reconnectRequired := 0
	for _, creator := range creators {
		if creator.AuthStatus == core.CreatorAuthReconnectRequired {
			reconnectRequired++
		}
	}

	s.logger().Info("integrity audit done",
		"creators", len(creators),
		"active_without_group", activeNoGroup,
		"creators_reconnect_required", reconnectRequired,
		"index_users", indexUsers,
		"index_repaired_users", repairedUsers,
		"index_missing_links", missingLinks,
		"index_stale_links", staleLinks,
	)
	return nil
}

// RunEventSubReconciler runs EventSub reconciliation once after initialDelay and then on each interval until ctx is done.
//
// RunEventSubReconciler returns ErrInvalidInterval if interval <= 0.
func (s *Service) RunEventSubReconciler(ctx context.Context, initialDelay, interval time.Duration) error {
	if interval <= 0 {
		if s != nil {
			s.logger().Warn("eventsub reconciler not started: non-positive interval", "interval", interval)
		}
		return ErrInvalidInterval
	}

	if initialDelay > 0 {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(initialDelay):
		}
	}

	s.ReconcileEventSubsOnce(ctx)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			s.ReconcileEventSubsOnce(ctx)
		}
	}
}

// ReconcileEventSubsOnce runs one EventSub reconciliation pass and records metrics.
func (s *Service) ReconcileEventSubsOnce(ctx context.Context) {
	if s == nil || s.eventSubRecon == nil {
		return
	}
	start := time.Now()
	result := "ok"
	defer func() {
		if s.obs != nil {
			s.obs.Emit(ctx, events.Event{
				Name:     events.NameBackgroundJob,
				Outcome:  result,
				Fields:   map[string]string{"job": "reconcile_eventsubs"},
				Duration: time.Since(start),
			})
		}
	}()
	if err := s.eventSubRecon.ReconcileEventSubsOnce(ctx); err != nil {
		result = "failed"
		s.logger().Warn("reconcile eventsubs failed", "error", err)
	}
}
