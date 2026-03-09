package operator

import (
	"context"
	"slices"
	"sync"
	"time"

	"imsub/internal/events"
)

// JobState summarizes the latest state of a scheduled background job.
type JobState struct {
	Name       string
	LastResult string
	LastRunAt  time.Time
	LastTook   time.Duration
	Runs       int
}

// EventCounter tracks aggregate event counts by name and outcome.
type EventCounter struct {
	Name    string
	Outcome string
	Count   int
}

// RepairState summarizes the latest observed state of a reconciliation repair stream.
type RepairState struct {
	Name       string
	Outcome    string
	LastRunAt  time.Time
	LastCount  int
	TotalCount int
}

type eventCounterKey struct {
	name    string
	outcome string
}

type repairKey struct {
	name    string
	outcome string
}

// Snapshot is the operator-facing state derived from shared events.
type Snapshot struct {
	CreatedAt                 time.Time
	CreatorsReconnectRequired int
	Jobs                      []JobState
	Repairs                   []RepairState
	Counters                  []EventCounter
}

// ReadModel projects shared events into an operator-facing in-memory snapshot.
type ReadModel struct {
	mu                        sync.RWMutex
	createdAt                 time.Time
	creatorsReconnectRequired int
	jobs                      map[string]JobState
	repairs                   map[repairKey]RepairState
	counters                  map[eventCounterKey]int
}

// NewReadModel creates an empty operator read model.
func NewReadModel() *ReadModel {
	return &ReadModel{
		createdAt: time.Now().UTC(),
		jobs:      make(map[string]JobState),
		repairs:   make(map[repairKey]RepairState),
		counters:  make(map[eventCounterKey]int),
	}
}

// Emit projects one shared event into the read model state.
func (m *ReadModel) Emit(_ context.Context, evt events.Event) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	switch evt.Name {
	case events.NameCreatorsReconnectRequired:
		m.creatorsReconnectRequired = evt.Count
	case events.NameBackgroundJob:
		jobName := evt.Fields["job"]
		if jobName != "" {
			state := m.jobs[jobName]
			state.Name = jobName
			state.LastResult = evt.Outcome
			state.LastRunAt = time.Now().UTC()
			state.LastTook = evt.Duration
			state.Runs++
			m.jobs[jobName] = state
		}
	case events.NameReconciliationRepair:
		repairName := evt.Fields["repair"]
		if repairName != "" {
			key := repairKey{name: repairName, outcome: evt.Outcome}
			state := m.repairs[key]
			state.Name = repairName
			state.Outcome = evt.Outcome
			state.LastRunAt = time.Now().UTC()
			state.LastCount = evt.Count
			state.TotalCount += evt.Count
			m.repairs[key] = state
		}
	}

	if evt.Outcome != "" {
		key := eventCounterKey{name: evt.Name, outcome: evt.Outcome}
		m.counters[key]++
	}
}

// Snapshot returns a consistent copy of the current operator-facing state.
func (m *ReadModel) Snapshot() Snapshot {
	if m == nil {
		return Snapshot{}
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]JobState, 0, len(m.jobs))
	for _, state := range m.jobs {
		jobs = append(jobs, state)
	}
	slices.SortFunc(jobs, func(a, b JobState) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		default:
			return 0
		}
	})

	repairs := make([]RepairState, 0, len(m.repairs))
	for _, state := range m.repairs {
		repairs = append(repairs, state)
	}
	slices.SortFunc(repairs, func(a, b RepairState) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		case a.Outcome < b.Outcome:
			return -1
		case a.Outcome > b.Outcome:
			return 1
		default:
			return 0
		}
	})

	counters := make([]EventCounter, 0, len(m.counters))
	for key, count := range m.counters {
		counters = append(counters, EventCounter{Name: key.name, Outcome: key.outcome, Count: count})
	}
	slices.SortFunc(counters, func(a, b EventCounter) int {
		switch {
		case a.Name < b.Name:
			return -1
		case a.Name > b.Name:
			return 1
		case a.Outcome < b.Outcome:
			return -1
		case a.Outcome > b.Outcome:
			return 1
		default:
			return 0
		}
	})

	return Snapshot{
		CreatedAt:                 m.createdAt,
		CreatorsReconnectRequired: m.creatorsReconnectRequired,
		Jobs:                      jobs,
		Repairs:                   repairs,
		Counters:                  counters,
	}
}
