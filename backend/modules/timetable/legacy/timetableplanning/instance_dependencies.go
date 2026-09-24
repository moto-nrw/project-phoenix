package timetableplanning

import (
	"context"
	"errors"
	"log/slog"
	"time"

	activitiesModel "github.com/moto-nrw/project-phoenix/models/activities"
	auditModel "github.com/moto-nrw/project-phoenix/models/audit"
	facilitiesModel "github.com/moto-nrw/project-phoenix/models/facilities"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	announcement "github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/uptrace/bun"
)

// InstanceServiceDependencies aggregates wiring. All repo fields are required;
// Broadcaster is optional (nil → no SSE).
type InstanceServiceDependencies struct {
	InstanceRepo       scheduleModel.ActivityInstanceRepository
	IdempotencyRepo    scheduleModel.InstanceIdempotencyRepository
	InstanceStaffRepo  scheduleModel.InstanceStaffRepository
	InstanceStudents   scheduleModel.InstanceStudentRepository
	ExceptionRepo      scheduleModel.ActivityExceptionRepository
	ActiveGroupRepo    studentpresence.SessionRecords
	SupervisorRepo     studentpresence.SupervisionRecords
	Presence           InstancePresence
	RoomRepo           facilitiesModel.RoomRepository
	ActivityGroupRepo  activitiesModel.GroupRepository
	StaffRepo          usersModel.StaffRepository
	StudentRepo        usersModel.StudentRepository
	CalendarPeriodRepo scheduleModel.CalendarPeriodRepository
	ActiveService      ActiveSessionEnder
	Materialization    MaterializationService
	// CareDayService decides which still-expected children may be stamped
	// absent when an instance ends (#1747) — required.
	CareDayService InstanceCareDays
	// DeviationEventRepo appends the Änderungsprotokoll (#1886) — required.
	DeviationEventRepo auditModel.DeviationEventRepository
	Broadcaster        realtime.Broadcaster
	DB                 *bun.DB
	Logger             *slog.Logger
	Settings           LifecycleSettings
	RecoveryRepo       scheduleModel.ActivityRecoveryRepository
	Now                func() time.Time
	EnforceTimePolicy  bool
	// GuardianNotices publishes the cancellation notice to families (#2601).
	// Optional: nil means a cancellation can never carry a notice. The
	// composition root passes a late-bound publisher because the announcement
	// service is built after this one.
	GuardianNotices announcement.CareCancellationPublisher
	// StartConflicts is the Timetable owner's start check (#2139, #3550) —
	// required for Start.
	StartConflicts timetable.StartConflictQuery
}

// detectStartConflicts asks the owner's start check for the block and keeps
// the retained error classification: a failed read aborts the transition as
// a ScheduleError.
func detectStartConflicts(ctx context.Context, query timetable.StartConflictQuery, instance *scheduleModel.ActivityInstance) ([]timetable.InstanceConflictWarning, error) {
	if query == nil {
		return nil, &ScheduleError{Op: "detect start conflicts", Err: errors.New("start conflict detection is not configured")}
	}
	warnings, err := query.DetectStartConflicts(ctx, timetable.StartConflictSubject{InstanceID: instance.ID, RoomID: instance.RoomID})
	if err != nil {
		return nil, &ScheduleError{Op: "detect start conflicts", Err: err}
	}
	return warnings, nil
}

// isProjectedUnderstaffed applies the owner's staffing rule to a deviation's
// projected presence: the planned positions are the block's non-substitute
// rows, whatever the deviation does to their presence.
func isProjectedUnderstaffed(rows []*scheduleModel.InstanceStaff, projectedPresent int) bool {
	planned := 0
	for _, row := range rows {
		if !row.IsSubstitute {
			planned++
		}
	}
	return timetable.IsUnderstaffedCounts(projectedPresent, planned)
}

// staffingRows maps retained staff rows onto the input of the owner's
// staffing rule; nil rows are ignored.
func staffingRows(rows []*scheduleModel.InstanceStaff) []timetable.InstanceStaff {
	out := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			out = append(out, timetable.InstanceStaff{IsAbsent: row.IsAbsent, IsSubstitute: row.IsSubstitute})
		}
	}
	return out
}
