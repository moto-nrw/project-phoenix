package users

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// PIN brute-force lockout policy (issue #586 — extracted from the model).
// After PINLockoutThreshold failed PIN entries the account is locked for
// PINLockoutDuration. These mirror the MFA lockout policy in services/auth.
// Per-tenant overrides live behind security.account_lockout_* settings keys.
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
	// opGetStudentsWithGroupsByTeacher is the operation name for GetStudentsWithGroupsByTeacher operations
	opGetStudentsWithGroupsByTeacher = "get students with groups by teacher"
)

// PersonServiceDependencies contains all dependencies required by the person service
type PersonServiceDependencies struct {
	// Repository dependencies
	PersonRepo  userModels.PersonRepository
	RFIDRepo    userModels.RFIDCardRepository
	AccountRepo auth.AccountRepository
	StudentRepo userModels.StudentRepository
	StaffRepo   userModels.StaffRepository
	TeacherRepo userModels.TeacherRepository
	// PersonnelNumberAudit is required for UpdatePersonnelNumber; the write
	// path refuses to run without it (no change without a trace, #1417).
	PersonnelNumberAudit auditModels.PersonnelNumberChangeRepository

	// Infrastructure
	DB              *bun.DB
	SettingsService configSvc.SettingsService
	Logger          *slog.Logger
}

// personService implements the PersonService interface
type personService struct {
	PersonServiceDependencies
}

// NewPersonService creates a new person service
func NewPersonService(deps PersonServiceDependencies) PersonService {
	return &personService{PersonServiceDependencies: deps}
}

// Get retrieves a person by their ID
func (s *personService) Get(ctx context.Context, id interface{}) (*userModels.Person, error) {
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

	person, err := s.PersonRepo.FindWithAccount(ctx, personID)
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

	persons, err := s.PersonRepo.FindByIDs(ctx, ids)
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

	// Set tenant ID from context
	person.SetTenantID(tenant.FromContext(ctx))

	// Check if the account exists if AccountID is set
	if person.AccountID != nil {
		account, err := s.AccountRepo.FindByID(ctx, *person.AccountID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if account == nil {
			return &UsersError{Op: opCreatePerson, Err: ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		card, err := s.RFIDRepo.FindByID(ctx, *person.TagID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if card == nil {
			return &UsersError{Op: opCreatePerson, Err: ErrRFIDCardNotFound}
		}
	}

	if err := s.PersonRepo.Create(ctx, person); err != nil {
		return &UsersError{Op: opCreatePerson, Err: err}
	}

	return nil
}

// Update updates an existing person
func (s *personService) Update(ctx context.Context, person *userModels.Person) error {
	if person.Validate() != nil {
		return &UsersError{Op: opUpdatePerson, Err: person.Validate()}
	}

	existingPerson, err := s.PersonRepo.FindByID(ctx, person.ID)
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

	if err := s.PersonRepo.Update(ctx, person); err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}

	return nil
}

// validateChangedRef checks that a re-pointed reference (account, RFID card)
// still resolves to an existing row. Skips when the new value is unset or
// unchanged; find reports whether the referenced row exists.
func validateChangedRef[T comparable](ctx context.Context, newID, oldID *T, find func(context.Context, T) (bool, error), notFound error) error {
	if newID == nil {
		return nil
	}

	if oldID != nil && *oldID == *newID {
		return nil
	}

	found, err := find(ctx, *newID)
	if err != nil {
		return &UsersError{Op: opUpdatePerson, Err: err}
	}
	if !found {
		return &UsersError{Op: opUpdatePerson, Err: notFound}
	}

	return nil
}

// validateAccountIfChanged validates account exists if AccountID is being changed
func (s *personService) validateAccountIfChanged(ctx context.Context, person, existingPerson *userModels.Person) error {
	return validateChangedRef(ctx, person.AccountID, existingPerson.AccountID,
		func(ctx context.Context, id int64) (bool, error) {
			account, err := s.AccountRepo.FindByID(ctx, id)
			return account != nil, err
		}, ErrAccountNotFound)
}

// validateRFIDCardIfChanged validates RFID card exists if TagID is being changed
func (s *personService) validateRFIDCardIfChanged(ctx context.Context, person, existingPerson *userModels.Person) error {
	return validateChangedRef(ctx, person.TagID, existingPerson.TagID,
		func(ctx context.Context, id string) (bool, error) {
			card, err := s.RFIDRepo.FindByID(ctx, id)
			return card != nil, err
		}, ErrRFIDCardNotFound)
}

// Delete removes a person
func (s *personService) Delete(ctx context.Context, id interface{}) error {
	// Verify the person exists
	person, err := s.PersonRepo.FindByID(ctx, id)
	if err != nil {
		return &UsersError{Op: opDeletePerson, Err: err}
	}
	if person == nil {
		return &UsersError{Op: opDeletePerson, Err: ErrPersonNotFound}
	}

	if err := s.PersonRepo.Delete(ctx, id); err != nil {
		return &UsersError{Op: opDeletePerson, Err: err}
	}
	return nil
}

// List retrieves persons matching the provided query options
func (s *personService) List(ctx context.Context, options *base.QueryOptions) ([]*userModels.Person, error) {
	persons, err := s.PersonRepo.ListWithOptions(ctx, options)
	if err != nil {
		return nil, &UsersError{Op: "list persons", Err: err}
	}
	return persons, nil
}

// FindByTagID finds a person by their RFID tag ID
func (s *personService) FindByTagID(ctx context.Context, tagID string) (*userModels.Person, error) {
	person, err := s.PersonRepo.FindByTagID(ctx, tagID)
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
	person, err := s.PersonRepo.FindByAccountID(ctx, accountID)
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
	account, err := s.AccountRepo.FindByID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if account == nil {
		return &UsersError{Op: opLinkToAccount, Err: ErrAccountNotFound}
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &UsersError{Op: opLinkToAccount, Err: ErrAccountAlreadyLinked}
	}

	if err := s.PersonRepo.LinkToAccount(ctx, personID, accountID); err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	return nil
}

// UnlinkFromAccount removes account association from a person
func (s *personService) UnlinkFromAccount(ctx context.Context, personID int64) error {
	if err := s.PersonRepo.UnlinkFromAccount(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from account", Err: err}
	}
	return nil
}

// LinkToRFIDCard associates a person with an RFID card
func (s *personService) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Check if the RFID card exists, create it if it doesn't (auto-create on assignment)
	card, err := s.RFIDRepo.FindByID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if card == nil {
		// Auto-create RFID card on assignment (per RFID Implementation Guide)
		newCard := &userModels.RFIDCard{
			StringIDModel: base.StringIDModel{ID: tagID},
			Active:        true,
		}
		newCard.SetTenantID(tenant.FromContext(ctx))
		if err := s.RFIDRepo.Create(ctx, newCard); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.PersonRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		// Auto-unlink from previous person (tag override behavior)
		if err := s.PersonRepo.UnlinkFromRFIDCard(ctx, existingPerson.ID); err != nil {
			return &UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	if err := s.PersonRepo.LinkToRFIDCard(ctx, personID, tagID); err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	return nil
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (s *personService) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	if err := s.PersonRepo.UnlinkFromRFIDCard(ctx, personID); err != nil {
		return &UsersError{Op: "unlink from RFID card", Err: err}
	}
	return nil
}

// Entity lookups (issue #584). CONTRACT: repository results and errors are
// returned VERBATIM — no wrapping, no sentinel mapping — because IoT device
// flows branch on sql.ErrNoRows and PyrePortal substring-matches the rendered
// error bodies. See the interface doc comment.

// GetStaffByID retrieves a staff member by ID.
func (s *personService) GetStaffByID(ctx context.Context, id int64) (*userModels.Staff, error) {
	return s.StaffRepo.FindByID(ctx, id)
}

// GetStaffByPersonID retrieves the staff record belonging to a person.
func (s *personService) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	return s.StaffRepo.FindByPersonID(ctx, personID)
}

// ResolveStaffIDByAccountID maps a JWT account id to its staff id via the
// account → person → staff chain.
func (s *personService) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	person, err := s.FindByAccountID(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("person not found for account: %w", err)
	}
	staff, err := s.GetStaffByPersonID(ctx, person.ID)
	if err != nil {
		return 0, fmt.Errorf("staff not found for editor account: %w", err)
	}
	return staff.ID, nil
}

// GetStaffWithPerson retrieves a staff member with person data preloaded.
func (s *personService) GetStaffWithPerson(ctx context.Context, id int64) (*userModels.Staff, error) {
	return s.StaffRepo.FindWithPerson(ctx, id)
}

// GetStaffWithPersonByIDs retrieves multiple staff with person data preloaded.
func (s *personService) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	return s.StaffRepo.FindWithPersonByIDs(ctx, ids)
}

// ListStaffWithPerson retrieves all staff with person data preloaded.
func (s *personService) ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	return s.StaffRepo.ListAllWithPerson(ctx)
}

// ListStaffByRoles retrieves staff holding any of the given roles.
func (s *personService) ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error) {
	return s.StaffRepo.ListStaffByRoles(ctx, roles)
}

// GetTeacherByStaffID retrieves the teacher record for a staff member.
func (s *personService) GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	return s.TeacherRepo.FindByStaffID(ctx, staffID)
}

// GetTeachersByStaffIDs retrieves teacher records for multiple staff members.
func (s *personService) GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	return s.TeacherRepo.FindByStaffIDs(ctx, staffIDs)
}

// GetTeachersBySpecialization retrieves teachers by specialization.
func (s *personService) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	return s.TeacherRepo.FindBySpecialization(ctx, specialization)
}

// GetTeacherWithStaffAndPerson retrieves a teacher with staff and person preloaded.
func (s *personService) GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	return s.TeacherRepo.FindWithStaffAndPerson(ctx, id)
}

// ListTeachersWithStaffAndPerson retrieves all teachers with staff and person preloaded.
func (s *personService) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	return s.TeacherRepo.ListAllWithStaffAndPerson(ctx)
}

// GetStudentByID retrieves a student by ID.
func (s *personService) GetStudentByID(ctx context.Context, id int64) (*userModels.Student, error) {
	return s.StudentRepo.FindByID(ctx, id)
}

// GetStudentByPersonID retrieves the student record belonging to a person.
func (s *personService) GetStudentByPersonID(ctx context.Context, personID int64) (*userModels.Student, error) {
	return s.StudentRepo.FindByPersonID(ctx, personID)
}

// GetStudentsByIDs retrieves multiple students by ID.
func (s *personService) GetStudentsByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	return s.StudentRepo.FindByIDs(ctx, ids)
}

// GetStudentsByGroupID retrieves the students of a group.
func (s *personService) GetStudentsByGroupID(ctx context.Context, groupID int64) ([]*userModels.Student, error) {
	return s.StudentRepo.FindByGroupID(ctx, groupID)
}

// GetStudentsByGroupIDs retrieves the students of multiple groups.
func (s *personService) GetStudentsByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	return s.StudentRepo.FindByGroupIDs(ctx, groupIDs)
}

// CountStudentsByGroupIDs counts students per group in a single query.
func (s *personService) CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	return s.StudentRepo.CountByGroupIDs(ctx, groupIDs)
}

// GetAllStudentsWithGroups retrieves all students with their group info
func (s *personService) GetAllStudentsWithGroups(ctx context.Context) ([]StudentWithGroup, error) {
	studentsWithGroups, err := s.StudentRepo.FindAllWithGroups(ctx)
	if err != nil {
		return nil, &UsersError{Op: "get all students with groups", Err: err}
	}

	results := make([]StudentWithGroup, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		results = append(results, StudentWithGroup{
			Student:   swg.Student,
			GroupName: swg.GroupName,
		})
	}

	return results, nil
}

// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
func (s *personService) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]StudentWithGroup, error) {
	// First verify the teacher exists
	teacher, err := s.TeacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: ErrTeacherNotFound}
	}

	// Use the enhanced repository method to get students with group info
	studentsWithGroups, err := s.StudentRepo.FindByTeacherIDWithGroups(ctx, teacherID)
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

// ---------------------------------------------------------------------------
// Staff write operations (issue #584: moved verbatim out of api/staff)
// ---------------------------------------------------------------------------

// CreateStaffWithTeacher creates a staff record and, when requested, a teacher
// record in one tenant transaction. A failed teacher creation is deliberately
// non-fatal: the staff row still persists (historical api/staff behaviour).
func (s *personService) CreateStaffWithTeacher(ctx context.Context, input CreateStaffInput) (*userModels.Staff, *userModels.Teacher, bool, error) {
	staff := &userModels.Staff{
		PersonID:   input.PersonID,
		StaffNotes: input.StaffNotes,
	}

	var teacher *userModels.Teacher
	teacherCreationFailed := false

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.StaffRepo.Create(ctx, staff); err != nil {
			return err
		}

		if input.IsTeacher {
			teacher = &userModels.Teacher{
				StaffID:        staff.ID,
				Specialization: strings.TrimSpace(input.Specialization),
				Role:           input.Role,
				Qualifications: input.Qualifications,
			}
			if s.TeacherRepo.Create(ctx, teacher) != nil {
				// Still return the staff member even if teacher creation fails
				teacher = nil
				teacherCreationFailed = true
			}
		}

		return nil
	}); err != nil {
		return nil, nil, false, err
	}

	return staff, teacher, teacherCreationFailed, nil
}

// UpdateStaffWithTeacher persists the (already mutated) staff row, reloads it
// with person data, and applies the requested teacher-record change.
// Teacher-record failures are non-fatal; the staff update always persists.
func (s *personService) UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, TeacherAction, error) {
	var teacher *userModels.Teacher
	action := TeacherActionNone

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.StaffRepo.Update(ctx, staff); err != nil {
			return err
		}

		// Reload staff with person data; fall back to loading the person alone.
		if reloaded, err := s.StaffRepo.FindWithPerson(ctx, staff.ID); err == nil {
			*staff = *reloaded
		} else if staff.Person == nil && staff.PersonID > 0 {
			if person, err := s.PersonRepo.FindByID(ctx, staff.PersonID); err == nil {
				staff.Person = person
			}
		}

		// Existing teacher record, if any (lookup errors intentionally ignored).
		existingTeacher, _ := s.TeacherRepo.FindByStaffID(ctx, staff.ID)

		if !isTeacher {
			if existingTeacher != nil {
				teacher = existingTeacher
				action = TeacherActionExisting
			}
			return nil
		}

		if existingTeacher != nil {
			existingTeacher.Specialization = specialization
			existingTeacher.Role = role
			existingTeacher.Qualifications = qualifications
			if s.TeacherRepo.Update(ctx, existingTeacher) != nil {
				action = TeacherActionUpdateFailed
				return nil
			}
			teacher = existingTeacher
			action = TeacherActionUpdated
			return nil
		}

		newTeacher := &userModels.Teacher{
			StaffID:        staff.ID,
			Specialization: specialization,
			Role:           role,
			Qualifications: qualifications,
		}
		if s.TeacherRepo.Create(ctx, newTeacher) != nil {
			action = TeacherActionCreateFailed
			return nil
		}
		teacher = newTeacher
		action = TeacherActionCreated
		return nil
	}); err != nil {
		return nil, TeacherActionNone, err
	}

	return teacher, action, nil
}
