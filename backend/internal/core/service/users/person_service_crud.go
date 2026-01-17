package users

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	authService "github.com/moto-nrw/project-phoenix/internal/core/service/auth"
)

// Get retrieves a person by their ID
func (s *personService) Get(ctx context.Context, id interface{}) (*userModels.Person, error) {
	// Try to use FindWithAccount if repository supports it
	if repo, ok := s.personRepo.(interface {
		FindWithAccount(context.Context, int64) (*userModels.Person, error)
	}); ok {
		// Convert id to int64
		var personID int64
		switch v := id.(type) {
		case int:
			personID = int64(v)
		case int64:
			personID = v
		default:
			return nil, &UsersError{Op: opGetPerson, Err: fmt.Errorf("invalid ID type")}
		}

		person, err := repo.FindWithAccount(ctx, personID)
		if err != nil {
			return nil, &UsersError{Op: opGetPerson, Err: err}
		}
		if person == nil {
			return nil, &UsersError{Op: opGetPerson, Err: ErrPersonNotFound}
		}
		return person, nil
	}

	// Fallback to regular FindByID
	person, err := s.personRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &UsersError{Op: opGetPerson, Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: opGetPerson, Err: ErrPersonNotFound}
	}
	return person, nil
}

// GetByIDs retrieves multiple persons by their IDs in a single query
func (s *personService) GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Person, error) {
	if len(ids) == 0 {
		return make(map[int64]*userModels.Person), nil
	}

	persons, err := s.personRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &UsersError{Op: "get persons by IDs", Err: err}
	}

	return persons, nil
}

// Create creates a new person
func (s *personService) Create(ctx context.Context, person *userModels.Person) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return &UsersError{Op: opCreatePerson, Err: err}
	}

	// Note: Removed the requirement for TagID or AccountID
	// Students can be created without either identifier

	// Check if the account exists if AccountID is set
	if person.AccountID != nil {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if account == nil {
			return &UsersError{Op: opCreatePerson, Err: authService.ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if card == nil {
			return &UsersError{Op: opCreatePerson, Err: ErrRFIDCardNotFound}
		}
	}

	if err := s.personRepo.Create(ctx, person); err != nil {
		return &UsersError{Op: opCreatePerson, Err: err}
	}

	return nil
}

// Update updates an existing person
func (s *personService) Update(ctx context.Context, person *userModels.Person) error {
	if err := person.Validate(); err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}

	existingPerson, err := s.personRepo.FindByID(ctx, person.ID)
	if err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}
	if existingPerson == nil {
		return &UsersError{Op: opUpdatePerson, Err: ErrPersonNotFound}
	}

	if err := s.validateAccountIfChanged(ctx, person, existingPerson); err != nil {
		return err
	}

	if err := s.validateRFIDCardIfChanged(ctx, person, existingPerson); err != nil {
		return err
	}

	if err := s.personRepo.Update(ctx, person); err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}

	return nil
}

// validateAccountIfChanged validates account exists if AccountID is being changed
func (s *personService) validateAccountIfChanged(ctx context.Context, person, existingPerson *userModels.Person) error {
	if person.AccountID == nil {
		return nil
	}

	if existingPerson.AccountID != nil && *existingPerson.AccountID == *person.AccountID {
		return nil
	}

	account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
	if err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}
	if account == nil {
		return &UsersError{Op: opUpdatePerson, Err: authService.ErrAccountNotFound}
	}

	return nil
}

// validateRFIDCardIfChanged validates RFID card exists if TagID is being changed
func (s *personService) validateRFIDCardIfChanged(ctx context.Context, person, existingPerson *userModels.Person) error {
	if person.TagID == nil {
		return nil
	}

	if existingPerson.TagID != nil && *existingPerson.TagID == *person.TagID {
		return nil
	}

	card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
	if err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}
	if card == nil {
		return &UsersError{Op: opUpdatePerson, Err: ErrRFIDCardNotFound}
	}

	return nil
}

// Delete removes a person
func (s *personService) Delete(ctx context.Context, id interface{}) error {
	// Verify the person exists
	person, err := s.personRepo.FindByID(ctx, id)
	if err != nil {
		return &UsersError{Op: opDeletePerson, Err: err}
	}
	if person == nil {
		return &UsersError{Op: opDeletePerson, Err: ErrPersonNotFound}
	}

	if err := s.personRepo.Delete(ctx, id); err != nil {
		return &UsersError{Op: opDeletePerson, Err: err}
	}
	return nil
}

// List retrieves persons matching the provided query options (see #557 for refactoring)
func (s *personService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	// Convert QueryOptions to map[string]interface{} for repository
	filters := make(map[string]interface{})
	if options != nil && options.Filter != nil {
		_ = options.Filter // Filter conversion not yet implemented
	}

	persons, err := s.personRepo.List(ctx, filters)
	if err != nil {
		return nil, &UsersError{Op: "list persons", Err: err}
	}
	return persons, nil
}

// FindByTagID finds a person by their RFID tag ID
func (s *personService) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	person, err := s.personRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return nil, &UsersError{Op: "find person by tag ID", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: "find person by tag ID", Err: ErrPersonNotFound}
	}
	return person, nil
}

// FindByAccountID finds a person by their account ID
func (s *personService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	person, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, &UsersError{Op: "find person by account ID", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: "find person by account ID", Err: ErrPersonNotFound}
	}
	return person, nil
}

// FindByName finds persons matching the provided name
func (s *personService) FindByName(ctx context.Context, firstName, lastName string) ([]*userModels.Person, error) {
	options := base.NewQueryOptions()
	filter := base.NewFilter()

	if firstName != "" {
		filter.ILike("first_name", firstName+"%")
	}

	if lastName != "" {
		filter.ILike("last_name", lastName+"%")
	}

	options.Filter = filter

	persons, err := s.List(ctx, options)
	if err != nil {
		return nil, &UsersError{Op: "find persons by name", Err: err}
	}
	return persons, nil
}
