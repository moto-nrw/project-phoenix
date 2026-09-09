package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

func liveGroupToPublic(row ports.LiveGroup) studentpresence.LiveGroup {
	return studentpresence.LiveGroup{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StartTime: row.StartTime, LastActivity: row.LastActivity, EndTime: row.EndTime,
		TimeoutMinutes: row.TimeoutMinutes, ActivityGroupID: row.ActivityGroupID, DeviceID: row.DeviceID, RoomID: row.RoomID,
	}
}

func groupSessionError(err error) error {
	switch {
	case errors.Is(err, ports.ErrGroupNotFound):
		return studentpresence.ErrGroupNotFound
	case errors.Is(err, ports.ErrGroupEnded):
		return studentpresence.ErrGroupEnded
	}
	return err
}

func (e engine) LockGroup(ctx context.Context, groupID int64) (studentpresence.LiveGroup, error) {
	row, err := e.Service.LockGroup(ctx, groupID)
	if err != nil {
		return studentpresence.LiveGroup{}, groupSessionError(err)
	}
	return liveGroupToPublic(row), nil
}

func (e engine) EndGroup(ctx context.Context, groupID int64, at time.Time) error {
	return groupSessionError(e.Service.EndGroup(ctx, groupID, at))
}

func (e engine) EndGroupSessions(ctx context.Context, groupIDs []int64, at time.Time) (studentpresence.EndedGroupSessions, error) {
	row, err := e.Service.EndGroupSessions(ctx, groupIDs, at, timezone.DateFromTime(at))
	if err != nil {
		return studentpresence.EndedGroupSessions{}, err
	}
	return studentpresence.EndedGroupSessions{
		VisitsClosed: row.VisitsClosed, SessionsEnded: row.SessionsEnded, SupervisorsEnded: row.SupervisorsEnded,
		EndedActiveGroupIDs: row.EndedActiveGroupIDs,
	}, nil
}

func (e engine) EndGroupSession(ctx context.Context, groupID int64, at time.Time) (studentpresence.EndedGroupSession, error) {
	// Supervisions end on the school's calendar day of the close instant.
	row, err := e.Service.EndGroupSession(ctx, groupID, at, timezone.DateFromTime(at))
	if err != nil {
		return studentpresence.EndedGroupSession{}, groupSessionError(err)
	}
	return studentpresence.EndedGroupSession{
		GroupID: row.GroupID, EndedAt: row.EndedAt,
		ClosedVisits: visitRowsToPublic(row.ClosedVisits), EndedSupervisorIDs: row.EndedSupervisorIDs,
	}, nil
}
