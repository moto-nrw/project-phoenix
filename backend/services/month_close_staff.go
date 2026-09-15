package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type monthCloseStaffRecords interface {
	ListAllWithPerson(context.Context) ([]*users.Staff, error)
}

type monthCloseStaffQuery struct{ source monthCloseStaffRecords }

// MonthCloseStaff preserves the staff selection used by school-wide month close.
func MonthCloseStaff(source monthCloseStaffRecords) active.MonthCloseStaffQuery {
	return monthCloseStaffQuery{source: source}
}

func (q monthCloseStaffQuery) ListStaffIDs(ctx context.Context) ([]int64, error) {
	rows, err := q.source.ListAllWithPerson(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	return ids, nil
}
