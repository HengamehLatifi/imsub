package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"imsub/internal/core"
	"imsub/internal/events"
)

const taskResultFailed = "failed"

type subscriberReconciler interface {
	ReconcileSubscribersOnce(ctx context.Context) error
}

type eventSubReconciler interface {
	ReconcileEventSubsOnce(ctx context.Context) error
}

type subscriberTask struct {
	reconciler subscriberReconciler
}

// NewSubscriberTask builds the subscriber-cache reconciliation task.
func NewSubscriberTask(r subscriberReconciler) Task {
	return subscriberTask{reconciler: r}
}

func (t subscriberTask) Name() string { return "reconcile_subscribers" }

func (t subscriberTask) Run(ctx context.Context) error {
	if t.reconciler == nil {
		return nil
	}
	if err := t.reconciler.ReconcileSubscribersOnce(ctx); err != nil {
		return fmt.Errorf("reconcile subscribers once: %w", err)
	}
	return nil
}

func (t subscriberTask) Classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, core.ErrListActiveCreators):
		return "list_active_creators_failed"
	case errors.Is(err, core.ErrPartialReconcile):
		return "partial_failure"
	default:
		return taskResultFailed
	}
}

type eventSubTask struct {
	reconciler eventSubReconciler
}

// NewEventSubTask builds the EventSub reconciliation task.
func NewEventSubTask(r eventSubReconciler) Task {
	return eventSubTask{reconciler: r}
}

func (t eventSubTask) Name() string { return "reconcile_eventsubs" }

func (t eventSubTask) Run(ctx context.Context) error {
	if t.reconciler == nil {
		return nil
	}
	if err := t.reconciler.ReconcileEventSubsOnce(ctx); err != nil {
		return fmt.Errorf("reconcile eventsubs once: %w", err)
	}
	return nil
}

func (t eventSubTask) Classify(err error) string {
	if err != nil {
		return taskResultFailed
	}
	return "ok"
}

type integrityAuditStore interface {
	ListCreators(ctx context.Context) ([]core.Creator, error)
	ActiveCreatorIDsWithoutGroup(ctx context.Context, creators []core.Creator) (int, error)
	RepairTrackedGroupReverseIndex(ctx context.Context) (indexUsers, repairedUsers, missingLinks, staleLinks int, err error)
}

type integrityAuditTask struct {
	store  integrityAuditStore
	logger *slog.Logger
	events events.EventSink
}

// NewIntegrityAuditTask builds the integrity audit and repair task.
func NewIntegrityAuditTask(store integrityAuditStore, logger *slog.Logger, sink events.EventSink) Task {
	if logger == nil {
		logger = slog.Default()
	}
	return integrityAuditTask{store: store, logger: logger, events: sink}
}

func (t integrityAuditTask) Name() string { return "integrity_audit" }

func (t integrityAuditTask) Classify(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errListCreators):
		return "list_creators_failed"
	case errors.Is(err, errReadActiveSet):
		return "active_set_read_failed"
	case errors.Is(err, errRepairTrackedIndex):
		return "tracked_reverse_index_repair_failed"
	default:
		return taskResultFailed
	}
}

var (
	errListCreators       = errors.New("list creators failed")
	errReadActiveSet      = errors.New("read active creator set failed")
	errRepairTrackedIndex = errors.New("repair tracked reverse index failed")
)

func (t integrityAuditTask) Run(ctx context.Context) error {
	if t.store == nil {
		return nil
	}

	creators, err := t.store.ListCreators(ctx)
	if err != nil {
		t.logger.Warn("integrity audit list creators failed", "error", err)
		return fmt.Errorf("list creators: %w", errors.Join(errListCreators, err))
	}

	activeNoGroup, err := t.store.ActiveCreatorIDsWithoutGroup(ctx, creators)
	if err != nil {
		t.logger.Warn("integrity audit active creator set read failed", "error", err)
		return fmt.Errorf("read active creator set: %w", errors.Join(errReadActiveSet, err))
	}

	indexUsers, repairedUsers, missingLinks, staleLinks, err := t.store.RepairTrackedGroupReverseIndex(ctx)
	if err != nil {
		t.logger.Warn("integrity audit tracked reverse index repair failed", "error", err)
		return fmt.Errorf("repair tracked reverse index: %w", errors.Join(errRepairTrackedIndex, err))
	}

	reconnectRequired := 0
	for _, creator := range creators {
		if creator.AuthStatus == core.CreatorAuthReconnectRequired {
			reconnectRequired++
		}
	}

	t.logger.Info("integrity audit done",
		"creators", len(creators),
		"active_without_group", activeNoGroup,
		"creators_reconnect_required", reconnectRequired,
		"index_users", indexUsers,
		"index_repaired_users", repairedUsers,
		"index_missing_links", missingLinks,
		"index_stale_links", staleLinks,
	)

	if t.events != nil {
		t.emitTrackedReverseIndexCount(ctx, "ok", repairedUsers)
		t.emitTrackedReverseIndexCount(ctx, "missing_links", missingLinks)
		t.emitTrackedReverseIndexCount(ctx, "stale_links", staleLinks)
		t.emitTrackedReverseIndexCount(ctx, "indexed_users", indexUsers)
	}
	return nil
}

func (t integrityAuditTask) emitTrackedReverseIndexCount(ctx context.Context, outcome string, count int) {
	if t.events == nil || count <= 0 {
		return
	}
	t.events.Emit(ctx, events.Event{
		Name:    events.NameReconciliationRepair,
		Outcome: outcome,
		Fields:  map[string]string{"repair": "tracked_reverse_index"},
		Count:   count,
	})
}
