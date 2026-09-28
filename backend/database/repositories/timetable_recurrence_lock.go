package repositories

import (
	"errors"

	"github.com/uptrace/bun"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// NewTimetableRecurrenceLock composes the Timetable owner's tenant recurrence
// gate (#3424 slice S2) over the caller's transaction and School Structure's
// grade-transition key. Every recurrence writer of the composition roots
// takes the gate through it, so the lock order stays the same everywhere.
func NewTimetableRecurrenceLock(db *bun.DB) (timetable.RecurrenceWriteLock, error) {
	if db == nil {
		return nil, errors.New("timetable recurrence lock: database is required")
	}
	return timetableCompose.NewRecurrenceWriteLock(timetableCompose.RecurrenceLockDependencies{
		DB:                     db,
		GradeTransitionLockKey: educationModels.TenantTransitionsLockKey,
	})
}

// TimetableTemplateRows are the retained repository rows the Timetable
// owner's template writes and materialization run over (#3424 slice S2). The
// composition root fills them from its repository set.
type TimetableTemplateRows struct {
	Groups          activitiesModels.GroupRepository
	Categories      activitiesModels.CategoryRepository
	Schedules       activitiesModels.ScheduleRepository
	Enrollments     activitiesModels.StudentEnrollmentRepository
	Supervisors     activitiesModels.SupervisorPlannedRepository
	Instances       scheduleModels.ActivityInstanceRepository
	InstanceStaff   scheduleModels.InstanceStaffRepository
	Participants    scheduleModels.InstanceStudentRepository
	Timeframes      scheduleModels.TimeframeRepository
	CalendarPeriods scheduleModels.CalendarPeriodRepository
	Exceptions      scheduleModels.ActivityExceptionRepository
	EducationGroups educationModels.GroupRepository
}
