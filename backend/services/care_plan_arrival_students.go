package services

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type arrivalPlanStudents struct{ people peopledirectory.Capability }

func arrivalPlanStudent(row peopledirectory.StudentRecord, date timezone.Date) careplanCompose.ArrivalBulkStudent {
	return careplanCompose.ArrivalBulkStudent{ScheduleStudent: careplan.ScheduleStudent{ID: row.ID, TenantID: row.TenantID},
		PersonID: row.PersonID, SchoolClass: row.SchoolClass, GroupID: row.GroupID, Alumnus: row.IsAlumnus(),
		CareEnded: row.EnrolledUntil != "" && date.After(timezone.Date(row.EnrolledUntil))}
}

func arrivalPlanRows(rows []peopledirectory.StudentRecord, date timezone.Date) []careplanCompose.ArrivalBulkStudent {
	result := make([]careplanCompose.ArrivalBulkStudent, len(rows))
	for i, row := range rows {
		result[i] = arrivalPlanStudent(row, date)
	}
	return result
}

func arrivalPlanMap(rows []peopledirectory.StudentRecord, date timezone.Date) map[int64]careplanCompose.ArrivalBulkStudent {
	result := make(map[int64]careplanCompose.ArrivalBulkStudent, len(rows))
	for _, row := range rows {
		result[row.ID] = arrivalPlanStudent(row, date)
	}
	return result
}

func (s arrivalPlanStudents) ByClass(ctx context.Context, class string, date timezone.Date) ([]careplanCompose.ArrivalBulkStudent, error) {
	rows, err := s.people.ListStudentRecordsByClass(ctx, []string{class}, peopledirectory.StudentScopeEnrolled)
	return arrivalPlanRows(rows, date), err
}

func (s arrivalPlanStudents) ByGroup(ctx context.Context, id int64, date timezone.Date) ([]careplanCompose.ArrivalBulkStudent, error) {
	rows, err := s.people.ListStudentRecordsByGroup(ctx, []int64{id}, peopledirectory.StudentScopeEnrolled)
	return arrivalPlanRows(rows, date), err
}

func (s arrivalPlanStudents) ByIDs(ctx context.Context, ids []int64, date timezone.Date) (map[int64]careplanCompose.ArrivalBulkStudent, error) {
	rows, err := s.people.ListStudentRecordsByID(ctx, ids)
	return arrivalPlanMap(rows, date), err
}

func (s arrivalPlanStudents) LockByID(ctx context.Context, id int64, date timezone.Date) (careplanCompose.ArrivalBulkStudent, error) {
	row, err := s.people.FindStudentRecordForMutation(ctx, id)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return careplanCompose.ArrivalBulkStudent{}, careplan.ErrBulkStudentNotFound
	}
	if err != nil {
		return careplanCompose.ArrivalBulkStudent{}, err
	}
	return arrivalPlanStudent(row, date), nil
}

func (s arrivalPlanStudents) LockByIDs(ctx context.Context, ids []int64, date timezone.Date) (map[int64]careplanCompose.ArrivalBulkStudent, error) {
	rows, err := s.people.LockStudentRecordsByID(ctx, ids)
	return arrivalPlanMap(rows, date), err
}

func (s arrivalPlanStudents) Name(ctx context.Context, student careplanCompose.ArrivalBulkStudent) (string, error) {
	return pickupPlanStudents(s).Name(ctx, careplanCompose.PickupBulkStudent{ScheduleStudent: student.ScheduleStudent, PersonID: student.PersonID})
}

func (s arrivalPlanStudents) LockStudent(ctx context.Context, id int64) error {
	_, err := s.LockByID(ctx, id, timezone.TodayDate())
	return err
}

func (s arrivalPlanStudents) SchoolClass(ctx context.Context, id int64) (string, error) {
	row, err := s.people.FindStudentRecord(ctx, id)
	return row.SchoolClass, err
}
