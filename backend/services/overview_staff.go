package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type overviewStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type overviewStaffQuery struct{ source overviewStaffRecords }

func OverviewStaff(source overviewStaffRecords) timetracking.OverviewStaffQuery {
	return overviewStaffQuery{source: source}
}

func (q overviewStaffQuery) ListOverviewStaff(ctx context.Context) ([]timetracking.OverviewStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]timetracking.OverviewStaff, 0, len(rows))
	for _, row := range rows {
		// External caregivers (#3823) record no working time.
		if row.IsGuest {
			continue
		}
		member := timetracking.OverviewStaff{
			ID: row.ID, EmploymentType: row.EmploymentType, PersonnelNumber: row.PersonnelNumber,
			WorkTimeModelID: row.WorkTimeModelID, RotationAnchorDate: row.RotationAnchorDate,
		}
		if row.Person != nil {
			member.FirstName = row.Person.FirstName
			member.LastName = row.Person.LastName
		}
		staff = append(staff, member)
	}
	return staff, nil
}
