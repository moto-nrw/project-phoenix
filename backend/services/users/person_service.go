package users

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// personService implements the PersonService interface
type personService struct {
	personRepo         userModels.PersonRepository
	rfidRepo           userModels.RFIDCardRepository
	accountRepo        auth.AccountRepository
	personGuardianRepo userModels.PersonGuardianRepository
	studentRepo        userModels.StudentRepository
	staffRepo          userModels.StaffRepository
	teacherRepo        userModels.TeacherRepository
	db                 *bun.DB
	txHandler          *base.TxHandler
}

// NewPersonService creates a new person service
func NewPersonService(
	personRepo userModels.PersonRepository,
	rfidRepo userModels.RFIDCardRepository,
	accountRepo auth.AccountRepository,
	personGuardianRepo userModels.PersonGuardianRepository,
	studentRepo userModels.StudentRepository,
	staffRepo userModels.StaffRepository,
	teacherRepo userModels.TeacherRepository,
	db *bun.DB,
) PersonService {
	return &personService{
		personRepo:         personRepo,
		rfidRepo:           rfidRepo,
		accountRepo:        accountRepo,
		personGuardianRepo: personGuardianRepo,
		studentRepo:        studentRepo,
		staffRepo:          staffRepo,
		teacherRepo:        teacherRepo,
		db:                 db,
		txHandler:          base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *personService) WithTx(tx bun.Tx) interface{} {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var personRepo = s.personRepo
	var rfidRepo = s.rfidRepo
	var accountRepo = s.accountRepo
	var personGuardianRepo = s.personGuardianRepo
	var studentRepo = s.studentRepo
	var staffRepo = s.staffRepo
	var teacherRepo = s.teacherRepo

	// Try to cast repositories to TransactionalRepository and apply the transaction
	if txRepo, ok := s.personRepo.(base.TransactionalRepository); ok {
		personRepo = txRepo.WithTx(tx).(userModels.PersonRepository)
	}
	if txRepo, ok := s.rfidRepo.(base.TransactionalRepository); ok {
		rfidRepo = txRepo.WithTx(tx).(userModels.RFIDCardRepository)
	}
	if txRepo, ok := s.accountRepo.(base.TransactionalRepository); ok {
		accountRepo = txRepo.WithTx(tx).(auth.AccountRepository)
	}
	if txRepo, ok := s.personGuardianRepo.(base.TransactionalRepository); ok {
		personGuardianRepo = txRepo.WithTx(tx).(userModels.PersonGuardianRepository)
	}
	if txRepo, ok := s.studentRepo.(base.TransactionalRepository); ok {
		studentRepo = txRepo.WithTx(tx).(userModels.StudentRepository)
	}
	if txRepo, ok := s.staffRepo.(base.TransactionalRepository); ok {
		staffRepo = txRepo.WithTx(tx).(userModels.StaffRepository)
	}
	if txRepo, ok := s.teacherRepo.(base.TransactionalRepository); ok {
		teacherRepo = txRepo.WithTx(tx).(userModels.TeacherRepository)
	}

	// Return a new service with the transaction
	return &personService{
		personRepo:         personRepo,
		rfidRepo:           rfidRepo,
		accountRepo:        accountRepo,
		personGuardianRepo: personGuardianRepo,
		studentRepo:        studentRepo,
		staffRepo:          staffRepo,
		teacherRepo:        teacherRepo,
		db:                 s.db,
		txHandler:          s.txHandler.WithTx(tx),
	}
}

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
			return nil, &UsersError{Op: "get person", Err: fmt.Errorf("invalid ID type")}
		}

		person, err := repo.FindWithAccount(ctx, personID)
		if err != nil {
			return nil, &UsersError{Op: "get person", Err: err}
		}
		if person == nil {
			return nil, &UsersError{Op: "get person", Err: ErrPersonNotFound}
		}
		return person, nil
	}

	// Fallback to regular FindByID
	person, err := s.personRepo.FindByID(ctx, id)
	if err != nil {
		return nil, &UsersError{Op: "get person", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: "get person", Err: ErrPersonNotFound}
	}
	return person, nil
}

// Create creates a new person
func (s *personService) Create(ctx context.Context, person *userModels.Person) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return &UsersError{Op: "create person", Err: err}
	}

	// Note: Removed the requirement for TagID or AccountID
	// Students can be created without either identifier

	// Check if the account exists if AccountID is set
	if person.AccountID != nil {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return &UsersError{Op: "create person", Err: err}
		}
		if account == nil {
			return &UsersError{Op: "create person", Err: ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return &UsersError{Op: "create person", Err: err}
		}
		if card == nil {
			return &UsersError{Op: "create person", Err: ErrRFIDCardNotFound}
		}
	}

	if err := s.personRepo.Create(ctx, person); err != nil {
		return &UsersError{Op: "create person", Err: err}
	}

	return nil
}

// Update updates an existing person
func (s *personService) Update(ctx context.Context, person *userModels.Person) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return &UsersError{Op: "update person", Err: err}
	}

	// Note: The requirement for either TagID or AccountID has been removed
	// Persons can now exist without either identifier

	// Check if the person exists
	existingPerson, err := s.personRepo.FindByID(ctx, person.ID)
	if err != nil {
		return &UsersError{Op: "update person", Err: err}
	}
	if existingPerson == nil {
		return &UsersError{Op: "update person", Err: ErrPersonNotFound}
	}

	// Check if the account exists if AccountID is set and changed
	if person.AccountID != nil &&
		(existingPerson.AccountID == nil || *existingPerson.AccountID != *person.AccountID) {
		account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return &UsersError{Op: "update person", Err: err}
		}
		if account == nil {
			return &UsersError{Op: "update person", Err: ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set and changed
	if person.TagID != nil &&
		(existingPerson.TagID == nil || *existingPerson.TagID != *person.TagID) {
		card, err := s.rfidRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return &UsersError{Op: "update person", Err: err}
		}
		if card == nil {
			return &UsersError{Op: "update person", Err: ErrRFIDCardNotFound}
		}
	}

	if err := s.personRepo.Update(ctx, person); err != nil {
		return &UsersError{Op: "update person", Err: err}
	}

	return nil
}

// Delete removes a person
func (s *personService) Delete(ctx context.Context, id interface{}) error {
	// Verify the person exists
	person, err := s.personRepo.FindByID(ctx, id)
	if err != nil {
		return &UsersError{Op: "delete person", Err: err}
	}
	if person == nil {
		return &UsersError{Op: "delete person", Err: ErrPersonNotFound}
	}

	if err := s.personRepo.Delete(ctx, id); err != nil {
		return &UsersError{Op: "delete person", Err: err}
	}
	return nil
}

// List retrieves persons matching the provided query options
func (s *personService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	// TODO: Follow education.groups pattern - add ListWithOptions to PersonRepository interface
	// and call it directly instead of converting to map[string]interface{}
	// See education service for the correct implementation pattern

	// Convert QueryOptions to map[string]interface{} for repository
	filters := make(map[string]interface{})
	if options != nil && options.Filter != nil {
		// Here we would convert filter conditions to map entries
		// For simplicity, this implementation is abbreviated
		// TODO: Implement filter conversion
		_ = options.Filter // Mark as intentionally unused for now
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

// LinkToAccount associates a person with an account
func (s *personService) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	// Verify the account exists
	account, err := s.accountRepo.FindByID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: "link to account", Err: err}
	}
	if account == nil {
		return &UsersError{Op: "link to account", Err: ErrAccountNotFound}
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: "link to account", Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &UsersError{Op: "link to account", Err: ErrAccountAlreadyLinked}
	}

	if err := s.personRepo.LinkToAccount(ctx, personID, accountID); err != nil {
		return &UsersError{Op: "link to account", Err: err}
	}
	return nil
}

// UnlinkFromAccount removes account association from a person
func (s *personService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	if err := s.personRepo.UnlinkFromAccount(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from account", Err: err}
	}
	return nil
}

// LinkToRFIDCard associates a person with an RFID card
func (s *personService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Verify the RFID card exists
	card, err := s.rfidRepo.FindByID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: "link to RFID card", Err: err}
	}
	if card == nil {
		return &UsersError{Op: "link to RFID card", Err: ErrRFIDCardNotFound}
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.personRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: "link to RFID card", Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &UsersError{Op: "link to RFID card", Err: ErrRFIDCardAlreadyLinked}
	}

	if err := s.personRepo.LinkToRFIDCard(ctx, personID, tagID); err != nil {
		return &UsersError{Op: "link to RFID card", Err: err}
	}
	return nil
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (s *personService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	if err := s.personRepo.UnlinkFromRFIDCard(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from RFID card", Err: err}
	}
	return nil
}

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

// StudentRepository returns the student repository
func (s *personService) StudentRepository() userModels.StudentRepository { return s.studentRepo }

// StaffRepository returns the staff repository
func (s *personService) StaffRepository() userModels.StaffRepository {
	return s.staffRepo
}

// TeacherRepository returns the teacher repository
func (s *personService) TeacherRepository() userModels.TeacherRepository {
	return s.teacherRepo
}

// ListAvailableRFIDCards returns RFID cards that are not assigned to any person
func (s *personService) ListAvailableRFIDCards(ctx context.Context) ([]*userModels.RFIDCard, error) {
	// First, get all active RFID cards
	filters := map[string]interface{}{
		"active": true,
	}

	allCards, err := s.rfidRepo.List(ctx, filters)
	if err != nil {
		return nil, &UsersError{Op: "list all RFID cards", Err: err}
	}

	// Get all persons to check which cards are assigned
	persons, err := s.personRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: "list all persons", Err: err}
	}

	// Create a map of assigned tag IDs for fast lookup
	assignedTags := make(map[string]bool)
	for _, person := range persons {
		if person.TagID != nil {
			assignedTags[*person.TagID] = true
		}
	}

	// Filter out assigned cards
	var availableCards []*userModels.RFIDCard
	for _, card := range allCards {
		if !assignedTags[card.ID] {
			availableCards = append(availableCards, card)
		}
	}

	return availableCards, nil
}

// ValidateStaffPIN validates a staff member's PIN and returns the staff record
func (s *personService) ValidateStaffPIN(ctx context.Context, pin string) (*userModels.Staff, error) {
	if pin == "" {
		return nil, &UsersError{Op: "validate staff PIN", Err: errors.New("PIN cannot be empty")}
	}

	// Get all accounts that have PINs set
	accounts, err := s.accountRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN", Err: err}
	}

	// Check PIN against all accounts that have PINs
	for _, account := range accounts {
		if account.HasPIN() && !account.IsPINLocked() {
			// Use the VerifyPIN method from the account model
			if account.VerifyPIN(pin) {
				// PIN is valid - find the person linked to this account
				person, err := s.personRepo.FindByAccountID(ctx, account.ID)
				if err != nil {
					return nil, &UsersError{Op: "validate staff PIN - find person", Err: err}
				}
				if person == nil {
					// Account has PIN but no person linked - continue searching
					continue
				}

				// Find the staff record for this person
				staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
				if err != nil {
					return nil, &UsersError{Op: "validate staff PIN - find staff", Err: err}
				}
				if staff == nil {
					// Person exists but is not staff - continue searching
					continue
				}

				// Reset PIN attempts on successful authentication
				account.ResetPINAttempts()
				if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
					// Log error but don't fail authentication
					_ = updateErr
				}

				// Load the person relation for the authenticated staff
				staff.Person = person
				
				return staff, nil
			} else {
				// Increment failed attempts for this account
				account.IncrementPINAttempts()
				
				// Update the account record with new attempt count/lock status
				if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
					// Log error but don't fail the authentication check
					// Continue checking other accounts
					_ = updateErr // Mark as intentionally ignored
				}
			}
		}
	}

	// No account found with matching PIN
	return nil, &UsersError{Op: "validate staff PIN", Err: ErrInvalidPIN}
}

// GetStudentsByTeacher retrieves students supervised by a teacher (through group assignments)
func (s *personService) GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*userModels.Student, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: "get students by teacher", Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: "get students by teacher", Err: ErrTeacherNotFound}
	}

	// Use the repository method to get students by teacher ID
	students, err := s.studentRepo.FindByTeacherID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: "get students by teacher", Err: err}
	}

	return students, nil
}

// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
func (s *personService) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: "get students with groups by teacher", Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: "get students with groups by teacher", Err: ErrTeacherNotFound}
	}

	// Use the enhanced repository method to get students with group info
	studentsWithGroups, err := s.studentRepo.FindByTeacherIDWithGroups(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: "get students with groups by teacher", Err: err}
	}

	// Convert to service layer struct
	results := make([]StudentWithGroup, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		result := StudentWithGroup{
			Student:   swg.Student,
			GroupName: swg.GroupName,
		}
		results = append(results, result)
	}

	return results, nil
}
