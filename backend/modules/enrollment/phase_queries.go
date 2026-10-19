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
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.CountPhaseSchemaReferences(ctx, schemaIDs)
}

// RepointPhaseSchemas advances phase form pins within the caller's school.
// A publication workflow supplies its transaction to keep insertion and repointing atomic.
func (m *Module) RepointPhaseSchemas(ctx context.Context, fromIDs []int64, toID int64) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.RepointPhaseSchemas(ctx, fromIDs, toID)
}

// PhaseCountsByCalendarPeriod reports phase references for the caller's school.
// Periods without phase references are omitted.
func (m *Module) PhaseCountsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PhaseCountsByCalendarPeriod(ctx)
}
