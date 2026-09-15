package studentpresence

import (
	"context"
	"time"
)

type VisitMaintenance interface {
	// CloseGroupVisits uses the database clock, clamped to each entry time.
	CloseGroupVisits(context.Context, []int64) (int64, error)
	TransferOpenVisits(context.Context, int64, int64) (int64, error)
	TransferRecentDeviceVisits(context.Context, int64, int64) (int64, error)
	DeleteCompletedVisitsBefore(context.Context, int64, time.Time) (int64, error)
}

func (m *Module) CloseGroupVisits(ctx context.Context, ids []int64) (int64, error) {
	return m.engine.CloseGroupVisits(ctx, ids)
}
func (m *Module) TransferOpenVisits(ctx context.Context, from, to int64) (int64, error) {
	return m.engine.TransferOpenVisits(ctx, from, to)
}
func (m *Module) TransferRecentDeviceVisits(ctx context.Context, to, deviceID int64) (int64, error) {
	return m.engine.TransferRecentDeviceVisits(ctx, to, deviceID)
}
func (m *Module) DeleteCompletedVisitsBefore(ctx context.Context, studentID int64, cutoff time.Time) (int64, error) {
	return m.engine.DeleteCompletedVisitsBefore(ctx, studentID, cutoff)
}
