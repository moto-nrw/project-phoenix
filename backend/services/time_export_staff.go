package services

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type timeExportStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type timeExportStaffQuery struct{ source timeExportStaffRecords }

func TimeExportStaff(source timeExportStaffRecords) workforceCompose.TimeExportStaffQuery {
	return timeExportStaffQuery{source: source}
}

func (q timeExportStaffQuery) ListExportStaff(ctx context.Context) ([]workforceCompose.TimeExportStaff, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	staff := make([]workforceCompose.TimeExportStaff, 0, len(rows))
	for _, row := range rows {
		// External caregivers (#3823) record no working time.
		if row.IsGuest {
			continue
		}
		member := workforceCompose.TimeExportStaff{ID: row.ID}
		if row.Person != nil {
			member.FirstName = row.Person.FirstName
			member.LastName = row.Person.LastName
		}
		staff = append(staff, member)
	}
	return staff, nil
}
