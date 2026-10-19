package owners

import (
	"context"
	"errors"
	"fmt"

	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// TeacherMembershipRepository serves users.TeacherRepository over the School
// Membership capability.
type TeacherMembershipRepository struct {
	Membership schoolmembership.Capability
	Deps       *staffMembershipDeps
}

func teacherFieldsFromLegacy(teacher *peopleValues.Teacher) schoolmembership.TeacherFields {
	return schoolmembership.TeacherFields{
		StaffID:        teacher.StaffID,
		Specialization: teacher.Specialization,
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}
}

func applyTeacherToLegacy(target *peopleValues.Teacher, value schoolmembership.Teacher) {
	target.ID = value.ID
	target.CreatedAt = value.CreatedAt
	target.UpdatedAt = value.UpdatedAt
	target.SetTenantID(value.TenantID)
	target.StaffID = value.StaffID
	target.Specialization = value.Specialization
	target.Role = value.Role
	target.Qualifications = value.Qualifications
	target.DeletedAt = value.DeletedAt
}

func toLegacyTeacher(value schoolmembership.Teacher) *peopleValues.Teacher {
	teacher := new(peopleValues.Teacher)
	applyTeacherToLegacy(teacher, value)
	return teacher
}

func toLegacyTeacherList(values []schoolmembership.Teacher) []*peopleValues.Teacher {
	result := make([]*peopleValues.Teacher, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyTeacher(value))
	}
	return result
}

func (r TeacherMembershipRepository) Create(ctx context.Context, entity *peopleValues.Teacher) error {
	if entity == nil {
		return repositoryError("create teacher", errors.New("teacher cannot be nil"))
	}
	created, err := r.Membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: teacherFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create teacher", err)
	}
	applyTeacherToLegacy(entity, created)
	return nil
}

func (r TeacherMembershipRepository) Update(ctx context.Context, entity *peopleValues.Teacher) error {
	if entity == nil {
		return repositoryError("update teacher", errors.New("teacher cannot be nil"))
	}
	updated, err := r.Membership.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{ID: entity.ID, TeacherFields: teacherFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update teacher", err)
	}
	applyTeacherToLegacy(entity, updated)
	return nil
}

func (r TeacherMembershipRepository) Delete(ctx context.Context, id any) error {
	teacherID, err := MembershipID(id)
	if err != nil {
		return repositoryError("delete teacher", err)
	}
	return membershipError("delete teacher", r.Membership.DeleteTeacher(ctx, teacherID))
}

func (r TeacherMembershipRepository) FindByID(ctx context.Context, id any) (*peopleValues.Teacher, error) {
	teacherID, err := MembershipID(id)
	if err != nil {
		return nil, repositoryError("find teacher by id", err)
	}
	value, err := r.Membership.FindTeacher(ctx, teacherID)
	if err != nil {
		return nil, membershipError("find teacher by id", err)
	}
	return toLegacyTeacher(value), nil
}

// FindByStaffID returns (nil, nil) when the staff member is not a teacher —
// the legacy contract every caller branches on.
func (r TeacherMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*peopleValues.Teacher, error) {
	value, err := r.Membership.FindTeacherByStaff(ctx, staffID)
	if err != nil {
		if errors.Is(err, schoolmembership.ErrTeacherNotFound) {
			return nil, nil
		}
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyTeacher(value), nil
}

func (r TeacherMembershipRepository) FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*peopleValues.Teacher, error) {
	if len(staffIDs) == 0 {
		return make(map[int64]*peopleValues.Teacher), nil
	}
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, membershipError("find by staff IDs", err)
	}
	result := make(map[int64]*peopleValues.Teacher, len(values))
	for _, teacher := range toLegacyTeacherList(values) {
		result[teacher.StaffID] = teacher
	}
	return result, nil
}

func (r TeacherMembershipRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*peopleValues.Teacher, error) {
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{Specialization: specialization})
	if err != nil {
		return nil, membershipError("find by specialization", err)
	}
	return toLegacyTeacherList(values), nil
}

func teacherFilterFromLegacy(filters map[string]any) (schoolmembership.TeacherFilter, error) {
	filter := schoolmembership.TeacherFilter{}
	for field, value := range filters {
		if value == nil {
			continue
		}
		apply, ok := teacherFilterFields[field]
		if !ok {
			return filter, fmt.Errorf("unsupported teacher filter %q", field)
		}
		if err := apply(&filter, field, value); err != nil {
			return filter, err
		}
	}
	return filter, nil
}

// teacherFilterFields translates each supported legacy filter key.
var teacherFilterFields = map[string]func(*schoolmembership.TeacherFilter, string, any) error{
	"id": func(filter *schoolmembership.TeacherFilter, _ string, value any) error {
		id, err := MembershipID(value)
		if err == nil {
			filter.IDs = append(filter.IDs, id)
		}
		return err
	},
	"staff_id": func(filter *schoolmembership.TeacherFilter, _ string, value any) error {
		id, err := MembershipID(value)
		if err == nil {
			filter.StaffIDs = append(filter.StaffIDs, id)
		}
		return err
	},
	"specialization": func(filter *schoolmembership.TeacherFilter, field string, value any) error {
		text, err := legacyFilterString("teacher", field, value)
		filter.Specialization = text
		return err
	},
	"specialization_like": func(filter *schoolmembership.TeacherFilter, field string, value any) error {
		text, err := legacyFilterString("teacher", field, value)
		filter.SpecializationContains = text
		return err
	},
	"role_like": func(filter *schoolmembership.TeacherFilter, field string, value any) error {
		text, err := legacyFilterString("teacher", field, value)
		filter.RoleContains = text
		return err
	},
	"has_qualifications": func(filter *schoolmembership.TeacherFilter, field string, value any) error {
		has, err := legacyFilterBool("teacher", field, value)
		if err == nil {
			filter.HasQualifications = &has
		}
		return err
	},
}

func (r TeacherMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*peopleValues.Teacher, error) {
	filter, err := teacherFilterFromLegacy(filters)
	if err != nil {
		return nil, repositoryError("list teachers", err)
	}
	values, err := r.Membership.ListTeachers(ctx, filter)
	if err != nil {
		return nil, membershipError("list teachers", err)
	}
	return toLegacyTeacherList(values), nil
}

// FindByGroupID resolves the group's teacher assignments through the education
// owner and then loads those teachers from the membership owner.
func (r TeacherMembershipRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*peopleValues.Teacher, error) {
	const op = "find by group ID"
	assignments, err := r.Deps.groupTeachers.FindByGroup(ctx, groupID)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	if len(assignments) == 0 {
		return []*peopleValues.Teacher{}, nil
	}
	ids := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.TeacherID)
	}
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, membershipError(op, err)
	}
	return toLegacyTeacherList(values), nil
}

// hydrateTeacherStaff attaches the staff member and their person to every
// teacher. Staff rows are loaded WITH tombstones because the legacy join had
// no deleted_at predicate on users.staff.
func (r TeacherMembershipRepository) hydrateTeacherStaff(ctx context.Context, teachers []*peopleValues.Teacher, liveStaffOnly bool) ([]*peopleValues.Teacher, error) {
	if len(teachers) == 0 {
		return teachers, nil
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffValues, err := r.Membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: !liveStaffOnly})
	if err != nil {
		return nil, err
	}
	staffByID := make(map[int64]*peopleValues.Staff, len(staffValues))
	personIDs := make([]int64, 0, len(staffValues))
	for _, value := range staffValues {
		staffByID[value.ID] = toLegacyStaff(value)
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := r.Deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	// The legacy query INNER JOINed staff and persons, so a teacher without
	// either simply did not appear.
	result := make([]*peopleValues.Teacher, 0, len(teachers))
	for _, teacher := range teachers {
		staff, found := staffByID[teacher.StaffID]
		if !found {
			continue
		}
		person, found := persons[staff.PersonID]
		if !found {
			continue
		}
		staff.Person = person
		teacher.Staff = staff
		result = append(result, teacher)
	}
	return result, nil
}

func (r TeacherMembershipRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*peopleValues.Teacher, error) {
	const op = "find with staff and person"
	value, err := r.Membership.FindTeacher(ctx, id)
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, []*peopleValues.Teacher{toLegacyTeacher(value)}, false)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	if len(teachers) == 0 {
		return nil, membershipNotFound(op)
	}
	return teachers[0], nil
}

func (r TeacherMembershipRepository) ListAllWithStaffAndPerson(ctx context.Context) ([]*peopleValues.Teacher, error) {
	const op = "list all with staff and person"
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, toLegacyTeacherList(values), true)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	return teachers, nil
}

func (r TeacherMembershipRepository) FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*peopleValues.Teacher, error) {
	const op = "find with staff and person by IDs"
	if len(ids) == 0 {
		return []*peopleValues.Teacher{}, nil
	}
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, toLegacyTeacherList(values), false)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	// FindWithStaffAndPersonByIDs historically did not project the teacher's
	// tenant_id (Teacher.TenantID serializes as "tenant_id", so projecting it
	// would be a wire change).
	for _, teacher := range teachers {
		teacher.SetTenantID(0)
	}
	return teachers, nil
}

// activeCaregivers lists School Membership's active caregivers of the
// caller's school over the People Directory persons and the Identity &
// Access account facts, in the picker order.
func (r TeacherMembershipRepository) activeCaregivers(ctx context.Context, accountID *int64) ([]*peopleValues.ActiveCaregiver, error) {
	values, err := schoolmembership.ActiveCaregivers(ctx, r.Membership, caregiverDirectory{r.Deps}, peopleCompose.CallerTenantID(ctx), accountID)
	if err != nil {
		return nil, err
	}
	schoolmembership.SortActiveCaregivers(values)
	result := make([]*peopleValues.ActiveCaregiver, 0, len(values))
	for _, value := range values {
		result = append(result, &peopleValues.ActiveCaregiver{
			AccountID: value.AccountID, PersonID: value.PersonID, StaffID: value.StaffID, TeacherID: value.TeacherID,
			FirstName: value.FirstName, LastName: value.LastName, Email: value.Email,
			CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		})
	}
	return result, nil
}

// caregiverDirectory serves School Membership's caregiver directory from the
// People Directory persons and the Identity & Access account query.
type caregiverDirectory struct{ deps *staffMembershipDeps }

func (d caregiverDirectory) FindCaregiverPersons(ctx context.Context, personIDs []int64) (map[int64]schoolmembership.CaregiverPerson, error) {
	persons, err := d.deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]schoolmembership.CaregiverPerson, len(persons))
	for id, person := range persons {
		if person != nil {
			result[id] = schoolmembership.CaregiverPerson{ID: person.ID, TenantID: person.GetTenantID(), AccountID: person.AccountID, FirstName: person.FirstName, LastName: person.LastName}
		}
	}
	return result, nil
}

func (d caregiverDirectory) ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error) {
	return d.deps.identity.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
}

func (d caregiverDirectory) ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error) {
	return d.deps.identity.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, roleNames, tenantID)
}

func (d caregiverDirectory) ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error) {
	return d.deps.identity.ListAccountEmails(ctx, accountIDs)
}

func (r TeacherMembershipRepository) ListActiveCaregivers(ctx context.Context) ([]*peopleValues.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, nil)
	if err != nil {
		return nil, membershipError("list active caregivers", err)
	}
	return caregivers, nil
}

func (r TeacherMembershipRepository) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*peopleValues.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, &accountID)
	if err != nil {
		return nil, membershipError("find active caregiver by account ID", err)
	}
	if len(caregivers) == 0 {
		return nil, nil
	}
	return caregivers[0], nil
}
