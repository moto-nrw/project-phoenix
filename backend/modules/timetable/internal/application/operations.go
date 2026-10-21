package application

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The operational day of the Timetable owner (#3424 slice S5): the
// "Heute geplant" views, the supervision rosters and the lifecycle actions
// staff take on a running block, behind timetable.OperationCapability. The
// reads use the retained repository rows, whose status carries the Student
// Presence session a block runs in. Collaborators the owner may not name
// arrive as consumer-owned ports the composition root binds: Care Plan's
// care days and effective times, the Settings Platform's policies, School
// Structure's groups and class names, Facilities' rooms, the realtime
// broadcaster, the staff attribution of the presence writes and the
// instance lifecycle that still lives in the retained planning services.

type operations struct {
	deps   ports.OperationDependencies
	caller ports.OperationCaller
}

// NewOperations composes the Timetable owner's operational day.
func NewOperations(deps ports.OperationDependencies, caller ports.OperationCaller) (timetable.OperationCapability, error) {
	if deps.Session == nil {
		return nil, errors.New("timetable operations: the request session is required")
	}
	if caller == nil || deps.Instances == nil || deps.InstanceStaff == nil || deps.Participants == nil || deps.Templates == nil ||
		deps.Sessions == nil || deps.Supervisions == nil || deps.Visits == nil || deps.Attendance == nil ||
		deps.Students == nil || deps.People == nil || deps.EducationGroups == nil || deps.Rooms == nil || deps.BlockMetadata == nil ||
		deps.Settings == nil || deps.CareDays == nil || deps.Arrivals == nil || deps.Pickups == nil ||
		deps.Lifecycle == nil || deps.NormalizeSchoolClass == nil {
		return nil, errors.New("timetable operations: required dependency is nil")
	}
	return &operations{deps: deps, caller: caller}, nil
}

func (s *operations) now() time.Time {
	if s.deps.Now != nil {
		return s.deps.Now()
	}
	return time.Now()
}

func (s *operations) today() calendar.Date {
	return calendar.DateFromTime(s.now())
}

func (s *operations) logger() *slog.Logger {
	if s.deps.Logger != nil {
		return s.deps.Logger
	}
	return slog.Default()
}

func (s *operations) loadInstance(ctx context.Context, instanceID int64) (*scheduleModels.ActivityInstance, error) {
	inst, err := findLegacyInstance(ctx, s.deps.Instances, instanceID)
	if err != nil {
		if domain.IsRecordNotFound(err) {
			return nil, timetable.ErrTimetableOperationNotFound
		}
		return nil, err
	}
	if inst == nil {
		return nil, timetable.ErrTimetableOperationNotFound
	}
	return inst, nil
}

// resolveStaffID resolves the caller's staff profile; a caller without one
// has hasStaff false.
func (s *operations) resolveStaffID(ctx context.Context, accountID int64) (int64, bool, error) {
	if accountID <= 0 {
		return 0, false, timetable.ErrTimetableOperationForbidden
	}
	person, err := s.deps.People.FindByAccountID(ctx, accountID)
	if err != nil {
		if s.caller.IsPersonNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if person == nil {
		return 0, false, nil
	}
	staff, err := s.deps.People.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		if domain.IsRecordNotFound(err) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if staff == nil {
		return 0, false, nil
	}
	return staff.ID, true, nil
}

// lifecycleWindow is the planned window of a retained block.
func lifecycleWindow(inst *scheduleModels.ActivityInstance) timetable.LifecycleWindow {
	return timetable.LifecycleWindow{
		Date: calendar.Date(inst.Date), StartTime: inst.StartTime, EndTime: inst.EndTime, IsSpontaneous: inst.IsSpontaneous,
	}
}

func retainedInstanceIDs(instances []*scheduleModels.ActivityInstance) []int64 {
	ids := make([]int64, 0, len(instances))
	for _, instance := range instances {
		ids = append(ids, instance.ID)
	}
	return ids
}

func indexInstanceStudentRows(rows []*scheduleModels.InstanceStudent) map[int64][]*scheduleModels.InstanceStudent {
	byInstance := make(map[int64][]*scheduleModels.InstanceStudent)
	for _, row := range rows {
		byInstance[row.InstanceID] = append(byInstance[row.InstanceID], row)
	}
	return byInstance
}

func staffAssigned(rows []*scheduleModels.InstanceStaff, staffID int64) bool {
	for _, row := range rows {
		if row.StaffID == staffID && !row.IsAbsent {
			return true
		}
	}
	return false
}

func findPlanned(rows []*scheduleModels.InstanceStudent, studentID int64) (*scheduleModels.InstanceStudent, bool) {
	for _, row := range rows {
		if row.StudentID == studentID {
			return row, true
		}
	}
	return nil, false
}
