package enrollment

import (
	"context"
	"time"
)

func (m *Module) InsertPhase(ctx context.Context, phase *Phase) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertPhase(ctx, phase)
}

func (m *Module) Phase(ctx context.Context, id int64) (*Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.Phase(ctx, id)
}

func (m *Module) PhasesByID(ctx context.Context, ids []int64) ([]*Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PhasesByID(ctx, ids)
}

func (m *Module) UpdatePhase(ctx context.Context, phase *Phase) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.UpdatePhase(ctx, phase)
}

func (m *Module) DeletePhase(ctx context.Context, id int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.DeletePhase(ctx, id)
}

func (m *Module) Phases(ctx context.Context) ([]*Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.Phases(ctx)
}

func (m *Module) PublicOpenPhases(ctx context.Context, now time.Time) ([]*Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PublicOpenPhases(ctx, now)
}

func (m *Module) PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.PhasesWithExpiredRolloverDeadline(ctx, asOf)
}

func (m *Module) HasActiveClassRestrictedPhase(ctx context.Context) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.HasActiveClassRestrictedPhase(ctx)
}

func (m *Module) HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.HasActiveGradeRestrictedPhase(ctx)
}

func (m *Module) MaxActivePhaseGrade(ctx context.Context) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.MaxActivePhaseGrade(ctx)
}

func (m *Module) HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.engine.HasRolloverSuccessor(ctx, sourcePhaseID)
}
