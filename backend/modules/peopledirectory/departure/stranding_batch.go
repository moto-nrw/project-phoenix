package departure

import (
	"context"
	"slices"
)

// The stranding batch lives here, with the plan whose detail is at stake,
// because the caller that opens the scope and the owner that decides the
// verdicts must both be able to name it without importing each other.

// StrandingBatch collects the verdicts a coordinated multi-child write defers.
//
// Inside such a write the far child may be another member of the same batch
// whose own plan change — the one that makes the removal legitimate — has not
// been applied yet, so deciding per write would refuse a legal edit. The batch
// owner decides every deferred verdict against the final state before it
// commits.
type StrandingBatch struct {
	days map[int64]map[string]bool
}

func NewStrandingBatch() *StrandingBatch {
	return &StrandingBatch{days: make(map[int64]map[string]bool)}
}

// Defer records that this write drops the child's link on the given weekdays.
func (b *StrandingBatch) Defer(studentID int64, days []string) {
	if b == nil || studentID <= 0 {
		return
	}
	for _, day := range days {
		if day == "" {
			continue
		}
		if b.days[studentID] == nil {
			b.days[studentID] = make(map[string]bool, len(PickupDayOrder))
		}
		b.days[studentID][day] = true
	}
}

// Pending returns the deferred children in a stable order, with the weekdays
// each of them lost, so the verdict reads the same on every run.
func (b *StrandingBatch) Pending() ([]int64, map[int64][]string) {
	if b == nil || len(b.days) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(b.days))
	for id := range b.days {
		ids = append(ids, id)
	}
	slices.Sort(ids)

	out := make(map[int64][]string, len(ids))
	for _, id := range ids {
		days := make([]string, 0, len(b.days[id]))
		for _, day := range PickupDayOrder {
			if b.days[id][day] {
				days = append(days, day)
			}
		}
		out[id] = days
	}
	return ids, out
}

type strandingBatchKey struct{}

// ContextWithStrandingBatch opens a batch scope for a coordinated multi-child
// write. The caller decides its deferred verdicts before committing.
func ContextWithStrandingBatch(ctx context.Context) (context.Context, *StrandingBatch) {
	batch := NewStrandingBatch()
	return context.WithValue(ctx, strandingBatchKey{}, batch), batch
}

// StrandingBatchFromContext returns the open batch, or nil for the ordinary
// single-child write, which decides its verdict immediately.
func StrandingBatchFromContext(ctx context.Context) *StrandingBatch {
	batch, _ := ctx.Value(strandingBatchKey{}).(*StrandingBatch)
	return batch
}
