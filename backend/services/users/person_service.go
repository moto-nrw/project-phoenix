package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

const (
	// opGetPerson is the operation name for Get operations
	opGetPerson = "get person"
	// opCreatePerson is the operation name for Create operations
	opCreatePerson = "create person"
	// opUpdatePerson is the operation name for Update operations
	opUpdatePerson = "update person"
	// opDeletePerson is the operation name for Delete operations
	opDeletePerson = "delete person"
	// opLinkToAccount is the operation name for LinkToAccount operations
	opLinkToAccount = "link to account"
	// opLinkToRFIDCard is the operation name for LinkToRFIDCard operations
	opLinkToRFIDCard = "link to RFID card"
	// opValidateStaffPIN is the operation name for ValidateStaffPIN operations
	opValidateStaffPIN = "validate staff PIN"
	// opValidateStaffPINSpecific is the operation name for ValidateStaffPINForSpecificStaff operations
	opValidateStaffPINSpecific = "validate staff PIN for specific staff"
	// opGetStudentsByTeacher is the operation name for GetStudentsByTeacher operations
	opGetStudentsByTeacher = "get students by teacher"
	// opGetStudentsWithGroupsByTeacher is the operation name for GetStudentsWithGroupsByTeacher operations
	opGetStudentsWithGroupsByTeacher = "get students with groups by teacher"
)

// PersonServiceDependencies contains all dependencies required by the person service
type PersonServiceDependencies struct {
	// Repository dependencies
	PersonRepo         userModels.PersonRepository
	RFIDRepo           userModels.RFIDCardRepository
	AccountRepo        auth.AccountRepository
	PersonGuardianRepo userModels.PersonGuardianRepository
	StudentRepo        userModels.StudentRepository
	StaffRepo          userModels.StaffRepository
	TeacherRepo        userModels.TeacherRepository

	// Infrastructure
	DB *bun.DB
}

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
func NewPersonService(deps PersonServiceDependencies) PersonService {
	return &personService{
		personRepo:         deps.PersonRepo,
		rfidRepo:           deps.RFIDRepo,
		accountRepo:        deps.AccountRepo,
		personGuardianRepo: deps.PersonGuardianRepo,
		studentRepo:        deps.StudentRepo,
		staffRepo:          deps.StaffRepo,
		teacherRepo:        deps.TeacherRepo,
		db:                 deps.DB,
		txHandler:          base.NewTxHandler(deps.DB),
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
			return &UsersError{Op: opCreatePerson, Err: ErrAccountNotFound}
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
	if person.Validate() != nil {
		return &UsersError{Op: opUpdatePerson, Err: person.Validate()}
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
		return &UsersError{Op: opUpdatePerson, Err: ErrAccountNotFound}
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

// List retrieves persons matching the provided query options
func (s *personService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	persons, err := s.personRepo.ListWithOptions(ctx, options)
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
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if account == nil {
		return &UsersError{Op: opLinkToAccount, Err: ErrAccountNotFound}
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.personRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &UsersError{Op: opLinkToAccount, Err: ErrAccountAlreadyLinked}
	}

	if err := s.personRepo.LinkToAccount(ctx, personID, accountID); err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
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
	// Check if the RFID card exists, create it if it doesn't (auto-create on assignment)
	card, err := s.rfidRepo.FindByID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if card == nil {
		// Auto-create RFID card on assignment (per RFID Implementation Guide)
		newCard := &userModels.RFIDCard{
			StringIDModel: base.StringIDModel{ID: tagID},
			Active:        true,
		}
		if err := s.rfidRepo.Create(ctx, newCard); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.personRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		// Auto-unlink from previous person (tag override behavior)
		if err := s.personRepo.UnlinkFromRFIDCard(ctx, existingPerson.ID); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	if err := s.personRepo.LinkToRFIDCard(ctx, personID, tagID); err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
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
		return nil, &UsersError{Op: opValidateStaffPIN, Err: errors.New("PIN cannot be empty")}
	}

	accounts, err := s.accountRepo.List(ctx, nil)
	if err != nil {
		return nil, &UsersError{Op: opValidateStaffPIN, Err: err}
	}

	for _, account := range accounts {
		staff, err := s.tryValidatePINForAccount(ctx, account, pin)
		if err != nil {
			// Propagate repository errors immediately
			return nil, &UsersError{Op: opValidateStaffPIN, Err: err}
		}
		if staff != nil {
			return staff, nil
		}
	}

	return nil, &UsersError{Op: opValidateStaffPIN, Err: ErrInvalidPIN}
}

// tryValidatePINForAccount attempts to validate PIN for a single account and returns staff if successful
// Returns (staff, nil) if PIN is valid and staff found
// Returns (nil, nil) if PIN is invalid or account has no staff record
// Returns (nil, error) if repository operations fail
func (s *personService) tryValidatePINForAccount(ctx context.Context, account *auth.Account, pin string) (*userModels.Staff, error) {
	if !account.HasPIN() || account.IsPINLocked() {
		return nil, nil
	}

	if !account.VerifyPIN(pin) {
		s.handleFailedPINAttempt(ctx, account)
		return nil, nil
	}

	staff, err := s.findStaffByAccount(ctx, account)
	if err != nil {
		return nil, err // Propagate repository errors
	}

	if staff != nil {
		s.handleSuccessfulPINAuth(ctx, account)
		// Load person details (ignore error as this is supplementary data)
		staff.Person, _ = s.personRepo.FindByAccountID(ctx, account.ID)
		return staff, nil
	}

	return nil, nil
}

// findStaffByAccount finds staff record from account, returning error if repository operations fail
func (s *personService) findStaffByAccount(ctx context.Context, account *auth.Account) (*userModels.Staff, error) {
	person, err := s.personRepo.FindByAccountID(ctx, account.ID)
	if err != nil {
		// Distinguish between "not found" and actual errors
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found is OK - account might not be linked to person
		}
		return nil, err // Propagate repository errors
	}

	if person == nil {
		return nil, nil // No person linked to account
	}

	staff, err := s.staffRepo.FindByPersonID(ctx, person.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Person exists but is not staff
		}
		return nil, err // Propagate repository errors
	}

	return staff, nil
}

// handleSuccessfulPINAuth resets PIN attempts after successful authentication
func (s *personService) handleSuccessfulPINAuth(ctx context.Context, account *auth.Account) {
	account.ResetPINAttempts()
	_ = s.accountRepo.Update(ctx, account)
}

// handleFailedPINAttempt increments PIN attempts after failed authentication
func (s *personService) handleFailedPINAttempt(ctx context.Context, account *auth.Account) {
	account.IncrementPINAttempts()
	_ = s.accountRepo.Update(ctx, account)
}

// ValidateStaffPINForSpecificStaff validates a PIN for a specific staff member
func (s *personService) ValidateStaffPINForSpecificStaff(ctx context.Context, staffID int64, pin string) (*userModels.Staff, error) {
	if pin == "" {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("PIN cannot be empty")}
	}

	// Get the specific staff member
	staff, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find staff", Err: err}
	}
	if staff == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member not found")}
	}

	// Get the person associated with this staff member
	person, err := s.personRepo.FindByID(ctx, staff.PersonID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find person", Err: err}
	}
	if person == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("person not found for staff member")}
	}

	// Check if person has an account
	if person.AccountID == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member has no account")}
	}

	// Get the account
	account, err := s.accountRepo.FindByID(ctx, *person.AccountID)
	if err != nil {
		return nil, &UsersError{Op: "validate staff PIN for specific staff - find account", Err: err}
	}
	if account == nil {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("account not found")}
	}

	// Check if account has PIN and is not locked
	if !account.HasPIN() {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("staff member has no PIN set")}
	}
	if account.IsPINLocked() {
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: errors.New("account is locked")}
	}

	// Verify the PIN
	if !account.VerifyPIN(pin) {
		// Increment failed attempts
		account.IncrementPINAttempts()
		if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
			// Log error but don't fail the authentication check
			_ = updateErr
		}
		return nil, &UsersError{Op: opValidateStaffPINSpecific, Err: ErrInvalidPIN}
	}

	// PIN is valid - reset attempts
	account.ResetPINAttempts()
	if updateErr := s.accountRepo.Update(ctx, account); updateErr != nil {
		// Log error but don't fail authentication
		_ = updateErr
	}

	// Load the person relation for the authenticated staff
	staff.Person = person

	return staff, nil
}

// GetStudentsByTeacher retrieves students supervised by a teacher (through group assignments)
func (s *personService) GetStudentsByTeacher(ctx context.Context, teacherID int64) ([]*userModels.Student, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: ErrTeacherNotFound}
	}

	// Use the repository method to get students by teacher ID
	students, err := s.studentRepo.FindByTeacherID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsByTeacher, Err: err}
	}

	return students, nil
}

// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
func (s *personService) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error) {
	// First verify the teacher exists
	teacher, err := s.teacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: ErrTeacherNotFound}
	}

	// Use the enhanced repository method to get students with group info
	studentsWithGroups, err := s.studentRepo.FindByTeacherIDWithGroups(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
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
