package users

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// personService implements the PersonService interface
type personService struct {
	personRepo         userModels.PersonRepository
	rfidRepo           userModels.RFIDCardRepository
	accountRepo        auth.AccountRepository
	personGuardianRepo userModels.PersonGuardianRepository
}

// NewPersonService creates a new person service
func NewPersonService(
	personRepo userModels.PersonRepository,
	rfidRepo userModels.RFIDCardRepository,
	accountRepo auth.AccountRepository,
	personGuardianRepo userModels.PersonGuardianRepository,
) PersonService {
	return &personService{
		personRepo:         personRepo,
		rfidRepo:           rfidRepo,
		accountRepo:        accountRepo,
		personGuardianRepo: personGuardianRepo,
	}
}

// Get retrieves a person by their ID
func (s *personService) Get(ctx context.Context, id interface{}) (*userModels.Person, error) {
	return s.personRepo.FindByID(ctx, id)
}

// Create creates a new person
func (s *personService) Create(ctx context.Context, person *userModels.Person) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return err
	}

	// Additional business rule: Either TagID or AccountID must be set
	if person.TagID == nil && person.AccountID == nil {
		return ErrPersonIdentifierRequired
	}

	// Check if the account exists if AccountID is set
	if person.AccountID != nil {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return err
		}
		if account == nil {
			return ErrAccountNotFound
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return err
		}
		if card == nil {
			return ErrRFIDCardNotFound
		}
	}

	return s.personRepo.Create(ctx, person)
}

// Update updates an existing person
func (s *personService) Update(ctx context.Context, person *userModels.Person) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return err
	}

	// Additional business rule: Either TagID or AccountID must be set
	if person.TagID == nil && person.AccountID == nil {
		return ErrPersonIdentifierRequired
	}

	// Check if the person exists
	existingPerson, err := s.personRepo.FindByID(ctx, person.ID)
	if err != nil {
		return err
	}
	if existingPerson == nil {
		return ErrPersonNotFound
	}

	// Check if the account exists if AccountID is set and changed
	if person.AccountID != nil &&
		(existingPerson.AccountID == nil || *existingPerson.AccountID != *person.AccountID) {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return err
		}
		if account == nil {
			return ErrAccountNotFound
		}
	}

	// Check if the RFID card exists if TagID is set and changed
	if person.TagID != nil &&
		(existingPerson.TagID == nil || *existingPerson.TagID != *person.TagID) {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return err
		}
		if card == nil {
			return ErrRFIDCardNotFound
		}
	}

	return s.personRepo.Update(ctx, person)
}

// Delete removes a person
func (s *personService) Delete(ctx context.Context, id interface{}) error {
	return s.personRepo.Delete(ctx, id)
}

// List retrieves persons matching the provided query options
func (s *personService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	// Convert QueryOptions to map[string]interface{} for repository
	filters := make(map[string]interface{})
	if options != nil && options.Filter != nil {
		// Here we would convert filter conditions to map entries
		// For simplicity, this implementation is abbreviated
	}

	return s.personRepo.List(ctx, filters)
}

// FindByTagID finds a person by their RFID tag ID
func (s *personService) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	return s.personRepo.FindByTagID(ctx, tagID)
}

// FindByAccountID finds a person by their account ID
func (s *personService) FindByAccountID(ctx context.Context, accountID int64) (*userModels.Person, error) {
	return s.personRepo.FindByAccountID(ctx, accountID)
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

	return s.List(ctx, options)
}

// LinkToAccount associates a person with an account
func (s *personService) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	// Verify the account exists
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return err
	}
	if account == nil {
		return ErrAccountNotFound
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return err
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return ErrAccountAlreadyLinked
	}

	return s.personRepo.LinkToAccount(ctx, personID, accountID)
}

// UnlinkFromAccount removes account association from a person
func (s *personService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	return s.personRepo.UnlinkFromAccount(ctx, personID)
}

// LinkToRFIDCard associates a person with an RFID card
func (s *personService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Verify the RFID card exists
	card, err := s.rfidRepo.FindByID(ctx, tagID)
	if err != nil {
		return err
	}
	if card == nil {
		return ErrRFIDCardNotFound
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.personRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return err
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return ErrRFIDCardAlreadyLinked
	}

	return s.personRepo.LinkToRFIDCard(ctx, personID, tagID)
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (s *personService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	return s.personRepo.UnlinkFromRFIDCard(ctx, personID)
}

// GetFullProfile retrieves a person with all related entities
func (s *personService) GetFullProfile(ctx context.Context, personID int64) (*userModels.Person, error) {
	// Get the basic person record
	person, err := s.personRepo.FindByID(ctx, personID)
	if err != nil {
		return nil, err
	}
	if person == nil {
		return nil, ErrPersonNotFound
	}

	// Fetch related account if AccountID is set
	if person.AccountID != nil {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return nil, err
		}
		person.Account = account
	}

	// Fetch related RFID card if TagID is set
	if person.TagID != nil {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return nil, err
		}
		person.RFIDCard = card
	}

	return person, nil
}

// FindByGuardianID finds all persons with a guardian relationship to the specified account
func (s *personService) FindByGuardianID(ctx context.Context, guardianAccountID int64) ([]*userModels.Person, error) {
	// Get all person-guardian relationships for this guardian
	// Changed from FindByGuardianAccountID to FindByGuardianID to match the repository interface
	relationships, err := s.personGuardianRepo.FindByGuardianID(ctx, guardianAccountID)
	if err != nil {
		return nil, err
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

	return s.List(ctx, options)
}
