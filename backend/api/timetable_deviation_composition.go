package api

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/delivery/compose/realtime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// The Timetable owner's deviation writes, attendance corrections and
// attendance mirror (#3424 slice S3), bound to the collaborators the owner
// may not name: the Audit Platform's protocol and correction trail
// (TimetableDeviationProtocol and
// TimetableAttendanceCorrectionTrail), the realtime hub and
// Student Presence's attendance syncer port.

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

// NewTimetableAttendanceCorrections composes the attendance correction over
// the retained rows (Participants, Instances and the optional completion
// Locks) with the Audit Platform's correction trail. trail is optional:
// without it every correction fails closed.
func NewTimetableAttendanceCorrections(rows timetableCompose.OwnerRows, trail attendanceCorrectionLedger, people timetablePorts.CorrectionPeople, logger *slog.Logger) (timetable.AttendanceCorrections, error) {
	return timetableCompose.NewAttendanceCorrections(timetablePorts.AttendanceCorrectionDependencies{
		Participants: rows.Participants,
		Instances:    rows.Instances,
		Trail:        TimetableAttendanceCorrectionTrail(trail),
		People:       people,
		Locks:        rows.Locks,
		Logger:       logger,
	})
}

// NewTimetableAttendanceMirror binds Student Presence's attendance syncer
// port to the Timetable owner's attendance mirror over the rows' Instances
// and Participants: Presence triggers the Timetable command inside its own
// tenant transaction and rolls its visit write back with a failing mirror.
// The Presence behaviour suites bind it the same way.
func NewTimetableAttendanceMirror(rows timetableCompose.OwnerRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
	return newNativeAttendanceMirror(timetableCompose.NewNativeMirrorRows(rows), logger)
}

func newNativeAttendanceMirror(rows timetableCompose.NativeMirrorRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
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
	return presenceAttendanceMirror{mirror: mirror}, nil
}

// presenceAttendanceMirror maps Student Presence's visits onto the mirror's
// own vocabulary. A missing visit is a no-op, as before the move.
type presenceAttendanceMirror struct {
	mirror timetable.AttendanceMirror
}

var _ studentpresence.AttendanceSyncer = presenceAttendanceMirror{}

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

func (m presenceAttendanceMirror) MirrorCheckInForVisit(ctx context.Context, visit *studentpresence.Visit) (*studentpresence.AttendanceSnapshot, error) {
	if visit == nil {
		return nil, nil
	}
	snapshot, err := m.mirror.MirrorCheckInForVisit(ctx, attendanceVisitOf(visit))
	return presenceSnapshotOf(snapshot), err
}

func (m presenceAttendanceMirror) MirrorCheckInAt(ctx context.Context, studentID int64, at time.Time) (*studentpresence.AttendanceSnapshot, error) {
	snapshot, err := m.mirror.MirrorCheckInAt(ctx, studentID, at)
	return presenceSnapshotOf(snapshot), err
}

func (m presenceAttendanceMirror) MirrorCheckOutForVisit(ctx context.Context, visit *studentpresence.Visit) (*studentpresence.AttendanceSnapshot, error) {
	if visit == nil {
		return nil, nil
	}
	snapshot, err := m.mirror.MirrorCheckOutForVisit(ctx, attendanceVisitOf(visit))
	return presenceSnapshotOf(snapshot), err
}

func (m presenceAttendanceMirror) MirrorVisitRevision(ctx context.Context, previous, updated *studentpresence.Visit) error {
	if previous == nil || updated == nil {
		return nil
	}
	return m.mirror.MirrorVisitRevision(ctx, attendanceVisitOf(previous), attendanceVisitOf(updated))
}

func (m presenceAttendanceMirror) MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) error {
	return m.mirror.MirrorCheckOutAt(ctx, studentID, at)
}

func (m presenceAttendanceMirror) MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error {
	return m.mirror.MirrorCheckInAtBatch(ctx, studentIDs, at)
}

func (m presenceAttendanceMirror) MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) error {
	return m.mirror.MirrorCheckOutAtBatch(ctx, studentIDs, at)
}

func (m presenceAttendanceMirror) MirrorCheckOutForVisits(ctx context.Context, visits []*studentpresence.Visit, at time.Time) error {
	mapped := make([]timetable.AttendanceVisit, 0, len(visits))
	for _, visit := range visits {
		if visit != nil {
			mapped = append(mapped, attendanceVisitOf(visit))
		}
	}
	return m.mirror.MirrorCheckOutForVisits(ctx, mapped, at)
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
