package enrollment

import (
	"context"
	"time"
)

func (m *Module) InsertLateInvite(ctx context.Context, invite *LateInvite) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.InsertLateInvite(ctx, invite) })
}
func (m *Module) UsableLateInvite(ctx context.Context, tokenHash string, phaseID int64, now time.Time, lock bool) (*LateInvite, error) {
	var result *LateInvite
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.UsableLateInvite(ctx, tokenHash, phaseID, now, lock)
		return err
	})
	return result, err
}
func (m *Module) LateInviteByUsedRequestID(ctx context.Context, requestID int64) (*LateInvite, error) {
	var result *LateInvite
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.LateInviteByUsedRequestID(ctx, requestID)
		return err
	})
	return result, err
}
func (m *Module) MarkLateInviteUsed(ctx context.Context, inviteID, requestID int64, at time.Time) error {
	return m.transactions.RunInTx(ctx, func(ctx context.Context) error { return m.engine.MarkLateInviteUsed(ctx, inviteID, requestID, at) })
}
func (m *Module) DeleteLateInvitesByUsedRequestID(ctx context.Context, requestID int64) (int64, error) {
	var result int64
	err := m.transactions.RunInTx(ctx, func(ctx context.Context) error {
		var err error
		result, err = m.engine.DeleteLateInvitesByUsedRequestID(ctx, requestID)
		return err
	})
	return result, err
}
