package compose

import (
	"context"
	"fmt"

	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// OwnerRows are the retained repository rows the Timetable owner's
// operational day, planner reads and ended-session completion run over
// (#3551). Their status carries the Student Presence session state the
// owner's own rows do not. The composition root fills them from its
// repository set and hands them to the owner's ports. The Facilities room
// names, School Structure group names and the Audit Platform's
// Änderungsprotokoll the owner may not name itself arrive as the ports the
// root binds over those owners.
type OwnerRows struct {
	Instances         scheduleModels.ActivityInstanceRepository
	InstanceStaff     scheduleModels.InstanceStaffRepository
	Participants      scheduleModels.InstanceStudentRepository
	Templates         activitiesModels.GroupRepository
	Categories        activitiesModels.CategoryRepository
	Students          RosterStudents
	EducationGroups   ports.EducationGroupNames
	Rooms             OwnerRooms
	PickupExceptions  scheduleModels.StudentPickupExceptionRepository
	ArrivalExceptions scheduleModels.StudentArrivalExceptionRepository
	// DeviationEvents reads and Deviations appends the Änderungsprotokoll.
	DeviationEvents ports.DeviationEventReader
	Deviations      ports.DeviationProtocol
	// Locks serializes attendance writes with a completion; optional.
	Locks scheduleModels.ActivityRecoveryRepository
}

// OwnerRooms names and locks the Facilities rooms for the operational day
// and the instance lifecycle.
type OwnerRooms interface {
	ports.RoomNames
	ports.LifecycleRooms
}

// OperationTemplates is the template read of the operational day, which the
// retained activity group repository serves beside its model interface.
func (r OwnerRows) OperationTemplates() (ports.OperationTemplates, error) {
	templates, ok := r.Templates.(ports.OperationTemplates)
	if !ok {
		return nil, fmt.Errorf("timetable operations: %T cannot read template targets", r.Templates)
	}
	return templates, nil
}

// DataTemplates is the Vorlagen read of the planner.
func (r OwnerRows) DataTemplates() (ports.DataTemplates, error) {
	templates, ok := r.Templates.(ports.DataTemplates)
	if !ok {
		return nil, fmt.Errorf("timetable data: %T cannot read the template list", r.Templates)
	}
	return templates, nil
}

// EndedSessionInstances is the session listing of the ended-session
// completion.
func (r OwnerRows) EndedSessionInstances() (ports.EndedSessionInstances, error) {
	instances, ok := r.Instances.(ports.EndedSessionInstances)
	if !ok {
		return nil, fmt.Errorf("ended session completion: %T cannot list instances by session", r.Instances)
	}
	return instances, nil
}

// SickCascadeRows are the Betreuungsplan rows the #1843 sick cascade reads,
// with the day-wide staffing lock (#1840) the caller supplies.
func (r OwnerRows) SickCascadeRows(dayLock func(context.Context, calendar.Date) error) SickCascadeTimetableRows {
	return SickCascadeTimetableRows{
		instances: application.NewTimetableInstanceReads(r.Instances), staff: application.NewTimetableInstanceStaffReads(r.InstanceStaff), dayLock: dayLock,
	}
}

// SickCascadeTimetableRows serves the shift-plan-sync workflow's timetable
// port from the retained rows, in the Timetable owner's public vocabulary.
type SickCascadeTimetableRows struct {
	instances application.TimetableInstanceReads
	staff     application.TimetableInstanceStaffReads
	dayLock   func(context.Context, calendar.Date) error
}

// AcquireSubstituteDayLock serializes the day-wide staffing mutations of the
// tenant within the caller's transaction.
func (r SickCascadeTimetableRows) AcquireSubstituteDayLock(ctx context.Context, date calendar.Date) error {
	return r.dayLock(ctx, date)
}

func (r SickCascadeTimetableRows) GetInstanceStaffByStaffAndDate(ctx context.Context, staffID int64, date calendar.Date) ([]*timetable.InstanceStaff, error) {
	return r.staff.FindByStaffAndDate(ctx, staffID, date)
}

func (r SickCascadeTimetableRows) GetActivityInstancesByID(ctx context.Context, ids []int64) (map[int64]*timetable.ScheduledInstance, error) {
	return r.instances.GetActivityInstancesByID(ctx, ids)
}

func (r SickCascadeTimetableRows) GetInstanceStaff(ctx context.Context, instanceID int64) ([]*timetable.InstanceStaff, error) {
	return r.staff.FindByInstanceID(ctx, instanceID)
}

// TemplateRows are the retained repository rows the Timetable owner's
// template writes and materialization run over (#3424 slice S2). The
// composition root fills them from its repository set.
type TemplateRows struct {
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
	EducationGroups ports.EducationGroupDirectory
}
