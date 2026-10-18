package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// PersonRenames serves the profile rename of the account lifecycle
// (peopledirectory.AccountLifecycleCommand). It locks the person and writes
// only its names, trimmed but not capped, as the profile flow always stored
// them.
type PersonRenames struct{ persons *Service }

func (p PersonRenames) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	s := p.persons
	return MapError(s.runWrite(ctx, "rename_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		person, found, lockStats, err := s.store.FindByID(txCtx, personID, "UPDATE")
		stats.Add(lockStats)
		if err != nil {
			return err
		}
		if !found {
			return domain.ErrNotFound
		}
		first, last := person.FirstName, person.LastName
		if firstName != nil {
			first = *firstName
		}
		if lastName != nil {
			last = *lastName
		}
		writeStats, err := s.store.SetNames(txCtx, personID, strings.TrimSpace(first), strings.TrimSpace(last))
		stats.Add(writeStats)
		return err
	}))
}
