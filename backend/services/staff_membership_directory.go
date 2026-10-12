package services

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The staff and teacher half of the person service left services/users in
// #3752. users.staff and users.teachers belong to School Membership and
// Workforce, so the lookups and the staff write orchestration are bound here
// over the retained repositories and injected into the person service through
// the peopleCompose.StaffDirectory port.
//
// Reads return the repository result and error verbatim: callers (IoT device
// flows, PyrePortal) branch on sql.ErrNoRows and render err.Error() into
// response bodies.

// The staff write refusals. The staff directory raises them and the staff
// membership runtime classifies them, both in this composition, so they live
// here rather than in People Directory's error vocabulary (#3753). The
// messages are rendered verbatim and must not change.
var (
	// ErrStaffAdoptionNotPermitted indicates a staff-creation request landed on
	// a person who already carries a live staff record, from a caller that may
	// only create. Adopting that record writes the notes and the caregiver
	// fields of someone who is already in the directory, which is an edit — and
	// POST /api/staff is gated on users:create alone.
	//
	// Since #2906 the required authority is staff:manage — the same one
	// PUT /api/staff/{id} needs. Admins hold it through the admin:* wildcard,
	// so this refuses the direct-API case, not the staff form.
	ErrStaffAdoptionNotPermitted = errors.New("Für das Ändern eines vorhandenen Mitarbeiter-Datensatzes fehlt die Berechtigung") //nolint:staticcheck // ST1005: user-facing German message

	// ErrStaffLehrkraftCaregiverProfile indicates a caregiver profile was
	// requested for an account holding the Lehrkraft system role (#1772). That
	// role is class_day:read only and is provisioned without a profile on
	// purpose; the role-assignment paths refuse the same combination from the
	// other direction (ErrRoleLehrkraftCaregiverProfile).
	ErrStaffLehrkraftCaregiverProfile = errors.New("Ein Lehrkraft-Konto kann kein Betreuungsprofil erhalten") //nolint:staticcheck // ST1005: user-facing German message

	// ErrStaffInUse indicates staff has attendance records or active supervisions
	ErrStaffInUse = errors.New("Personal kann nicht gelöscht werden: Mitarbeiter/in hat aktive Aufsichten oder Anwesenheitseinträge") //nolint:staticcheck // ST1005: user-facing German message
)

// LehrkraftRoleQuery is the consumer-owned port over the Identity & Access
// role administration: whether the account holds the Lehrkraft system role at
// the tenant in context (#3314).
type LehrkraftRoleQuery interface {
	AccountHoldsLehrkraftRole(ctx context.Context, accountID int64) (bool, error)
}

// StaffDirectoryDependencies are the retained collaborators of the staff
// directory.
type StaffDirectoryDependencies struct {
	DB             *bun.DB
	Persons        userModels.PersonRepository
	Staff          userModels.StaffRepository
	Teachers       userModels.TeacherRepository
	LehrkraftRoles LehrkraftRoleQuery
}

type staffMembershipDirectory struct {
	db             *bun.DB
	persons        userModels.PersonRepository
	staff          userModels.StaffRepository
	teachers       userModels.TeacherRepository
	lehrkraftRoles LehrkraftRoleQuery
}

// NewStaffDirectory binds the staff directory over the retained repositories.
func NewStaffDirectory(deps StaffDirectoryDependencies) peopleCompose.StaffDirectory {
	if deps.Staff == nil {
		panic("staff directory: the staff repository is required")
	}
	return &staffMembershipDirectory{
		db: deps.DB, persons: deps.Persons, staff: deps.Staff, teachers: deps.Teachers, lehrkraftRoles: deps.LehrkraftRoles,
	}
}

func (s *staffMembershipDirectory) GetStaffByID(ctx context.Context, id int64) (*userModels.Staff, error) {
	return s.staff.FindByID(ctx, id)
}

func (s *staffMembershipDirectory) GetStaffByPersonID(ctx context.Context, personID int64) (*userModels.Staff, error) {
	return s.staff.FindByPersonID(ctx, personID)
}

func (s *staffMembershipDirectory) GetStaffWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Staff, error) {
	return s.staff.FindWithPersonByIDs(ctx, ids)
}

func (s *staffMembershipDirectory) ListStaffWithPerson(ctx context.Context) ([]*userModels.Staff, error) {
	return s.staff.ListAllWithPerson(ctx)
}

func (s *staffMembershipDirectory) ListStaffByRoles(ctx context.Context, roles []string) ([]*userModels.StaffWithRoleInfo, error) {
	return s.staff.ListStaffByRoles(ctx, roles)
}

func (s *staffMembershipDirectory) GetTeacherByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	return s.teachers.FindByStaffID(ctx, staffID)
}

func (s *staffMembershipDirectory) GetTeachersByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	return s.teachers.FindByStaffIDs(ctx, staffIDs)
}

func (s *staffMembershipDirectory) GetTeachersBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	return s.teachers.FindBySpecialization(ctx, specialization)
}

func (s *staffMembershipDirectory) GetTeacherWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	return s.teachers.FindWithStaffAndPerson(ctx, id)
}

func (s *staffMembershipDirectory) ListTeachersWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	return s.teachers.ListAllWithStaffAndPerson(ctx)
}

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
func (s *staffMembershipDirectory) CreateStaffWithTeacher(ctx context.Context, input peopleCompose.CreateStaffInput) (*userModels.Staff, *userModels.Teacher, bool, error) {
	var staff *userModels.Staff
	var teacher *userModels.Teacher
	teacherCreationFailed := false

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.refuseCaregiverProfileForLehrkraft(ctx, input.PersonID, input.IsTeacher); err != nil {
			return err
		}

		existing, err := s.staff.FindByPersonID(ctx, input.PersonID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if existing != nil && existing.DeletedAt == nil {
			// Adoption is an edit of a record that is already in the directory,
			// so it owes staff:manage — the same authority PUT /api/staff/{id}
			// requires since #2906. The create permission this route is gated
			// on does not cover overwriting someone's notes or writing a
			// caregiver profile onto them, and neither does users:update,
			// which the plain Betreuer role holds for the child-data surfaces.
			if !securityruntime.HasPermission(securityruntime.PermissionStaffManage, input.ActorPermissions) {
				return ErrStaffAdoptionNotPermitted
			}
			existing.StaffNotes = input.StaffNotes
			if err := s.staff.Update(ctx, existing); err != nil {
				return err
			}
			staff = existing
		} else {
			staff = &userModels.Staff{
				PersonID:   input.PersonID,
				StaffNotes: input.StaffNotes,
			}
			if err := s.staff.Create(ctx, staff); err != nil {
				return err
			}
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
		return ErrStaffLehrkraftCaregiverProfile
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
// answers the identical question the identical way (peopleCompose.TeacherActionExisting).
//
// Lookup errors are swallowed to nil, matching the non-fatal contract of
// everything else touching the teacher record here: a failed read must not sink
// a staff record that persisted.
func (s *staffMembershipDirectory) liveTeacherForStaff(ctx context.Context, staffID int64) *userModels.Teacher {
	existing, err := s.teachers.FindByStaffID(ctx, staffID)
	if err != nil || existing == nil || existing.DeletedAt != nil {
		return nil
	}
	return existing
}

// ensureTeacherForStaff creates the caregiver profile or updates the live one.
// Failures are non-fatal by design (see CreateStaffWithTeacher).
func (s *staffMembershipDirectory) ensureTeacherForStaff(ctx context.Context, staffID int64, input peopleCompose.CreateStaffInput) (*userModels.Teacher, bool) {
	existing, err := s.teachers.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
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

	teacher := &userModels.Teacher{
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
func (s *staffMembershipDirectory) UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, peopleCompose.TeacherAction, error) {
	var teacher *userModels.Teacher
	action := peopleCompose.TeacherActionNone

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.db, tenantID, func(ctx context.Context, _ bun.Tx) error {
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

		// Existing teacher record, if any (lookup errors intentionally ignored).
		existingTeacher, _ := s.teachers.FindByStaffID(ctx, staff.ID)

		if !isTeacher {
			if existingTeacher != nil {
				teacher = existingTeacher
				action = peopleCompose.TeacherActionExisting
			}
			return nil
		}

		if existingTeacher != nil {
			existingTeacher.Specialization = specialization
			existingTeacher.Role = role
			existingTeacher.Qualifications = qualifications
			if s.teachers.Update(ctx, existingTeacher) != nil {
				action = peopleCompose.TeacherActionUpdateFailed
				return nil
			}
			teacher = existingTeacher
			action = peopleCompose.TeacherActionUpdated
			return nil
		}

		newTeacher := &userModels.Teacher{
			StaffID:        staff.ID,
			Specialization: specialization,
			Role:           role,
			Qualifications: qualifications,
		}
		if s.teachers.Create(ctx, newTeacher) != nil {
			action = peopleCompose.TeacherActionCreateFailed
			return nil
		}
		teacher = newTeacher
		action = peopleCompose.TeacherActionCreated
		return nil
	}); err != nil {
		return nil, peopleCompose.TeacherActionNone, err
	}

	return teacher, action, nil
}
