package application

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
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
	// opGetStudentsWithGroupsByTeacher is the operation name for GetStudentsWithGroupsByTeacher operations
	opGetStudentsWithGroupsByTeacher = "get students with groups by teacher"
	// opLockStudent is the operation name for the locked child read; it is the
	// spelling the retained repository reported, because callers match on the
	// wrapped sentinels rather than on the text.
	opLockStudent = "find_by_id_for_update"
)

// RetainedPersonDirectory is the retained person service, moved out of services/users
// (#3753): the people a school keeps records about, and the children among
// them, in the model-typed shape its consumers' ports name.
//
// The writes reach People Directory (#3349); the reads below are the
// remainder still served by the retained repositories. The staff and teacher
// half is the StaffDirectory port (#3752), because School Membership and
// Workforce own those tables. There is no interface for the whole surface:
// every consumer declares a port with only the methods it calls (#3771), and
// the composition root passes this type.
type RetainedPersonDirectory struct {
	deps peoplePorts.PersonDirectoryDependencies
	// StaffDirectory is embedded so its lookups and writes are the person
	// directory's own; the deps field of the same name feeds it.
	peoplePorts.StaffDirectory
}

// NewRetainedPersonDirectory creates the retained person directory.
func NewRetainedPersonDirectory(deps peoplePorts.PersonDirectoryDependencies) *RetainedPersonDirectory {
	return &RetainedPersonDirectory{deps: deps, StaffDirectory: deps.StaffDirectory}
}

// WithCareParticipation returns a copy of the directory whose dated roster
// read asks resolve. The root passes the resolver at construction; a test graph
// that composes Care Plan after the directory derives the configured copy.
func (s *RetainedPersonDirectory) WithCareParticipation(resolve peoplePorts.CareParticipationResolver) peoplePorts.PersonDirectory {
	deps := s.deps
	deps.CareParticipation = resolve
	return NewRetainedPersonDirectory(deps)
}

// Get retrieves a person by their ID
func (s *RetainedPersonDirectory) Get(ctx context.Context, id interface{}) (*peopleValues.DirectoryPersonSnapshot, error) {
	// Convert id to int64
	var personID int64
	switch v := id.(type) {
	case int:
		personID = int64(v)
	case int64:
		personID = v
	default:
		return nil, &userscontract.UsersError{Op: opGetPerson, Err: fmt.Errorf("invalid ID type")}
	}

	person, err := s.deps.PersonRepo.FindWithAccount(ctx, personID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: opGetPerson, Err: err}
	}
	if person == nil {
		return nil, &userscontract.UsersError{Op: opGetPerson, Err: userscontract.ErrPersonNotFound}
	}
	return person, nil
}

// GetByIDs retrieves multiple persons by their IDs in a single query
func (s *RetainedPersonDirectory) GetByIDs(ctx context.Context, ids []int64) (map[int64]*peopleValues.DirectoryPersonSnapshot, error) {
	if len(ids) == 0 {
		return make(map[int64]*peopleValues.DirectoryPersonSnapshot), nil
	}

	persons, err := s.deps.PersonRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "get persons by IDs", Err: err}
	}

	return persons, nil
}

// Create creates a new person
func (s *RetainedPersonDirectory) Create(ctx context.Context, person *peopleValues.DirectoryPersonSnapshot) error {
	// Apply business rules and validation
	if err := person.Validate(); err != nil {
		return &userscontract.UsersError{Op: opCreatePerson, Err: err}
	}

	// Set tenant ID from context
	person.SetTenantID(tenant.FromContext(ctx))

	// Check if the account exists if AccountID is set
	if person.AccountID != nil {
		exists, err := s.deps.AccountExists(ctx, *person.AccountID)
		if err != nil {
			return &userscontract.UsersError{Op: opCreatePerson, Err: err}
		}
		if !exists {
			return &userscontract.UsersError{Op: opCreatePerson, Err: userscontract.ErrAccountNotFound}
		}
	}

	// Check if the RFID card exists if TagID is set
	if person.TagID != nil {
		_, _, found, err := s.deps.RFIDRepo.LookupRFIDCard(ctx, *person.TagID)
		if err != nil {
			return &userscontract.UsersError{Op: opCreatePerson, Err: err}
		}
		if !found {
			return &userscontract.UsersError{Op: opCreatePerson, Err: userscontract.ErrRFIDCardNotFound}
		}
	}

	if s.deps.PersonDirectory == nil {
		return &userscontract.UsersError{Op: opCreatePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.deps.PersonDirectory.CreatePerson(ctx, person); err != nil {
		return &userscontract.UsersError{Op: opCreatePerson, Err: translateMissingPerson(err)}
	}

	return nil
}

// Update updates an existing person
func (s *RetainedPersonDirectory) Update(ctx context.Context, person *peopleValues.DirectoryPersonSnapshot) error {
	if person.Validate() != nil {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: person.Validate()}
	}

	existingPerson, err := s.deps.PersonRepo.FindByID(ctx, person.ID)
	if err != nil {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: err}
	}
	if existingPerson == nil {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: userscontract.ErrPersonNotFound}
	}

	if err := s.validateAccountIfChanged(ctx, person, existingPerson); err != nil {
		return err
	}

	if err := s.validateRFIDCardIfChanged(ctx, person, existingPerson); err != nil {
		return err
	}

	if s.deps.PersonDirectory == nil {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.deps.PersonDirectory.UpdatePerson(ctx, person); err != nil {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: translateMissingPerson(err)}
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
		return &userscontract.UsersError{Op: opUpdatePerson, Err: err}
	}
	if !found {
		return &userscontract.UsersError{Op: opUpdatePerson, Err: notFound}
	}

	return nil
}

// validateAccountIfChanged validates account exists if AccountID is being changed
func (s *RetainedPersonDirectory) validateAccountIfChanged(ctx context.Context, person, existingPerson *peopleValues.DirectoryPersonSnapshot) error {
	return validateChangedRef(ctx, person.AccountID, existingPerson.AccountID,
		s.deps.AccountExists, userscontract.ErrAccountNotFound)
}

// validateRFIDCardIfChanged validates RFID card exists if TagID is being changed
func (s *RetainedPersonDirectory) validateRFIDCardIfChanged(ctx context.Context, person, existingPerson *peopleValues.DirectoryPersonSnapshot) error {
	return validateChangedRef(ctx, person.TagID, existingPerson.TagID,
		func(ctx context.Context, id string) (bool, error) {
			_, _, found, err := s.deps.RFIDRepo.LookupRFIDCard(ctx, id)
			return found, err
		}, userscontract.ErrRFIDCardNotFound)
}

// Delete removes a person
func (s *RetainedPersonDirectory) Delete(ctx context.Context, id interface{}) error {
	// Verify the person exists
	person, err := s.deps.PersonRepo.FindByID(ctx, id)
	if err != nil {
		return &userscontract.UsersError{Op: opDeletePerson, Err: err}
	}
	if person == nil {
		return &userscontract.UsersError{Op: opDeletePerson, Err: userscontract.ErrPersonNotFound}
	}

	if s.deps.PersonDirectory == nil {
		return &userscontract.UsersError{Op: opDeletePerson, Err: errPersonDirectoryUnwired}
	}
	if err := s.deps.PersonDirectory.DeletePerson(ctx, person.ID); err != nil {
		return &userscontract.UsersError{Op: opDeletePerson, Err: translateMissingPerson(err)}
	}
	return nil
}

// errPersonDirectoryUnwired names a composition graph that reaches a person
// write without the owner behind it. It is a configuration error, reported
// rather than panicked so it cannot abort a request mid-transaction.
var errPersonDirectoryUnwired = errors.New("person directory is not configured")

// translateMissingPerson restates the owner's missing person as the retained
// sentinel, so a row that disappeared between the existence check and the
// write is reported exactly as one that was never there.
func translateMissingPerson(err error) error {
	if errors.Is(err, peopleValues.ErrPersonRowMissing) {
		return userscontract.ErrPersonNotFound
	}
	return err
}

// FindByTagID finds a person by their RFID tag ID
func (s *RetainedPersonDirectory) FindByTagID(ctx context.Context, tagID string) (*peopleValues.DirectoryPersonSnapshot, error) {
	person, err := s.deps.PersonRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "find person by tag ID", Err: err}
	}
	if person == nil {
		return nil, &userscontract.UsersError{Op: "find person by tag ID", Err: userscontract.ErrPersonNotFound}
	}
	return person, nil
}

// FindByAccountID finds a person by their account ID
func (s *RetainedPersonDirectory) FindByAccountID(ctx context.Context, accountID int64) (*peopleValues.DirectoryPersonSnapshot, error) {
	person, err := s.deps.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "find person by account ID", Err: err}
	}
	if person == nil {
		return nil, &userscontract.UsersError{Op: "find person by account ID", Err: userscontract.ErrPersonNotFound}
	}
	return person, nil
}

// LinkToAccount associates a person with an account
func (s *RetainedPersonDirectory) LinkToAccount(ctx context.Context, personID int64, accountID int64) error {
	// Verify the account exists
	exists, err := s.deps.AccountExists(ctx, accountID)
	if err != nil {
		return &userscontract.UsersError{Op: opLinkToAccount, Err: err}
	}
	if !exists {
		return &userscontract.UsersError{Op: opLinkToAccount, Err: userscontract.ErrAccountNotFound}
	}

	// Check if the account is already linked to another person
	existingPerson, err := s.deps.PersonRepo.FindByAccountID(ctx, accountID)
	if err != nil {
		return &userscontract.UsersError{Op: opLinkToAccount, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		return &userscontract.UsersError{Op: opLinkToAccount, Err: userscontract.ErrAccountAlreadyLinked}
	}

	if err := s.deps.PersonRepo.LinkToAccount(ctx, personID, accountID); err != nil {
		return &userscontract.UsersError{Op: opLinkToAccount, Err: err}
	}
	return nil
}

// UnlinkFromAccount removes account association from a person
func (s *RetainedPersonDirectory) UnlinkFromAccount(ctx context.Context, personID int64) error {
	if err := s.deps.PersonRepo.UnlinkFromAccount(ctx, personID); err != nil {
		return &userscontract.UsersError{Op: "unlink from account", Err: err}
	}
	return nil
}

// LinkToRFIDCard associates a person with an RFID card
func (s *RetainedPersonDirectory) LinkToRFIDCard(ctx context.Context, personID int64, tagID string) error {
	// Check if the RFID card exists, create it if it doesn't (auto-create on assignment)
	_, _, found, err := s.deps.RFIDRepo.LookupRFIDCard(ctx, tagID)
	if err != nil {
		return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if !found {
		// Auto-create RFID card on assignment (per RFID Implementation Guide)
		if err := s.deps.RFIDRepo.RegisterRFIDCard(ctx, tagID); err != nil {
			return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	// Check if the card is already linked to another person
	existingPerson, err := s.deps.PersonRepo.FindByTagID(ctx, tagID)
	if err != nil {
		return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if existingPerson != nil && existingPerson.ID != personID {
		// Auto-unlink from previous person (tag override behavior)
		if err := s.deps.PersonRepo.UnlinkFromRFIDCard(ctx, existingPerson.ID); err != nil {
			return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
		}
	}

	if err := s.deps.PersonRepo.LinkToRFIDCard(ctx, personID, tagID); err != nil {
		return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
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
func (s *RetainedPersonDirectory) LinkStudentToRFIDCard(ctx context.Context, studentID int64, tagID string) error {
	student, err := s.deps.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		if errors.Is(err, transactionruntime.ErrNoRows) {
			return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: userscontract.ErrStudentNotFound}
		}
		return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: err}
	}
	if student.Status == peopleValues.StudentStatusAlumnus {
		return &userscontract.UsersError{Op: opLinkToRFIDCard, Err: userscontract.ErrStudentGraduated}
	}

	return s.LinkToRFIDCard(ctx, student.PersonID, tagID)
}

// UnlinkFromRFIDCard removes RFID card association from a person
func (s *RetainedPersonDirectory) UnlinkFromRFIDCard(ctx context.Context, personID int64) error {
	if err := s.deps.PersonRepo.UnlinkFromRFIDCard(ctx, personID); err != nil {
		return &userscontract.UsersError{Op: "unlink from RFID card", Err: err}
	}
	return nil
}

// Entity lookups (issue #584). CONTRACT: repository results and errors are
// returned VERBATIM — no wrapping, no sentinel mapping — because IoT device
// flows branch on sql.ErrNoRows and PyrePortal substring-matches the rendered
// error bodies.

// GetStudentByID retrieves a student by ID.
func (s *RetainedPersonDirectory) GetStudentByID(ctx context.Context, id int64) (*peopleValues.StudentState, error) {
	return s.deps.StudentRepo.FindByID(ctx, id)
}

// GetStudentByIDForUpdate retrieves a student by ID under a row lock held until
// the caller's transaction ends. Callers that validate the status before writing
// a row that references the student need it: an unlocked read can be obsolete
// the moment a grade transition commits.
func (s *RetainedPersonDirectory) GetStudentByIDForUpdate(ctx context.Context, id int64) (*peopleValues.StudentState, error) {
	if s.deps.StudentDirectory == nil {
		return nil, &userscontract.UsersError{Op: opLockStudent, Err: errStudentDirectoryUnwired}
	}
	student, err := s.deps.StudentDirectory.LockStudent(ctx, id)
	return student, translateMissingStudent(opLockStudent, err)
}

// errStudentDirectoryUnwired names a composition graph that reaches a locked
// child read without the owner behind it. It is a configuration error,
// reported rather than panicked so it cannot abort a request mid-transaction.
var errStudentDirectoryUnwired = errors.New("student directory is not configured")

// StudentRows returns the retained student repository.
func (s *RetainedPersonDirectory) StudentRows() peoplePorts.StudentRepository {
	return s.deps.StudentRepo
}

// GetParticipationCandidatesByGroupIDs includes alumni so the shared dated
// participation rule, including its actual-presence exception, gets the full
// candidate set. Administrative group readers keep the legacy method above.
func (s *RetainedPersonDirectory) GetParticipationCandidatesByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopleValues.StudentState, error) {
	if len(groupIDs) == 0 {
		return []*peopleValues.StudentState{}, nil
	}
	return s.deps.StudentRepo.ListByGroupIDsIncludingAlumni(ctx, groupIDs)
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
func (s *RetainedPersonDirectory) GetEligibleStudentsByGroupIDsOnDate(ctx context.Context, groupIDs []int64, date, today calendar.Date) ([]*peopleValues.StudentState, error) {
	students, err := s.GetParticipationCandidatesByGroupIDs(ctx, groupIDs)
	if err != nil {
		return nil, err
	}
	if s.deps.CareParticipation == nil {
		return nil, errors.New("person service: care participation resolver is not configured")
	}
	if len(students) == 0 {
		return students, nil
	}
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		ids = append(ids, student.ID)
	}
	participating, err := s.deps.CareParticipation(ctx, ids, date, today)
	if err != nil {
		return nil, err
	}
	started := filterStudentsStartedOnDate(students, date, today)
	kept := make([]*peopleValues.StudentState, 0, len(started))
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
// This is deliberately NOT peopleValues.EnrolledOn (#1565, #2606): that rule also
// applies the enrolled_until upper bound, which would undo the resolver's
// actual-presence exception for a child who is still there after their planned
// care ended. Keep the lower-bound half in step with EnrolledOn.
func filterStudentsStartedOnDate(students []*peopleValues.StudentState, date, today calendar.Date) []*peopleValues.StudentState {
	eligible := make([]*peopleValues.StudentState, 0, len(students))
	for _, student := range students {
		if student == nil {
			continue
		}
		if studentStartedOnDate(student.EnrolledFrom, student.EnrolledUntil, string(student.Status), date, today) {
			eligible = append(eligible, student)
		}
	}
	return eligible
}

// studentStartedOnDate is that lower bound for one child, over the three
// fields it reads.
func studentStartedOnDate(enrolledFrom, enrolledUntil *calendar.Date, status string, date, today calendar.Date) bool {
	if enrolledFrom != nil && date.Before(*enrolledFrom) &&
		(status != string(peopleValues.StudentStatusActive) || date.Before(today)) {
		return false
	}
	if enrolledFrom == nil && enrolledUntil == nil && status == string(peopleValues.StudentStatusInactive) {
		return false
	}
	return true
}

// CountStudentsByGroupIDs counts students per group in a single query.
func (s *RetainedPersonDirectory) CountStudentsByGroupIDs(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	return s.deps.StudentRepo.CountByGroupIDs(ctx, groupIDs)
}

// GetAllStudentsWithGroups retrieves all students with their group info
func (s *RetainedPersonDirectory) GetAllStudentsWithGroups(ctx context.Context) ([]peopleValues.StudentRosterEntry, error) {
	studentsWithGroups, err := s.deps.StudentRepo.FindAllWithGroups(ctx)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "get all students with groups", Err: err}
	}

	results := make([]peopleValues.StudentRosterEntry, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		results = append(results, peopleValues.StudentRosterEntry{
			Record:    swg.Record,
			GroupName: swg.GroupName,
		})
	}

	return results, nil
}

// GetStudentsWithGroupsByTeacher retrieves students with group info supervised by a teacher
func (s *RetainedPersonDirectory) GetStudentsWithGroupsByTeacher(ctx context.Context, teacherID int64) ([]peopleValues.StudentRosterEntry, error) {
	// First verify the teacher exists
	teacher, err := s.deps.TeacherRepo.FindByID(ctx, teacherID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	if teacher == nil {
		return nil, &userscontract.UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: userscontract.ErrTeacherNotFound}
	}

	// Use the enhanced repository method to get students with group info
	studentsWithGroups, err := s.deps.StudentRepo.FindByTeacherIDWithGroups(ctx, teacherID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}

	// Convert to service layer struct
	results := make([]peopleValues.StudentRosterEntry, 0, len(studentsWithGroups))
	for _, swg := range studentsWithGroups {
		result := peopleValues.StudentRosterEntry{
			Record:    swg.Record,
			GroupName: swg.GroupName,
		}
		results = append(results, result)
	}

	return results, nil
}

// GetStudentsWithGroupsByTeacherStaffIDs retrieves the union of students
// supervised by teachers belonging to any supplied staff ID.
func (s *RetainedPersonDirectory) GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]peopleValues.StudentRosterEntry, error) {
	if len(staffIDs) == 0 {
		return []peopleValues.StudentRosterEntry{}, nil
	}
	rows, err := s.deps.StudentRepo.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
	if err != nil {
		return nil, &userscontract.UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	results := make([]peopleValues.StudentRosterEntry, 0, len(rows))
	for _, row := range rows {
		results = append(results, peopleValues.StudentRosterEntry{Record: row.Record, GroupName: row.GroupName})
	}
	return results, nil
}

// ResolveStaffIDByAccountID maps a JWT account id to its staff id via the
// account → person → staff chain.
func (s *RetainedPersonDirectory) ResolveStaffIDByAccountID(ctx context.Context, accountID int64) (int64, error) {
	person, err := s.FindByAccountID(ctx, accountID)
	if err != nil {
		return 0, fmt.Errorf("person not found for account: %w", err)
	}
	staff, err := s.StaffRows().FindByPersonID(ctx, person.ID)
	if err != nil {
		return 0, fmt.Errorf("staff not found for editor account: %w", err)
	}
	return staff.ID, nil
}

// ListActiveCaregivers is the canonical operational caregiver lookup.
func (s *RetainedPersonDirectory) ListActiveCaregivers(ctx context.Context) ([]*peopleValues.ActiveCaregiver, error) {
	caregivers, err := s.deps.TeacherRepo.ListActiveCaregivers(ctx)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "list active caregivers", Err: err}
	}
	return caregivers, nil
}

// FindActiveCaregiverByAccountID returns the active caregiver bound to the
// account.
func (s *RetainedPersonDirectory) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*peopleValues.ActiveCaregiver, error) {
	caregiver, err := s.deps.TeacherRepo.FindActiveCaregiverByAccountID(ctx, accountID)
	if err != nil {
		return nil, &userscontract.UsersError{Op: "find active caregiver by account ID", Err: err}
	}
	return caregiver, nil
}

// translateMissingStudent restates the owner's missing child in the error shape
// the retained callers branch on: a DatabaseError wrapping base.ErrNotFound and
// sql.ErrNoRows (usersRepo.MissingStudentError).
func translateMissingStudent(op string, err error) error {
	if !errors.Is(err, peopleValues.ErrStudentRowMissing) {
		return err
	}
	return peoplePorts.MissingStudentError(op)
}
