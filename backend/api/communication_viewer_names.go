package api

import (
	"context"

	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// communicationViewerNames binds Communication's viewer-name port to People
// Directory's person query: an announcement viewer's name is the full name of
// the person behind the account.
type communicationViewerNames struct{ query peopledirectory.Query }

func (v communicationViewerNames) NamesByAccount(ctx context.Context, accountIDs []int64) (map[int64]string, communicationCompose.OperationStats, error) {
	people, err := v.query.ListPersonsByAccount(ctx, accountIDs)
	if err != nil {
		return nil, communicationCompose.OperationStats{Queries: 1}, err
	}
	result := make(map[int64]string, len(people))
	for _, person := range people {
		if person.AccountID == nil {
			continue
		}
		name := person.FullName()
		if name != " " {
			result[*person.AccountID] = name
		}
	}
	return result, communicationCompose.OperationStats{Queries: 1, Rows: int64(len(people))}, nil
}
