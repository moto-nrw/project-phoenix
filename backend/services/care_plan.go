package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
	care     careplan.StudentProfileCommands
}

func CarePlanStatusStudents(db *bun.DB, students peopledirectory.Capability) (carePlanCompose.StatusStudentDirectory, error) {
	statusFlags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		return nil, errors.New("compose Care Plan: People Directory status-flag capability is required")
	}
	care, err := carePlanCompose.NewStudentProfiles(db, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
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
	slots timetableCompose.InstanceStatusSlots
}

func CarePlanStatusSlots(repository timetableCompose.StatusSlotRows) carePlanCompose.StatusSlotDirectory {
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

type pickupExceptionDirectory struct {
	query carePlanCompose.ExceptionQueries
}

func (d pickupExceptionDirectory) FindPickupException(ctx context.Context, id int64) (*timetableCompose.PickupExceptionProjection, error) {
	value, err := d.query.FindPickupException(ctx, id, false)
	if errors.Is(err, careplan.ErrStudentScheduleNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto}, nil
}

func (d pickupExceptionDirectory) ListPickupExceptions(ctx context.Context, filter timetableCompose.PickupExceptionFilter) ([]timetableCompose.PickupExceptionProjection, error) {
	ownerFilter := careplan.StudentScheduleFilter{IDs: filter.IDs, StudentIDs: filter.StudentIDs}
	if filter.Date != "" {
		ownerFilter.Date = careplan.Date(filter.Date)
	}
	if filter.From != "" {
		ownerFilter.From = careplan.Date(filter.From)
	}
	values, err := d.query.ListPickupExceptions(ctx, ownerFilter)
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.PickupExceptionProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.PickupExceptionProjection{ID: value.ID, StudentID: value.StudentID, ExceptionDate: value.ExceptionDate.String(), ExcusedFrom: value.ExcusedFrom, ExcusedAuto: value.ExcusedAuto})
	}
	return result, nil
}

func (d pickupExceptionDirectory) FindStudentStatusDay(ctx context.Context, id int64, activeOnly bool) (*timetableCompose.StudentStatusDayProjection, error) {
	value, err := d.query.FindStudentStatusDay(ctx, id, activeOnly)
	if errors.Is(err, careplan.ErrStudentStatusDayNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status}, nil
}

func (d pickupExceptionDirectory) ListStudentStatusDays(ctx context.Context, filter timetableCompose.StudentStatusDayFilter) ([]timetableCompose.StudentStatusDayProjection, error) {
	values, err := d.query.ListStudentStatusDays(ctx, careplan.StudentStatusDayFilter{
		IDs: filter.IDs, StudentIDs: filter.StudentIDs, Date: careplan.Date(filter.Date),
		From: careplan.Date(filter.From), ActiveOnly: filter.ActiveOnly, LatestOnly: filter.LatestOnly,
	})
	if err != nil {
		return nil, err
	}
	result := make([]timetableCompose.StudentStatusDayProjection, 0, len(values))
	for _, value := range values {
		result = append(result, timetableCompose.StudentStatusDayProjection{ID: value.ID, StudentID: value.StudentID, Date: value.Date.String(), Status: value.Status})
	}
	return result, nil
}
