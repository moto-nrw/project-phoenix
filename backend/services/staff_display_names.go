package services

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/active"
)

type staffNameRecords interface {
	FindWithPersonByIDs(context.Context, []int64) (map[int64]*users.Staff, error)
}

type staffDisplayNameQuery struct{ source staffNameRecords }

// StaffDisplayNames projects the tenant-scoped staff batch query for audit readers.
func StaffDisplayNames(source staffNameRecords) active.StaffDisplayNameQuery {
	return staffDisplayNameQuery{source: source}
}

func (q staffDisplayNameQuery) StaffDisplayNames(ctx context.Context, ids []int64) (map[int64]string, error) {
	rows, err := q.source.FindWithPersonByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	names := make(map[int64]string, len(rows))
	for id, row := range rows {
		if row != nil && row.Person != nil {
			names[id] = strings.TrimSpace(row.Person.FirstName + " " + row.Person.LastName)
		}
	}
	return names, nil
}
