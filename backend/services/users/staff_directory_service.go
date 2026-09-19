package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// The staff and teacher half of the retained user service: who works at the
// school, which of them teach, and the group-scoped child lists that only a
// teacher assignment can answer.
//
// It stays here rather than moving to People Directory with the person and
// student directory (#3349): users.staff and users.teachers belong to School
// Membership, and these reads are about those tables.

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

// GetStudentsWithGroupsByTeacherStaffIDs retrieves the union of students
// supervised by teachers belonging to any supplied staff ID.
func (s *personService) GetStudentsWithGroupsByTeacherStaffIDs(ctx context.Context, staffIDs []int64) ([]StudentWithGroup, error) {
	if len(staffIDs) == 0 {
		return []StudentWithGroup{}, nil
	}
	rows, err := s.StudentRepo.FindByTeacherStaffIDsWithGroups(ctx, staffIDs)
	if err != nil {
		return nil, &UsersError{Op: opGetStudentsWithGroupsByTeacher, Err: err}
	}
	results := make([]StudentWithGroup, 0, len(rows))
	for _, row := range rows {
		results = append(results, StudentWithGroup{Student: row.Student, GroupName: row.GroupName})
	}
	return results, nil
}

// ---------------------------------------------------------------------------
// Staff write operations (issue #584: moved verbatim out of api/staff)
// ---------------------------------------------------------------------------

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
func (s *personService) CreateStaffWithTeacher(ctx context.Context, input CreateStaffInput) (*userModels.Staff, *userModels.Teacher, bool, error) {
	var staff *userModels.Staff
	var teacher *userModels.Teacher
	teacherCreationFailed := false

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		if err := s.refuseCaregiverProfileForLehrkraft(ctx, input.PersonID, input.IsTeacher); err != nil {
			return err
		}

		existing, err := s.StaffRepo.FindByPersonID(ctx, input.PersonID)
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
			if !authorize.HasPermission(permissions.StaffManage, input.ActorPermissions) {
				return ErrStaffAdoptionNotPermitted
			}
			existing.StaffNotes = input.StaffNotes
			if err := s.StaffRepo.Update(ctx, existing); err != nil {
				return err
			}
			staff = existing
		} else {
			staff = &userModels.Staff{
				PersonID:   input.PersonID,
				StaffNotes: input.StaffNotes,
			}
			if err := s.StaffRepo.Create(ctx, staff); err != nil {
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
func (s *personService) refuseCaregiverProfileForLehrkraft(ctx context.Context, personID int64, isTeacher bool) error {
	if !isTeacher {
		return nil
	}

	person, err := s.PersonRepo.FindByID(ctx, personID)
	if err != nil {
		return err
	}
	if person == nil || person.AccountID == nil {
		return nil
	}

	if s.LehrkraftRoles == nil {
		return errors.New("lehrkraft role query is required to decide the caregiver profile")
	}
	isLehrkraft, err := s.LehrkraftRoles.AccountHoldsLehrkraftRole(ctx, *person.AccountID)
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
// answers the identical question the identical way (TeacherActionExisting).
//
// Lookup errors are swallowed to nil, matching the non-fatal contract of
// everything else touching the teacher record here: a failed read must not sink
// a staff record that persisted.
func (s *personService) liveTeacherForStaff(ctx context.Context, staffID int64) *userModels.Teacher {
	existing, err := s.TeacherRepo.FindByStaffID(ctx, staffID)
	if err != nil || existing == nil || existing.DeletedAt != nil {
		return nil
	}
	return existing
}

// ensureTeacherForStaff creates the caregiver profile or updates the live one.
// Failures are non-fatal by design (see CreateStaffWithTeacher).
func (s *personService) ensureTeacherForStaff(ctx context.Context, staffID int64, input CreateStaffInput) (*userModels.Teacher, bool) {
	existing, err := s.TeacherRepo.FindByStaffID(ctx, staffID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, true
	}
	if existing != nil && existing.DeletedAt == nil {
		existing.Specialization = strings.TrimSpace(input.Specialization)
		existing.Role = input.Role
		existing.Qualifications = input.Qualifications
		if s.TeacherRepo.Update(ctx, existing) != nil {
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
	if s.TeacherRepo.Create(ctx, teacher) != nil {
		return nil, true
	}
	return teacher, false
}

// UpdateStaffWithTeacher applies the mutable directory fields to a freshly
// locked staff row, reloads it with person data, and applies the requested
// teacher-record change. Teacher-record failures are non-fatal; the staff
// update always persists.
func (s *personService) UpdateStaffWithTeacher(ctx context.Context, staff *userModels.Staff, isTeacher bool, specialization, role, qualifications string) (*userModels.Teacher, TeacherAction, error) {
	var teacher *userModels.Teacher
	action := TeacherActionNone

	tenantID := tenant.FromContext(ctx)
	if err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(ctx context.Context, _ bun.Tx) error {
		currentStaff, err := s.StaffRepo.FindByIDForUpdate(ctx, staff.ID)
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

		if err := s.StaffRepo.Update(ctx, currentStaff); err != nil {
			return err
		}
		*staff = *currentStaff

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
