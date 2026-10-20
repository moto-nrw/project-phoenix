package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *Service) InsertLateInvite(ctx context.Context, invite *enrollment.LateInvite) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.InsertLateInvite(ctx, invite)
}

func (m *Service) UsableLateInvite(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*enrollment.LateInvite, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.UsableLateInvite(ctx, tokenHash, phaseID, now, lock)
}

func (m *Service) LateInviteByUsedRequestID(ctx context.Context, requestID int64) (*enrollment.LateInvite, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.store.LateInviteByUsedRequestID(ctx, requestID)
}

func (m *Service) MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, at time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.store.MarkLateInviteUsed(ctx, inviteID, requestID, at)
}

// DeleteLateInvitesSettledBefore deletes the invitations settled before
// cutoff: unused ones that expired before it, and used ones whose request is
// gone and that were used before it (#3960).
func (m *Service) DeleteLateInvitesSettledBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.DeleteLateInvitesSettledBefore(ctx, cutoff)
}

func (m *Service) DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.store.DeleteLateInvitesByUsedRequestID(ctx, requestID)
}
