package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/workforce/legacy/timetracking"
)

type timeExportStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type timeExportStaffQuery struct{ source timeExportStaffRecords }

func TimeExportStaff(source timeExportStaffRecords) timetracking.TimeExportStaffQuery {
	return timeExportStaffQuery{source: source}
}

func (q timeExportStaffQuery) ListExportStaff(ctx context.Context) ([]timetracking.TimeExportStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]timetracking.TimeExportStaff, 0, len(rows))
	for _, row := range rows {
		// External caregivers (#3823) record no working time.
		if row.IsGuest {
			continue
		}
		member := timetracking.TimeExportStaff{ID: row.ID}
		if row.Person != nil {
			member.FirstName = row.Person.FirstName
			member.LastName = row.Person.LastName
		}
		staff = append(staff, member)
	}
	return staff, nil
}
