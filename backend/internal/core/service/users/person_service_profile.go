package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/uptrace/bun"
)

// GetFullProfile retrieves a person with all related entities
func (s *personService) GetFullProfile(ctx context.Context, personID int64) (*userModels.Person, error) {
	var result *userModels.Person

	// Use transaction to ensure consistent data across all fetches
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Get transactional service
		txService := s.WithTx(tx).(PersonService)

		// Get the basic person record
		person, err := txService.Get(ctx, personID)
		if err != nil {
			return err
		}

		// Fetch related account if AccountID is set
		if person.AccountID != nil {
			account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
			if err != nil {
				return &UsersError{Op: "get full profile - fetch account", Err: err}
			}
			person.Account = account
		}

		// Fetch related RFID card if TagID is set
		if person.TagID != nil {
			card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
			if err != nil {
				return &UsersError{Op: "get full profile - fetch RFID card", Err: err}
			}
			person.RFIDCard = card
		}

		// Save the result for returning after transaction completes
		result = person
		return nil
	})

	if err != nil {
		return nil, &UsersError{Op: "get full profile", Err: err}
	}

	return result, nil
}

// FindByGuardianID finds all persons with a guardian relationship to the specified account
func (s *personService) FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*userModels.Person, error) {
	// Get all person-guardian relationships for this guardian
	// Changed from FindByGuardianAccountID to FindByGuardianID to match the repository interface
	relationships, err := s.personGuardianRepo.FindByGuardianID(ctx, guardianAccountID)
	if err != nil {
		return nil, &UsersError{Op: "find by guardian ID", Err: err}
	}

	// Extract person IDs from relationships
	personIDs := make([]interface{}, 0, len(relationships))
	for _, rel := range relationships {
		personIDs = append(personIDs, rel.PersonID)
	}

	// If no person IDs found, return empty slice
	if len(personIDs) == 0 {
		return []*userModels.Person{}, nil
	}

	// Create a filter to get persons by IDs
	options := base.NewQueryOptions()
	filter := base.NewFilter().In("id", personIDs...)
	options.Filter = filter

	persons, err := s.List(ctx, options)
	if err != nil {
		return nil, &UsersError{Op: "find by guardian ID", Err: err}
	}
	return persons, nil
}
