package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertPhase(ctx context.Context, phase *enrollment.Phase) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertPhase(ctx, phase)
}

func (m *Service) Phase(ctx context.Context, id int64) (*enrollment.Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.Phase(ctx, id)
}

func (m *Service) PhasesByID(ctx context.Context, ids []int64) ([]*enrollment.Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhasesByID(ctx, ids)
}

func (m *Service) UpdatePhase(ctx context.Context, phase *enrollment.Phase) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.UpdatePhase(ctx, phase)
}

func (m *Service) DeletePhase(ctx context.Context, id int64) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.DeletePhase(ctx, id)
}

func (m *Service) Phases(ctx context.Context) ([]*enrollment.Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.Phases(ctx)
}

func (m *Service) PublicOpenPhases(ctx context.Context, now time.Time) ([]*enrollment.Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PublicOpenPhases(ctx, now)
}

func (m *Service) PhasesWithExpiredRolloverDeadline(ctx context.Context, asOf time.Time) ([]*enrollment.Phase, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.PhasesWithExpiredRolloverDeadline(ctx, asOf)
}

func (m *Service) HasActiveClassRestrictedPhase(ctx context.Context) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.HasActiveClassRestrictedPhase(ctx)
}

func (m *Service) HasActiveGradeRestrictedPhase(ctx context.Context) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.HasActiveGradeRestrictedPhase(ctx)
}

func (m *Service) MaxActivePhaseGrade(ctx context.Context) (int, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.MaxActivePhaseGrade(ctx)
}

func (m *Service) HasRolloverSuccessor(ctx context.Context, sourcePhaseID int64) (bool, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return false, err
	}
	return m.store.HasRolloverSuccessor(ctx, sourcePhaseID)
}
