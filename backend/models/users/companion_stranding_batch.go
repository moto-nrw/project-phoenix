package users

import (
	"context"
	"sort"
)

// CompanionStrandingBatch collects the "would dropping this edge strand the far
// child" verdicts of a COORDINATED multi-child write, so they can be decided
// once against the final state of every child in the batch instead of against
// the intermediate state each individual write happens to see.
//
// Why this exists (#1694): the enrollment change-request approval applies one
// approved child after the other, but a single approval is one atomic edit of
// the whole family. Two children linked only to each other, both dropping the
// accompanied mode in the same approval, are a VALID edit — after it, neither
// claims "Anderes Kind" and neither needs a "mit wem" detail. Evaluated
// per-write, the first child's write sees the second one still carrying its old
// accompanied plan, no note and no other link, and refuses the whole approval
// with ErrCompanionWouldLoseDeparture. Ordering the writes cannot fix it: the
// same argument refuses whichever child goes first.
//
// A batch defers those verdicts. The write path records them here instead of
// refusing, and the coordinating service decides them once at the end via
// StudentRepository.VerifyCompanionStrandingBatch, when every plan change and
// every edge deletion of the batch is already applied. The verdict is then the
// same one a single-child write would reach against the final state: a child
// left with an accompanied day, no note and no remaining link on that day still
// fails the batch (and rolls the whole approval back), a child whose own plan
// dropped that day passes.
//
// Scope is one transaction — the batch lives in the context of the coordinating
// write and is never shared across requests or goroutines.
type CompanionStrandingBatch struct {
	// days holds, per far child, the weekdays on which this batch removed one
	// of their links.
	days map[int64]map[string]bool
}

type companionStrandingBatchKey struct{}

// ContextWithCompanionStrandingBatch opens a batch scope and returns both the
// derived context to run the coordinated writes with and the batch itself.
// Every departure-plan write made with that context defers its stranding
// verdicts instead of refusing on the spot, so the caller MUST decide them via
// StudentRepository.VerifyCompanionStrandingBatch before it commits.
func ContextWithCompanionStrandingBatch(ctx context.Context) (context.Context, *CompanionStrandingBatch) {
	batch := &CompanionStrandingBatch{days: make(map[int64]map[string]bool)}
	return context.WithValue(ctx, companionStrandingBatchKey{}, batch), batch
}

// CompanionStrandingBatchFromContext returns the open batch, or nil when the
// write is a plain single-child one and must decide its verdicts immediately.
func CompanionStrandingBatchFromContext(ctx context.Context) *CompanionStrandingBatch {
	batch, _ := ctx.Value(companionStrandingBatchKey{}).(*CompanionStrandingBatch)
	return batch
}

// Defer records that the batch removed a link of studentID on the given
// weekdays. Purely additive: a child whose links are trimmed by several writes
// of the same batch accumulates every affected weekday.
func (b *CompanionStrandingBatch) Defer(studentID int64, days []string) {
	if b == nil || studentID <= 0 || len(days) == 0 {
		return
	}
	if b.days[studentID] == nil {
		b.days[studentID] = make(map[string]bool, len(days))
	}
	for _, day := range days {
		b.days[studentID][day] = true
	}
}

// Pending returns the deferred verdicts: the far children in ascending id order
// and, per child, the affected weekdays in the canonical PickupDayOrder (the
// only keys a stored edge can carry, see CompanionWeekdayKeys) — so the flush
// queries and, on a refusal, the refused child are deterministic.
func (b *CompanionStrandingBatch) Pending() ([]int64, map[int64][]string) {
	if b == nil || len(b.days) == 0 {
		return nil, nil
	}
	ids := make([]int64, 0, len(b.days))
	for id := range b.days {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	days := make(map[int64][]string, len(ids))
	for _, id := range ids {
		ordered := make([]string, 0, len(b.days[id]))
		for _, day := range PickupDayOrder {
			if b.days[id][day] {
				ordered = append(ordered, day)
			}
		}
		days[id] = ordered
	}
	return ids, days
}
