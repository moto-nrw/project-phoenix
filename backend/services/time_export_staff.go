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
	staff := make([]timetracking.TimeExportStaff, len(rows))
	for i, row := range rows {
		staff[i].ID = row.ID
		if row.Person != nil {
			staff[i].FirstName = row.Person.FirstName
			staff[i].LastName = row.Person.LastName
		}
	}
	return staff, nil
}
