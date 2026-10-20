package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// PersonWrites serves the person writes of the profile rename
// (peopledirectory.AccountLifecycleCommand), of the staff record and
// Stammdaten decision (peopledirectory.PersonDetailsCommand) and of an
// operator's person deletion (peopledirectory.PersonRemovalCommand). The
// renames lock the person, deleted or not, and write only the columns they
// name, trimmed but not capped, as those flows always stored them.
type PersonWrites struct{ persons *Service }

func (p PersonWrites) RenamePerson(ctx context.Context, personID int64, firstName, lastName *string) error {
	s := p.persons
	return MapError(s.runWrite(ctx, "rename_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		person, err := p.lock(txCtx, personID, stats)
		if err != nil {
			return err
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

func (p PersonWrites) SetPersonDetails(ctx context.Context, personID int64, details peopledirectory.PersonDetails) error {
	s := p.persons
	return MapError(s.runWrite(ctx, "set_person_details", func(txCtx context.Context, stats *domain.OperationStats) error {
		if _, err := p.lock(txCtx, personID, stats); err != nil {
			return err
		}
		writeStats, err := s.store.SetDetails(txCtx, personID, details.FirstName, details.LastName, details.Birthday)
		stats.Add(writeStats)
		return err
	}))
}

func (p PersonWrites) AnonymizeAndSoftDeletePerson(ctx context.Context, personID int64) error {
	s := p.persons
	return MapError(s.runWrite(ctx, "anonymize_and_soft_delete_person", func(txCtx context.Context, stats *domain.OperationStats) error {
		writeStats, err := s.store.AnonymizeAndSoftDelete(txCtx, personID)
		stats.Add(writeStats)
		return err
	}))
}

// lock holds the person's row for the write.
func (p PersonWrites) lock(ctx context.Context, personID int64, stats *domain.OperationStats) (peopledirectory.Person, error) {
	person, found, lockStats, err := p.persons.store.FindByID(ctx, personID, "UPDATE")
	stats.Add(lockStats)
	if err != nil {
		return peopledirectory.Person{}, err
	}
	if !found {
		return peopledirectory.Person{}, peopledirectory.ErrPersonNotFound
	}
	return person, nil
}
