package owners

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type pickupPlanStudents struct{ people peopledirectory.Capability }

func (s pickupPlanStudents) FindByIDs(ctx context.Context, ids []int64, date timezone.Date) (map[int64]careplanCompose.PickupBulkStudent, error) {
	rows, err := s.people.ListStudentRecordsByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]careplanCompose.PickupBulkStudent, len(rows))
	for _, row := range rows {
		result[row.ID] = pickupPlanStudent(row, date)
	}
	return result, nil
}

func (s pickupPlanStudents) LockByID(ctx context.Context, id int64, date timezone.Date) (careplanCompose.PickupBulkStudent, error) {
	row, err := s.people.FindStudentRecordForMutation(ctx, id)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return careplanCompose.PickupBulkStudent{}, careplan.ErrBulkStudentNotFound
	}
	if err != nil {
		return careplanCompose.PickupBulkStudent{}, err
	}
	return pickupPlanStudent(row, date), nil
}

func pickupPlanStudent(row peopledirectory.StudentRecord, date timezone.Date) careplanCompose.PickupBulkStudent {
	return careplanCompose.PickupBulkStudent{ScheduleStudent: careplan.ScheduleStudent{ID: row.ID, TenantID: row.TenantID}, PersonID: row.PersonID, Eligible: row.PickupEligibleOn(date)}
}

func (s pickupPlanStudents) Name(ctx context.Context, student careplanCompose.PickupBulkStudent) (string, error) {
	person, err := s.people.FindPerson(ctx, student.PersonID)
	if err != nil {
		return "", fmt.Errorf("find person for student %d: %w", student.ID, err)
	}
	return person.FullName(), nil
}
