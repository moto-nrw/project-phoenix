package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type overviewStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type overviewStaffQuery struct{ source overviewStaffRecords }

func OverviewStaff(source overviewStaffRecords) active.OverviewStaffQuery {
	return overviewStaffQuery{source: source}
}

func (q overviewStaffQuery) ListOverviewStaff(ctx context.Context) ([]active.OverviewStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]active.OverviewStaff, len(rows))
	for i, row := range rows {
		staff[i] = active.OverviewStaff{
			ID: row.ID, EmploymentType: row.EmploymentType, PersonnelNumber: row.PersonnelNumber,
			WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: row.RotationAnchorDate,
		}
		if row.Person != nil {
			staff[i].FirstName = row.Person.FirstName
			staff[i].LastName = row.Person.LastName
		}
	}
	return staff, nil
}
