package compose

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
)

// Planning carries the owner's planning capabilities the timetable HTTP
// adapter mounts beside the operational day (#3424): the template writes,
// the recurrence gate, the planner's reads, the conflict detection, the
// attendance correction of completed blocks and the deviation writes
// (slice S3). They travel as one value.
type Planning struct {
	Templates             timetable.TemplateAdministration
	RecurrenceLock        timetable.RecurrenceWriteLock
	Data                  timetable.TimetableDataCapability
	ConflictDetection     timetable.ConflictDetectionCapability
	AttendanceCorrections timetable.AttendanceCorrections
	Deviations            timetable.StaffDeviations
}

// NewLifecycleSchedulers composes the scheduler's auto start and auto end
// over the lifecycle, with the owner's start check.
func NewLifecycleSchedulers(rows OwnerRows, lifecycle timetable.InstanceLifecycle, conflicts timetable.StartConflictQuery, logger *slog.Logger) (timetable.InstanceAutoStart, timetable.InstanceAutoEnd, error) {
	autoStart, err := application.NewInstanceAutoStart(application.InstanceAutoStartDependencies{
		InstanceRepo:      rows.Instances,
		InstanceStaffRepo: rows.InstanceStaff,
		Lifecycle:         lifecycle,
		Rooms:             rows.Rooms,
		Conflicts:         conflicts,
		Logger:            logger,
	})
	if err != nil {
		return nil, nil, err
	}
	autoEnd, err := NewInstanceAutoEnd(rows.Instances.Reads, lifecycle)
	if err != nil {
		return nil, nil, err
	}
	return autoStart, autoEnd, nil
}

// NewSubstituteConflictsOverRows composes the substitute time-overlap
// advisories the instance lifecycle's staff move asks for over rows.
func NewSubstituteConflictsOverRows(rows OwnerRows) (timetable.SubstituteConflictQuery, error) {
	return application.NewSubstituteConflicts(application.SubstituteConflictDependencies{
		Instances: rows.Instances.Reads, InstanceStaff: rows.InstanceStaff,
	})
}
