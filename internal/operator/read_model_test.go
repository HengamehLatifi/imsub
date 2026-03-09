package operator

import (
	"testing"
	"time"

	"imsub/internal/events"
)

func TestReadModelProjectsJobsAndRepairs(t *testing.T) {
	t.Parallel()

	model := NewReadModel()
	model.Emit(t.Context(), events.Event{
		Name:     events.NameBackgroundJob,
		Outcome:  "ok",
		Fields:   map[string]string{"job": "integrity_audit"},
		Duration: 150 * time.Millisecond,
	})
	model.Emit(t.Context(), events.Event{
		Name:    events.NameReconciliationRepair,
		Outcome: "missing_links",
		Fields:  map[string]string{"repair": "tracked_reverse_index"},
		Count:   3,
	})
	model.Emit(t.Context(), events.Event{
		Name:  events.NameCreatorsReconnectRequired,
		Count: 2,
	})

	snap := model.Snapshot()
	if snap.CreatorsReconnectRequired != 2 {
		t.Fatalf("CreatorsReconnectRequired = %d, want 2", snap.CreatorsReconnectRequired)
	}
	if len(snap.Jobs) != 1 || snap.Jobs[0].Name != "integrity_audit" || snap.Jobs[0].LastResult != "ok" || snap.Jobs[0].Runs != 1 {
		t.Fatalf("Jobs = %+v", snap.Jobs)
	}
	if len(snap.Repairs) != 1 || snap.Repairs[0].Name != "tracked_reverse_index" || snap.Repairs[0].Outcome != "missing_links" || snap.Repairs[0].LastCount != 3 {
		t.Fatalf("Repairs = %+v", snap.Repairs)
	}
}
