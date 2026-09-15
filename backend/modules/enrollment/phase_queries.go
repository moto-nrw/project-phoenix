package enrollment

import "context"

// OpenPhaseCandidates supplies the parent workflow's phase discovery candidates.
// Unlike the anonymous list, it includes restricted audiences for that workflow
// to authorize against guardian relationships. Cross-school discovery requires
// the caller's authorized admin transaction.
func (m *Module) OpenPhaseCandidates(ctx context.Context) ([]*Phase, error) {
	return m.engine.OpenPhaseCandidates(ctx)
}

// CountPhaseSchemaReferences counts phases pinned to any of the supplied versions.
func (m *Module) CountPhaseSchemaReferences(ctx context.Context, schemaIDs []int64) (int, error) {
	var count int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		count, err = m.engine.CountPhaseSchemaReferences(txCtx, schemaIDs)
		return err
	})
	return count, err
}

// RepointPhaseSchemas advances phase form pins within the caller's school.
// A publication workflow supplies its transaction to keep insertion and repointing atomic.
func (m *Module) RepointPhaseSchemas(ctx context.Context, fromIDs []int64, toID int64) (int64, error) {
	var count int64
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		count, err = m.engine.RepointPhaseSchemas(txCtx, fromIDs, toID)
		return err
	})
	return count, err
}

// PhaseCountsByCalendarPeriod reports phase references for the caller's school.
// Periods without phase references are omitted.
func (m *Module) PhaseCountsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	var counts map[int64]int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		counts, err = m.engine.PhaseCountsByCalendarPeriod(txCtx)
		return err
	})
	return counts, err
}
