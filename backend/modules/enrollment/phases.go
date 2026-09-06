package enrollment

import (
	"context"
	"time"
)

func (m *Module) InsertPhase(ctx context.Context, phase *Phase) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.InsertPhase(txCtx, phase) })
}

func (m *Module) Phase(ctx context.Context, id int64) (*Phase, error) {
	var result *Phase
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error { var err error; result, err = m.engine.Phase(txCtx, id); return err })
	return result, err
}

func (m *Module) PhasesByID(ctx context.Context, ids []int64) ([]*Phase, error) {
	var result []*Phase
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.PhasesByID(txCtx, ids)
		return err
	})
	return result, err
}

func (m *Module) UpdatePhase(ctx context.Context, phase *Phase) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.UpdatePhase(txCtx, phase) })
}

func (m *Module) DeletePhase(ctx context.Context, id int64) error {
	return m.transactions.RunInTx(ctx, func(txCtx context.Context) error { return m.engine.DeletePhase(txCtx, id) })
}

func (m *Module) Phases(ctx context.Context) ([]*Phase, error) {
	var result []*Phase
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error { var err error; result, err = m.engine.Phases(txCtx); return err })
	return result, err
}

func (m *Module) PublicOpenPhases(ctx context.Context, now time.Time) ([]*Phase, error) {
	var result []*Phase
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.PublicOpenPhases(txCtx, now)
		return err
	})
	return result, err
}

func (m *Module) PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*Phase, error) {
	var result []*Phase
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.PhasesWithExpiredRolloverDeadline(txCtx, asOf)
		return err
	})
	return result, err
}

func (m *Module) HasActiveClassRestrictedPhase(ctx context.Context) (bool, error) {
	var result bool
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.HasActiveClassRestrictedPhase(txCtx)
		return err
	})
	return result, err
}

func (m *Module) HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error) {
	var result bool
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.HasActiveGradeRestrictedPhase(txCtx)
		return err
	})
	return result, err
}

func (m *Module) MaxActivePhaseGrade(ctx context.Context) (int, error) {
	var result int
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.MaxActivePhaseGrade(txCtx)
		return err
	})
	return result, err
}

func (m *Module) HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error) {
	var result bool
	err := m.transactions.RunInTx(ctx, func(txCtx context.Context) error {
		var err error
		result, err = m.engine.HasRolloverSuccessor(txCtx, sourcePhaseID)
		return err
	})
	return result, err
}
