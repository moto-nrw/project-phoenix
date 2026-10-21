package application

import (
	"context"
	"errors"
	"strings"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain/userscontract"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
)

// The staff and teacher half of the person service left services/users in
// #3752. users.staff and users.teachers belong to School Membership and
// Workforce, so the staff write orchestration is bound here over the retained
// repositories and injected into the person service through the
// StaffDirectory port, which hands the repositories to the lookups.
//
// Lookups read the repositories directly, so their result and error stay
// verbatim: callers (IoT device flows, PyrePortal) branch on sql.ErrNoRows and
// render err.Error() into response bodies.

type staffMembershipDirectory struct {
	runtime        peoplePorts.StaffDirectoryRuntime
	persons        peoplePorts.PersonRepository
	staff          peoplePorts.StaffRepository
	teachers       peoplePorts.TeacherRepository
	lehrkraftRoles peoplePorts.LehrkraftRoleQuery
	mayManageStaff func([]string) bool
}

// NewStaffDirectory binds the staff directory over the retained repositories.
func NewStaffDirectory(deps peoplePorts.StaffDirectoryDependencies) peoplePorts.StaffDirectory {
	if deps.Persons == nil {
		panic("staff directory: the person repository is required")
	}
	if deps.Staff == nil {
		panic("staff directory: the staff repository is required")
	}
	if deps.MayManageStaff == nil {
		panic("staff directory: the staff administration check is required")
	}
	return &staffMembershipDirectory{
		runtime: deps.Runtime, persons: deps.Persons, staff: deps.Staff, teachers: deps.Teachers, lehrkraftRoles: deps.LehrkraftRoles,
		mayManageStaff: deps.MayManageStaff,
	}
}

// StaffRows returns the retained staff repository.
func (s *staffMembershipDirectory) StaffRows() peoplePorts.StaffRepository { return s.staff }

// TeacherRows returns the retained teacher repository.
func (s *staffMembershipDirectory) TeacherRows() peoplePorts.TeacherRepository { return s.teachers }

// CreateStaffWithTeacher creates a staff record and, when requested, a teacher
// record in one tenant transaction. A failed teacher creation is deliberately
// non-fatal: the staff row still persists (historical api/staff behaviour).
//
// A person that already carries a live staff record adopts it instead of
// getting a second one. Since #2222 the account-creating paths provision the
// person → staff chain themselves, so the staff creation that follows in the
// same flow finds its row already there; adopting turns what would be a
// duplicate row into the detail update the caller meant.
//
// The returned teacher record reports the state the staff member is actually
// in, not the state the request asked for: an adopted staff row can already
// carry a live caregiver profile, and a request that did not ask for one does
// not remove it (see liveTeacherForStaff).
func (s *staffMembershipDirectory) CreateStaffWithTeacher(ctx context.Context, input peoplePorts.CreateStaffInput) (*peopleValues.Staff, *peopleValues.Teacher, bool, error) {
	var staff *peopleValues.Staff
	var teacher *peopleValues.Teacher
	teacherCreationFailed := false

	if err := s.runtime.InTenantTx(ctx, func(ctx context.Context) error {
		schoolID, err := s.runtime.RequireTenantID(ctx)
		if err != nil {
			return err
		}
		if err := s.runtime.AcquireLock(ctx, peopleValues.StaffPersonMergeLockKey(schoolID), true); err != nil {
			return err
		}
		if err := s.requireLivePersonLocked(ctx, input.PersonID); err != nil {
			return err
		}
		if err := s.refuseCaregiverProfileForLehrkraft(ctx, input.PersonID, input.IsTeacher); err != nil {
			return err
		}

		if staff, err = s.adoptOrCreateStaff(ctx, input); err != nil {
			return err
		}

		if input.IsTeacher {
			teacher, teacherCreationFailed = s.ensureTeacherForStaff(ctx, staff.ID, input)
		} else {
			teacher = s.liveTeacherForStaff(ctx, staff.ID)
		}

		return nil
	}); err != nil {
		return nil, nil, false, err
	}

	return staff, teacher, teacherCreationFailed, nil
}

// requireLivePersonLocked keeps a staff write and a person retirement ordered
// on the person row. A person deleted after an earlier request validation must
// not receive a new active staff membership once this transaction resumes.
func (s *staffMembershipDirectory) requireLivePersonLocked(ctx context.Context, personID int64) error {
	person, err := s.persons.FindByIDForUpdate(ctx, personID)
	if err != nil {
		if peoplePorts.IsRepositoryNoRows(err) {
			return userscontract.ErrPersonNotFound
		}
		return err
	}
	if person == nil || person.DeletedAt != nil {
		return userscontract.ErrPersonNotFound
	}
	return nil
}

// adoptOrCreateStaff adopts the live staff record of the person or creates
// one, inside the caller's transaction.
func (s *staffMembershipDirectory) adoptOrCreateStaff(ctx context.Context, input peoplePorts.CreateStaffInput) (*peopleValues.Staff, error) {
	existing, err := s.staff.FindByPersonID(ctx, input.PersonID)
	if err != nil && !errors.Is(err, transactionruntime.ErrNoRows) {
		return nil, err
	}
	if existing != nil && existing.DeletedAt == nil {
		// Adoption is an edit of a record that is already in the directory,
		// so it owes staff:manage — the same authority PUT /api/staff/{id}
		// requires since #2906. The create permission this route is gated
		// on does not cover overwriting someone's notes or writing a
		// caregiver profile onto them, and neither does users:update,
		// which the plain Betreuer role holds for the child-data surfaces.
		if !s.mayManageStaff(input.ActorPermissions) {
			return nil, peoplePorts.ErrStaffAdoptionNotPermitted
		}
		existing.StaffNotes = input.StaffNotes
		if err := s.staff.Update(ctx, existing); err != nil {
			return nil, err
		}
		return existing, nil
	}
	staff := &peopleValues.Staff{
		PersonID:   input.PersonID,
		StaffNotes: input.StaffNotes,
	}
	if err := s.staff.Create(ctx, staff); err != nil {
		return nil, err
	}
	return staff, nil
}

// refuseCaregiverProfileForLehrkraft rejects a request that would give a
// caregiver profile to an account holding the Lehrkraft system role (#1772).
//
// The role-assignment paths already refuse the combination from the other
// direction: AssignRoleToAccount, the operator role change and the invitation
// flow all reject Lehrkraft on an account that carries a live profile. Staff
// creation was the open side of the same door — a Lehrkraft account is
// provisioned with a staff record and deliberately without a profile, so
// POST /api/staff with is_teacher:true found the record, adopted it and
// created exactly the profile the other paths forbid, along with the caregiver
// permissions the create handler grants on that basis.
//
// Runs inside the caller's transaction, before anything is written.
//
// Fails closed: a person without an account cannot be a Lehrkraft, but an
// unreadable role set is not permission to write the profile — the same call
// the default-permission grant makes when it cannot resolve the roles.
func (s *staffMembershipDirectory) refuseCaregiverProfileForLehrkraft(ctx context.Context, personID int64, isTeacher bool) error {
	if !isTeacher {
		return nil
	}

	person, err := s.persons.FindByID(ctx, personID)
	if err != nil {
		return err
	}
	if person == nil || person.AccountID == nil {
		return nil
	}

	if s.lehrkraftRoles == nil {
		return errors.New("lehrkraft role query is required to decide the caregiver profile")
	}
	isLehrkraft, err := s.lehrkraftRoles.AccountHoldsLehrkraftRole(ctx, *person.AccountID)
	if err != nil {
		return err
	}
	if isLehrkraft {
		return peoplePorts.ErrStaffLehrkraftCaregiverProfile
	}
	return nil
}

// liveTeacherForStaff returns the caregiver profile the staff record already
// carries, or nil.
//
// Read when the request did NOT ask for one, which is not the same as asking
// for it to be gone. Adopting an existing staff row can find a live
// users.teachers row underneath, and reporting that staff member back as
// "no caregiver profile" was wrong twice over: the caller renders a plain staff
// record for someone whose caregiver screens work, and the create handler hands
// out the non-caregiver default permissions on that basis.
//
// Deleting the profile instead is not this path's call, and deliberately so:
// the row carries group supervisions, and removing it is what staff offboarding
// does — the same reason the operator paths refuse a Lehrkraft role change
// rather than clearing the profile out from under it. UpdateStaffWithTeacher
// answers the identical question the identical way (TeacherActionExisting).
//
// Lookup errors are swallowed to nil, matching the non-fatal contract of
// everything else touching the teacher record here: a failed read must not sink
// a staff record that persisted.
func (s *staffMembershipDirectory) liveTeacherForStaff(ctx context.Context, staffID int64) *peopleValues.Teacher {
	existing, err := s.teachers.FindByStaffID(ctx, staffID)
	if err != nil || existing == nil || existing.DeletedAt != nil {
		return nil
	}
	return existing
}

// ensureTeacherForStaff creates the caregiver profile or updates the live one.
// Failures are non-fatal by design (see CreateStaffWithTeacher).
func (s *staffMembershipDirectory) ensureTeacherForStaff(ctx context.Context, staffID int64, input peoplePorts.CreateStaffInput) (*peopleValues.Teacher, bool) {
	existing, err := s.teachers.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, transactionruntime.ErrNoRows) {
		return nil, true
	}
	if existing != nil && existing.DeletedAt == nil {
		existing.Specialization = strings.TrimSpace(input.Specialization)
		existing.Role = input.Role
		existing.Qualifications = input.Qualifications
		if s.teachers.Update(ctx, existing) != nil {
			return nil, true
		}
		return existing, false
	}

	teacher := &peopleValues.Teacher{
		StaffID:        staffID,
		Specialization: strings.TrimSpace(input.Specialization),
		Role:           input.Role,
		Qualifications: input.Qualifications,
	}
	if s.teachers.Create(ctx, teacher) != nil {
		return nil, true
	}
	return teacher, false
}

// UpdateStaffWithTeacher applies the mutable directory fields to a freshly
// locked staff row, reloads it with person data, and applies the requested
// teacher-record change. Teacher-record failures are non-fatal; the staff
// update always persists.
func (s *staffMembershipDirectory) UpdateStaffWithTeacher(ctx context.Context, staff *peopleValues.Staff, isTeacher bool, specialization, role, qualifications string) (*peopleValues.Teacher, peoplePorts.TeacherAction, error) {
	var teacher *peopleValues.Teacher
	action := peoplePorts.TeacherActionNone

	if err := s.runtime.InTenantTx(ctx, func(ctx context.Context) error {
		if err := s.updateStaffRecord(ctx, staff, isTeacher); err != nil {
			return err
		}
		teacher, action = s.applyTeacherChange(ctx, staff.ID, isTeacher, specialization, role, qualifications)
		return nil
	}); err != nil {
		return nil, peoplePorts.TeacherActionNone, err
	}

	return teacher, action, nil
}

// updateStaffRecord writes the mutable directory fields onto the freshly
// locked staff row and reloads it with its person into staff.
func (s *staffMembershipDirectory) updateStaffRecord(ctx context.Context, staff *peopleValues.Staff, isTeacher bool) error {
	currentStaff, err := s.staff.FindByIDForUpdate(ctx, staff.ID)
	if err != nil {
		return err
	}

	// Same guard as the create path: the edit form must not be the way a
	// Lehrkraft account acquires the caregiver profile its role excludes.
	if err := s.refuseCaregiverProfileForLehrkraft(ctx, currentStaff.PersonID, isTeacher); err != nil {
		return err
	}
	currentStaff.PersonID = staff.PersonID
	currentStaff.StaffNotes = staff.StaffNotes

	if err := s.staff.Update(ctx, currentStaff); err != nil {
		return err
	}
	*staff = *currentStaff

	// Reload staff with person data; fall back to loading the person alone.
	if reloaded, err := s.staff.FindWithPerson(ctx, staff.ID); err == nil {
		*staff = *reloaded
	} else if staff.Person == nil && staff.PersonID > 0 {
		if person, err := s.persons.FindByID(ctx, staff.PersonID); err == nil {
			staff.Person = person
		}
	}
	return nil
}

// applyTeacherChange applies the requested teacher-record change after the
// staff update. Its failures are non-fatal and reported as the action.
func (s *staffMembershipDirectory) applyTeacherChange(ctx context.Context, staffID int64, isTeacher bool, specialization, role, qualifications string) (*peopleValues.Teacher, peoplePorts.TeacherAction) {
	// Existing teacher record, if any (lookup errors intentionally ignored).
	existingTeacher, _ := s.teachers.FindByStaffID(ctx, staffID)

	if !isTeacher {
		if existingTeacher != nil {
			return existingTeacher, peoplePorts.TeacherActionExisting
		}
		return nil, peoplePorts.TeacherActionNone
	}

	if existingTeacher != nil {
		existingTeacher.Specialization = specialization
		existingTeacher.Role = role
		existingTeacher.Qualifications = qualifications
		if s.teachers.Update(ctx, existingTeacher) != nil {
			return nil, peoplePorts.TeacherActionUpdateFailed
		}
		return existingTeacher, peoplePorts.TeacherActionUpdated
	}

	newTeacher := &peopleValues.Teacher{
		StaffID:        staffID,
		Specialization: specialization,
		Role:           role,
		Qualifications: qualifications,
	}
	if s.teachers.Create(ctx, newTeacher) != nil {
		return nil, peoplePorts.TeacherActionCreateFailed
	}
	return newTeacher, peoplePorts.TeacherActionCreated
}
