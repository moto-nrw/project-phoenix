package owners

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// TimetableStaffDeviationDependencies binds the deviation writes to the
// realtime hub over the retained rows (Instances, InstanceStaff and the Audit
// Platform's DeviationEvents). lifecycle is the instance lifecycle the cancel
// branch and the "deliberately unstaffed" acknowledgement hand over to. The
// composition root adds its database handle to the dependencies they bind.
func TimetableStaffDeviationDependencies(
	rows timetableCompose.OwnerRows, staff timetablePorts.DeviationStaff, supervisions studentpresence.SupervisionRecords,
	lifecycle timetablePorts.DeviationLifecycle, broadcaster realtime.Broadcaster, logger *slog.Logger, now func() time.Time,
) (timetablePorts.StaffDeviationDependencies, error) {
	if lifecycle == nil || rows.Deviations == nil || broadcaster == nil {
		return timetablePorts.StaffDeviationDependencies{}, errors.New("timetable staff deviations: lifecycle, protocol and broadcaster are required")
	}
	return timetablePorts.StaffDeviationDependencies{
		Instances:       rows.Instances,
		InstanceStaff:   rows.InstanceStaff,
		Staff:           staff,
		Supervisions:    supervisions,
		Lifecycle:       lifecycle,
		Protocol:        rows.Deviations,
		ActivityUpdates: timetableActivityUpdates{broadcaster: broadcaster},
		Logger:          logger,
		Now:             now,
	}, nil
}

// NewTimetableAttendanceMirror binds Student Presence's attendance syncer
// port to the Timetable owner's attendance mirror over the rows' Instances
// and Participants: Presence triggers the Timetable command inside its own
// tenant transaction and rolls its visit write back with a failing mirror.
// The Presence behaviour suites bind it the same way.
func NewTimetableAttendanceMirror(rows timetableCompose.OwnerRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
	return NewNativeAttendanceMirror(timetableCompose.NewNativeMirrorRows(rows), logger)
}

func NewNativeAttendanceMirror(rows timetableCompose.NativeMirrorRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
	var instances *timetablePorts.MirrorInstances
	var participants *timetablePorts.MirrorParticipants
	if rows.Instances != nil {
		instances = rows.Instances.MirrorInstances
	}
	if rows.Participants != nil {
		participants = rows.Participants.MirrorParticipants
	}
	mirror, err := timetableCompose.NewAttendanceMirror(timetablePorts.AttendanceMirrorDependencies{
		Instances: instances, Participants: participants, Logger: logger,
	})
	if err != nil {
		return nil, err
	}
	return PresenceAttendanceMirror{Mirror: mirror}, nil
}

// presenceAttendanceMirror maps Student Presence's visits onto the mirror's
// own vocabulary. A missing visit is a no-op, as before the move.
type PresenceAttendanceMirror struct {
	Mirror timetable.AttendanceMirror
}

func attendanceVisitOf(visit *studentpresence.Visit) timetable.AttendanceVisit {
	return timetable.AttendanceVisit{
		StudentID: visit.StudentID, ActiveGroupID: visit.ActiveGroupID,
		EntryTime: visit.EntryTime, ExitTime: visit.ExitTime,
	}
}

func presenceSnapshotOf(snapshot *timetable.AttendanceSnapshot) *studentpresence.AttendanceSnapshot {
	if snapshot == nil {
		return nil
	}
	return &studentpresence.AttendanceSnapshot{
		Status: snapshot.Status, Substatus: snapshot.Substatus, Note: snapshot.Note,
		InstanceID: snapshot.InstanceID, IsUnplanned: snapshot.IsUnplanned,
	}
}

func (m PresenceAttendanceMirror) MirrorCheckInForVisit(ctx context.Context, visit *studentpresence.Visit) (*studentpresence.AttendanceSnapshot, error) {
	if visit == nil {
		return nil, nil
	}
	snapshot, err := m.Mirror.MirrorCheckInForVisit(ctx, attendanceVisitOf(visit))
	return presenceSnapshotOf(snapshot), err
}

func (m PresenceAttendanceMirror) MirrorCheckInAt(ctx context.Context, studentID int64, at time.Time) (*studentpresence.AttendanceSnapshot, error) {
	snapshot, err := m.Mirror.MirrorCheckInAt(ctx, studentID, at)
	return presenceSnapshotOf(snapshot), err
}

func (m PresenceAttendanceMirror) MirrorCheckOutForVisit(ctx context.Context, visit *studentpresence.Visit) (*studentpresence.AttendanceSnapshot, error) {
	if visit == nil {
		return nil, nil
	}
	snapshot, err := m.Mirror.MirrorCheckOutForVisit(ctx, attendanceVisitOf(visit))
	return presenceSnapshotOf(snapshot), err
}

func (m PresenceAttendanceMirror) MirrorVisitRevision(ctx context.Context, previous, updated *studentpresence.Visit) error {
	if previous == nil || updated == nil {
		return nil
	}
	return m.Mirror.MirrorVisitRevision(ctx, attendanceVisitOf(previous), attendanceVisitOf(updated))
}

func (m PresenceAttendanceMirror) MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) error {
	return m.Mirror.MirrorCheckOutAt(ctx, studentID, at)
}

func (m PresenceAttendanceMirror) MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error {
	return m.Mirror.MirrorCheckInAtBatch(ctx, studentIDs, at)
}

func (m PresenceAttendanceMirror) MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error {
	return m.Mirror.MirrorCheckOutAtBatch(ctx, studentIDs, at)
}

func (m PresenceAttendanceMirror) MirrorCheckOutForVisits(ctx context.Context, visits []*studentpresence.Visit, at time.Time) error {
	mapped := make([]timetable.AttendanceVisit, 0, len(visits))
	for _, visit := range visits {
		if visit != nil {
			mapped = append(mapped, attendanceVisitOf(visit))
		}
	}
	return m.Mirror.MirrorCheckOutForVisits(ctx, mapped, at)
}

// timetableActivityUpdates delivers the owner's activity updates through the
// realtime hub. The event is a refetch trigger carrying only the slot
// identity.
type timetableActivityUpdates struct {
	broadcaster realtime.Broadcaster
}

func (a timetableActivityUpdates) AnnounceActivityUpdate(tenantID, activeGroupID int64, update timetable.TouchedActivity) error {
	activeGroupIDStr := strconv.FormatInt(activeGroupID, 10)
	instanceID := strconv.FormatInt(update.InstanceID, 10)
	instanceDate := update.Date.String()
	instanceStart := update.StartTime.Format("15:04:05")
	event := realtime.NewEvent(realtime.EventActivityUpdate, activeGroupIDStr, realtime.EventData{
		InstanceID:        &instanceID,
		InstanceDate:      &instanceDate,
		InstanceStartTime: &instanceStart,
	})
	return a.broadcaster.BroadcastToGroup(tenantID, activeGroupIDStr, event)
}
