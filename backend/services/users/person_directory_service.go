package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The person and student half of the retained user service: the people a
// school keeps records about, and the children among them.
//
// The writes reach People Directory (#3349); the reads below are the
// remainder still served by the retained repositories, which that issue is
// moving. The staff and teacher half lives in staff_directory_service.go,
// because School Membership owns those tables.

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
	// opLockStudent is the operation name for the locked child read; it is the
	// spelling the retained repository reported, because callers match on the
	// wrapped sentinels rather than on the text.
	opLockStudent = "find_by_id_for_update"
)

// PersonServiceDependencies contains all dependencies required by the person service
type PersonServiceDependencies struct {
	// PersonDirectory is the owner's person write path (#3349); the
	// composition root binds database/repositories.NewPersonDirectory.
	PersonDirectory PersonWriter
	// StudentDirectory is the owner's child-row access (#3349); the
	// composition root binds database/repositories.NewStudentDirectory.
	StudentDirectory StudentDirectoryLocker
	// Repository dependencies
	PersonRepo    userModels.PersonRepository
	RFIDRepo      RFIDCards
	AccountExists func(context.Context, int64) (bool, error)
	StudentRepo   userModels.StudentRepository
	StaffRepo     userModels.StaffRepository
	TeacherRepo   userModels.TeacherRepository
	// LehrkraftRoles answers whether the staff member's account holds the
	// Lehrkraft role. Required by the caregiver-profile paths: the Lehrkraft
	// role (#1772) is provisioned without a profile on purpose and must not be
	// handed one here.
	LehrkraftRoles LehrkraftRoleQuery
	// PersonnelNumberAudit is required for UpdatePersonnelNumber; the write
	// path refuses to run without it (no change without a trace, #1417).
	PersonnelNumberAudit auditModels.PersonnelNumberChangeCreator

	// Stammdaten storage + audit (#1423). StammdatenAudit is required for
	// every section write; DataAccessLog is required for every financial
	// read — both paths refuse to run unaudited.
	StaffMasterDataRepo    userModels.StaffMasterDataRepository
	StaffQualificationRepo userModels.StaffQualificationRepository
	StaffFinancialRepo     userModels.StaffFinancialDataRepository
	StammdatenAudit        auditModels.StaffMasterDataChangeCreator
	DataAccessLog          auditModels.DataAccessLogRepository

	// Infrastructure
	DB              *bun.DB
	SettingsService configSvc.SettingsService
	Logger          *slog.Logger
}

// CareParticipationResolver is the Care Plan seam behind the dated visibility
// decision this directory read applies (#3350). Care Plan owns the exit and
// withdrawal boundaries it resolves; the directory only asks which of the
// children it selected still take part in care on the given day.
type CareParticipationResolver func(
	ctx context.Context, studentIDs []int64, on, today timezone.Date,
) (map[int64]bool, error)

// personService implements the PersonService interface
type personService struct {
	PersonServiceDependencies
	careParticipation CareParticipationResolver
}

// NewPersonService creates a new person service
func NewPersonService(deps PersonServiceDependencies) PersonService {
	return &personService{PersonServiceDependencies: deps}
}

func WirePersonCareParticipation(service PersonService, resolve CareParticipationResolver) {
	concrete, ok := service.(*personService)
	if !ok {
		panic("person service does not support care-participation wiring")
	}
	concrete.careParticipation = resolve
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
		exists, err := s.AccountExists(ctx, *person.AccountID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if !exists {
			return &UsersError{Op: opCreatePerson, Err: ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		_, _, found, err := s.RFIDRepo.LookupRFIDCard(ctx, *person.TagID)
		if err != nil {
			return &UsersError{Op: opCreatePerson, Err: err}
		}
		if !found {
			return &UsersError{Op: opCreatePerson, Err: ErrRFIDCardNotFound}
		}
	}

	if s.PersonDirectory == nil {
		return &UsersError{Op: opCreatePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.PersonDirectory.CreatePerson(ctx, person); err != nil {
		return &UsersError{Op: opCreatePerson, Err: translateMissingPerson(err)}
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

	if s.PersonDirectory == nil {
		return &UsersError{Op: opUpdatePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.PersonDirectory.UpdatePerson(ctx, person); err != nil {
		return &UsersError{Op: opUpdatePerson, Err: translateMissingPerson(err)}
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
		s.AccountExists, ErrAccountNotFound)
}

// validateRFIDCardIfChanged validates RFID card exists if TagID is being changed
func (s *personService) validateRFIDCardIfChanged(ctx context.Context, person, existingPerson *userModels.Person) error {
	return validateChangedRef(ctx, person.TagID, existingPerson.TagID,
		func(ctx context.Context, id string) (bool, error) {
			_, _, found, err := s.RFIDRepo.LookupRFIDCard(ctx, id)
			return found, err
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

	if s.PersonDirectory == nil {
		return &UsersError{Op: opDeletePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.PersonDirectory.DeletePerson(ctx, person.ID); err != nil {
		return &UsersError{Op: opDeletePerson, Err: translateMissingPerson(err)}
	}
	return nil
}

// errPersonDirectoryUnwired names a composition graph that reaches a person
// write without the owner behind it. It is a configuration error, reported
// rather than panicked so it cannot abort a request mid-transaction.
var errPersonDirectoryUnwired = errors.New("person directory is not configured")

// translateMissingPerson restates the owner's missing person as this package's
// own sentinel, so a row that disappeared between the existence check and the
// write is reported exactly as one that was never there.
func translateMissingPerson(err error) error {
	if errors.Is(err, userModels.ErrPersonRowMissing) {
		return ErrPersonNotFound
	}
	return err
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
	exists, err := s.AccountExists(ctx, accountID)
	if err != nil {
		return &UsersError{Op: opLinkToAccount, Err: err}
	}
	if !exists {
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
	_, _, found, err := s.RFIDRepo.LookupRFIDCard(ctx, tagID)
	if err != nil {
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if !found {
		// Auto-create RFID card on assignment (per RFID Implementation Guide)
		if err := s.RFIDRepo.RegisterRFIDCard(ctx, tagID); err != nil {
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

// LinkStudentToRFIDCard assigns a bracelet to a student, refusing a graduated
// (alumnus) child under a row lock held for the caller's transaction.
//
// The alumnus check the handler already ran happened before the write, and the
// write itself only waits on users.persons. A graduation apply locks the
// student row first and clears the tag second, so an assignment that passed the
// handler gate can sit waiting on the person row while the apply commits, and
// then re-link a bracelet onto a departed child — the exact state graduation
// releases tags to avoid, and one no staff-facing route can undo because every
// one of them 404s on an alumnus. Re-reading the student under the SAME lock
// order the apply uses (student row, then person row) closes the window: either
// this call wins the row and the apply observes the tag it must release, or the
// apply wins and this call sees the alumnus status and refuses (#405 review).
func (s *personService) LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &UsersError{Op: opLinkToRFIDCard, Err: ErrStudentNotFound}
		}
		return &UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if student.Status == userModels.StudentStatusAlumnus {
		return &UsersError{Op: opLinkToRFIDCard, Err: ErrStudentGraduated}
	}

	return s.LinkToRFIDCard(ctx, student.PersonID, tagID)
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

// GetStudentByID retrieves a student by ID.
func (s *personService) GetStudentByID(ctx context.Context, id int64) (*userModels.Student, error) {
	return s.StudentRepo.FindByID(ctx, id)
}

// GetStudentByIDForUpdate retrieves a student by ID under a row lock held until
// the caller's transaction ends. Callers that validate the status before writing
// a row that references the student need it: an unlocked read can be obsolete
// the moment a grade transition commits.
func (s *personService) GetStudentByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	if s.StudentDirectory == nil {
		return nil, &UsersError{Op: opLockStudent, Err: errStudentDirectoryUnwired}
	}
	student, err := s.StudentDirectory.LockStudent(ctx, id)
	return student, translateMissingStudent(opLockStudent, err)
}

// errStudentDirectoryUnwired names a composition graph that reaches a locked
// child read without the owner behind it. It is a configuration error,
// reported rather than panicked so it cannot abort a request mid-transaction.
var errStudentDirectoryUnwired = errors.New("student directory is not configured")

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

// GetParticipationCandidatesByGroupIDs includes alumni so the shared dated
// participation rule, including its actual-presence exception, gets the full
// candidate set. Administrative group readers keep the legacy method above.
func (s *personService) GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*userModels.Student, error) {
	if len(groupIDs) == 0 {
		return []*userModels.Student{}, nil
	}
	return s.StudentRepo.ListByGroupIDsIncludingAlumni(ctx, groupIDs)
}

// GetEligibleStudentsByGroupIDsOnDate retrieves group students whose
// enrollment covers the requested date. Current lifecycle status is only used
// for legacy rows without enrollment dates.
//
// today is the caller's calendar day. It is a parameter rather than a fresh
// timezone.TodayDate() read so a request that spans Berlin midnight keeps one
// notion of "today": re-reading the process clock here could validate one day
// and then build the roster for another, dropping a child who was activated
// immediately and is deliberately part of the current day.
func (s *personService) GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date, today timezone.Date) ([]*userModels.Student, error) {
	students, err := s.GetParticipationCandidatesByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if s.careParticipation == nil {
		return nil, errors.New("person service: care participation resolver is not configured")
	}
	if len(students) == 0 {
		return students, nil
	}
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	participating, err := s.careParticipation(ctx, ids, date, today)
	if err != nil {
		return nil, err
	}
	started := filterStudentsStartedOnDate(students, date, today)
	kept := make([]*userModels.Student, 0, len(started))
	for _, student := range started {
		if participating[student.ID] {
			kept = append(kept, student)
		}
	}
	return kept, nil
}

// filterStudentsStartedOnDate applies the enrollment lower bound after the
// shared participation resolver handled the upper boundary and actual-presence
// exception. Immediate activation
// (enrollment.default_activation_mode = "immediate") as the single deliberate
// exception — the decision service creates an already 'active' student while
// enrolled_from still points at the phase's future start date, and that child
// may check in from today. The override therefore lifts the enrolled_from
// lower bound from today onward only; a past date keeps the bound so nobody is
// retroactively enrolled. Whether a child whose enrollment has not started yet
// is REPORTED for the day is a separate question the day log answers on its
// own — being on the roster only means their records count.
//
// This is deliberately NOT userModels.EnrolledOn (#1565, #2606): that rule also
// applies the enrolled_until upper bound, which would undo the resolver's
// actual-presence exception for a child who is still there after their planned
// care ended. Keep the lower-bound half in step with EnrolledOn.
func filterStudentsStartedOnDate(students []*userModels.Student, date, today timezone.Date) []*userModels.Student {
	eligible := make([]*userModels.Student, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		if student.EnrolledFrom != nil && date.Before(*student.EnrolledFrom) &&
			(student.Status != userModels.StudentStatusActive || date.Before(today)) {
			continue
		}
		if student.EnrolledFrom == nil && student.EnrolledUntil == nil && student.Status == userModels.StudentStatusInactive {
			continue
		}
		eligible = append(eligible, student)
	}
	return eligible
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

// LehrkraftRoleQuery is the consumer-owned port over the Identity & Access
// role administration: whether the account holds the Lehrkraft system role at
// the tenant in context (#3314).
type LehrkraftRoleQuery interface {
	AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error)
}
