package services

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/realtime"
)

// The Timetable owner's deviation writes, attendance corrections and
// attendance mirror (#3424 slice S3), bound to the collaborators the owner
// may not name: the Audit Platform's protocol and correction trail
// (repositories.TimetableDeviationProtocol and
// repositories.TimetableAttendanceCorrectionTrail), the realtime hub and
// Student Presence's attendance syncer port.

// timetableDeviationInputs compose the deviation writes over the retained
// rows (Instances, InstanceStaff and the Audit Platform's DeviationEvents).
// Lifecycle is the instance lifecycle the cancel branch and the
// "deliberately unstaffed" acknowledgement hand over to.
type timetableDeviationInputs struct {
	Rows         repositories.TimetableOwnerRows
	Staff        usersModels.StaffRepository
	Supervisions studentpresence.SupervisionRecords
	Lifecycle    timetableCompose.DeviationLifecycle
	Broadcaster  realtime.Broadcaster
	DB           *bun.DB
	Logger       *slog.Logger
	Now          func() time.Time
}

func newTimetableStaffDeviations(in timetableDeviationInputs) (timetable.StaffDeviations, error) {
	if in.Lifecycle == nil || in.Rows.DeviationEvents == nil || in.Broadcaster == nil {
		return nil, errors.New("timetable staff deviations: lifecycle, protocol and broadcaster are required")
	}
	return timetableCompose.NewStaffDeviations(timetableCompose.StaffDeviationDependencies{
		Instances:       in.Rows.Instances,
		InstanceStaff:   in.Rows.InstanceStaff,
		Staff:           in.Staff,
		Supervisions:    in.Supervisions,
		Lifecycle:       in.Lifecycle,
		Protocol:        repositories.TimetableDeviationProtocol(in.Rows.DeviationEvents),
		ActivityUpdates: timetableActivityUpdates{broadcaster: in.Broadcaster},
		DB:              in.DB,
		Logger:          in.Logger,
		Now:             in.Now,
	})
}

// newTimetableSubstituteConflicts composes the substitute time-overlap
// advisories the instance lifecycle's staff move asks for.
func newTimetableSubstituteConflicts(rows repositories.TimetableOwnerRows) (timetable.SubstituteConflictQuery, error) {
	return timetableCompose.NewSubstituteConflicts(timetableCompose.SubstituteConflictDependencies{
		Instances: rows.Instances, InstanceStaff: rows.InstanceStaff,
	})
}

// timetableCorrectionInputs compose the attendance correction over the
// retained rows (Participants, Instances and the optional completion Locks).
// Trail is optional: without it every correction fails closed.
type timetableCorrectionInputs struct {
	Rows   repositories.TimetableOwnerRows
	Trail  auditModels.AttendanceCorrectionRepository
	People usersModels.PersonRepository
	Logger *slog.Logger
}

func newTimetableAttendanceCorrections(in timetableCorrectionInputs) (timetable.AttendanceCorrections, error) {
	return timetableCompose.NewAttendanceCorrections(timetableCompose.AttendanceCorrectionDependencies{
		Participants: in.Rows.Participants,
		Instances:    in.Rows.Instances,
		Trail:        repositories.TimetableAttendanceCorrectionTrail(in.Trail),
		People:       in.People,
		Locks:        in.Rows.AttendanceLocks(),
		Logger:       in.Logger,
	})
}

// NewTimetableAttendanceMirror binds Student Presence's attendance syncer
// port to the Timetable owner's attendance mirror over the rows' Instances
// and Participants: Presence triggers the Timetable command inside its own
// tenant transaction and rolls its visit write back with a failing mirror.
// The Presence behaviour suites bind it the same way.
func NewTimetableAttendanceMirror(rows repositories.TimetableOwnerRows, logger *slog.Logger) (studentpresence.AttendanceSyncer, error) {
	mirror, err := timetableCompose.NewAttendanceMirror(timetableCompose.AttendanceMirrorDependencies{
		Instances: rows.Instances, Participants: rows.Participants, Logger: logger,
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
