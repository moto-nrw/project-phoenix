package services

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
)

type monthCloseStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type monthCloseStaffQuery struct{ source monthCloseStaffRecords }

// MonthCloseStaff preserves the staff selection used by school-wide month close.
func MonthCloseStaff(source monthCloseStaffRecords) workforceCompose.MonthCloseStaffQuery {
	return monthCloseStaffQuery{source: source}
}

func (q monthCloseStaffQuery) ListStaffIDs(ctx context.Context) ([]int64, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		// External caregivers (#3823) record no working time to close.
		if !row.IsGuest {
			ids = append(ids, row.ID)
		}
	}
	return ids, nil
}
