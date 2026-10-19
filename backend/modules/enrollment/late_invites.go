package enrollment

import (
	"context"
	"time"
)

func (m *Module) InsertLateInvite(ctx context.Context, invite *LateInvite) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.InsertLateInvite(ctx, invite)
}
func (m *Module) UsableLateInvite(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*LateInvite, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.UsableLateInvite(ctx, tokenHash, phaseID, now, lock)
}
func (m *Module) LateInviteByUsedRequestID(ctx context.Context, requestID int64) (*LateInvite, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return nil, err
	}
	return m.engine.LateInviteByUsedRequestID(ctx, requestID)
}
func (m *Module) MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, at time.Time) error {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return err
	}
	return m.engine.MarkLateInviteUsed(ctx, inviteID, requestID, at)
}

// DeleteLateInvitesSettledBefore deletes the invitations settled before
// cutoff: unused ones that expired before it, and used ones whose request is
// gone and that were used before it (#3960).
func (m *Module) DeleteLateInvitesSettledBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.DeleteLateInvitesSettledBefore(ctx, cutoff)
}
func (m *Module) DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error) {
	if err := m.transactions.RequireTransaction(ctx); err != nil {
		return 0, err
	}
	return m.engine.DeleteLateInvitesByUsedRequestID(ctx, requestID)
}
