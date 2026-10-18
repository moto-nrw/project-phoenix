package api

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
	care     statusFlagClearer
}

// statusFlagClearer is Care Plan's student profile command the status
// directory clears flags through.
type statusFlagClearer interface {
	ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error)
}

// CarePlanStatusStudents binds the Care Plan status directory to the People
// Directory and to Care Plan's own student profile commands.
func CarePlanStatusStudents(students peopledirectory.Capability, care careplan.StudentProfileCommands) (carePlanCompose.StatusStudentDirectory, error) {
	statusFlags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return nil, errors.New("compose Care Plan: People Directory status-flag capability is required")
	}
	return statusStudentDirectory{students: students, flags: statusFlags, care: care}, nil
}

func (d statusStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]carePlanCompose.StatusStudent, error) {
	students, err := d.students.ListEnrolledStudents(ctx)
	return statusStudents(students), err
}

func (d statusStudentDirectory) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]carePlanCompose.StatusStudent, error) {
	students, err := d.flags.ListStudentsWithStatusFlag(ctx, status)
	return statusStudents(students), err
}

func (d statusStudentDirectory) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	return d.care.ClearStudentStatusFlags(ctx, ids, status)
}

func (d statusStudentDirectory) LockStudent(ctx context.Context, id int64) error {
	return d.students.LockStudent(ctx, id)
}

func statusStudents(values []peopledirectory.Student) []carePlanCompose.StatusStudent {
	result := make([]carePlanCompose.StatusStudent, 0, len(values))
	for _, value := range values {
		result = append(result, carePlanCompose.StatusStudent{
			ID: value.ID, TenantID: value.TenantID, Status: value.Status,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
		})
	}
	return result
}

type statusSlotDirectory struct {
	slots timetablePorts.InstanceStatusSlots
}

func CarePlanStatusSlots(repository timetablePorts.StatusSlotRows) carePlanCompose.StatusSlotDirectory {
	if repository == nil {
		return nil
	}
	return statusSlotDirectory{slots: timetableCompose.NewInstanceStatusSlots(repository)}
}

func (d statusSlotDirectory) ApplyStatusDay(ctx context.Context, studentID int64, date careplan.Date, statusDayID int64, substatus string) (int, error) {
	return d.slots.ApplyStatusDay(ctx, studentID, string(date), statusDayID, substatus)
}

func (d statusSlotDirectory) ReleaseStatusDay(ctx context.Context, statusDayID int64) (int, error) {
	return d.slots.ReleaseStatusDay(ctx, statusDayID)
}
