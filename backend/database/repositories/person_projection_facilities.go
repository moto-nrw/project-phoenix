package repositories

import (
	"context"
	"slices"
	"strings"

	filestoreModels "github.com/moto-nrw/project-phoenix/models/filestore"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// personFolderRepository keeps the audience picker on accounts that are
// backed by a person of the school and orders them by name.
type personFolderRepository struct {
	filestoreModels.FolderRepository
	persons peopledirectory.Query
}

func (r personFolderRepository) ListAudienceAccounts(ctx context.Context) ([]*filestoreModels.AudienceAccount, error) {
	rows, err := r.FolderRepository.ListAudienceAccounts(ctx)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.AccountID)
	}
	persons, err := personsByAccount(ctx, r.persons, ids)
	if err != nil {
		return nil, err
	}
	result := make([]*filestoreModels.AudienceAccount, 0, len(rows))
	for _, row := range rows {
		person, found := persons[row.AccountID]
		if !found {
			continue
		}
		row.FirstName = person.FirstName
		row.LastName = person.LastName
		result = append(result, row)
	}
	slices.SortStableFunc(result, func(left, right *filestoreModels.AudienceAccount) int {
		if order := compareStrings(strings.ToLower(left.LastName), strings.ToLower(right.LastName)); order != 0 {
			return order
		}
		return compareStrings(strings.ToLower(left.FirstName), strings.ToLower(right.FirstName))
	})
	return result, nil
}
