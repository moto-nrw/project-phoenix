package api

import (
	"context"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// planStaffRows are the People Directory staff reads the Dienstplan and the
// substitution overview take, in the retained row shape.
type planStaffRows interface {
	FindByID(ctx context.Context, id any) (*peopleValues.Staff, error)
	ListAllWithPerson(ctx context.Context) ([]*peopleValues.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.Staff, error)
}

// planStaffDirectory serves those reads as Workforce plan staff (#2729).
// Errors pass through unchanged, so the planners keep classifying a missing
// staff member as the repository not-found.
type planStaffDirectory struct{ rows planStaffRows }

func NewPlanStaffDirectory(rows planStaffRows) planStaffDirectory {
	return planStaffDirectory{rows: rows}
}

func (d planStaffDirectory) FindByID(ctx context.Context, id any) (*workforce.PlanStaff, error) {
	row, err := d.rows.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return planStaff(row), nil
}

func (d planStaffDirectory) ListAllWithPerson(ctx context.Context) ([]*workforce.PlanStaff, error) {
	rows, err := d.rows.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*workforce.PlanStaff, 0, len(rows))
	for _, row := range rows {
		result = append(result, planStaff(row))
	}
	return result, nil
}

func (d planStaffDirectory) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*workforce.PlanStaff, error) {
	rows, err := d.rows.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]*workforce.PlanStaff, len(rows))
	for id, row := range rows {
		result[id] = planStaff(row)
	}
	return result, nil
}

// planStaff maps one staff row; a nil row stays nil.
func planStaff(row *peopleValues.Staff) *workforce.PlanStaff {
	if row == nil {
		return nil
	}
	staff := &workforce.PlanStaff{
		ID: row.ID, PersonID: row.PersonID, WorkTimeModelID: row.WorkTimeModelID,
		RotationAnchorDate: row.RotationAnchorDate, IsGuest: row.IsGuest,
	}
	if row.Person != nil {
		staff.Person = &workforce.PlanStaffPerson{FirstName: row.Person.FirstName, LastName: row.Person.LastName, AccountID: row.Person.AccountID}
	}
	return staff
}
