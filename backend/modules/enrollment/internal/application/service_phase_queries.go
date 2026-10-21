package application

import (
	"context"
)

// CountPhaseSchemaReferences counts phases pinned to any of the supplied versions.
func (m *Service) CountPhaseSchemaReferences(ctx context.Context, schemaIDs []int64) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.CountPhaseSchemaReferences(ctx, schemaIDs)
}

// RepointPhaseSchemas advances phase form pins within the caller's school.
// A publication workflow supplies its transaction to keep insertion and repointing atomic.
func (m *Service) RepointPhaseSchemas(ctx context.Context, fromIDs []int64, toID int64) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.RepointPhaseSchemas(ctx, fromIDs, toID)
}

// PhaseCountsByCalendarPeriod reports phase references for the caller's school.
// Periods without phase references are omitted.
func (m *Service) PhaseCountsByCalendarPeriod(ctx context.Context) (map[int64]int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhaseCountsByCalendarPeriod(ctx)
}
