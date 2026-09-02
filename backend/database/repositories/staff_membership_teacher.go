package repositories

import (
	"context"
	"errors"
	"fmt"
	"sort"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

// teacherMembershipRepository serves users.TeacherRepository over the School
// Membership capability.
type teacherMembershipRepository struct {
	membership schoolmembership.Capability
	deps       *staffMembershipDeps
}

var _ userModels.TeacherRepository = teacherMembershipRepository{}

func teacherFieldsFromLegacy(teacher *userModels.Teacher) schoolmembership.TeacherFields {
	return schoolmembership.TeacherFields{
		StaffID:        teacher.StaffID,
		Specialization: teacher.Specialization,
		Role:           teacher.Role,
		Qualifications: teacher.Qualifications,
	}
}

func applyTeacherToLegacy(target *userModels.Teacher, value schoolmembership.Teacher) {
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

func toLegacyTeacher(value schoolmembership.Teacher) *userModels.Teacher {
	teacher := new(userModels.Teacher)
	applyTeacherToLegacy(teacher, value)
	return teacher
}

func toLegacyTeacherList(values []schoolmembership.Teacher) []*userModels.Teacher {
	result := make([]*userModels.Teacher, 0, len(values))
	for _, value := range values {
		result = append(result, toLegacyTeacher(value))
	}
	return result
}

// --- CRUD ---

func (r teacherMembershipRepository) Create(ctx context.Context, entity *userModels.Teacher) error {
	if entity == nil {
		return usersRepo.WrapError("create teacher", errors.New("teacher cannot be nil"))
	}
	created, err := r.membership.CreateTeacher(ctx, schoolmembership.CreateTeacher{TeacherFields: teacherFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("create teacher", err)
	}
	applyTeacherToLegacy(entity, created)
	return nil
}

func (r teacherMembershipRepository) Update(ctx context.Context, entity *userModels.Teacher) error {
	if entity == nil {
		return usersRepo.WrapError("update teacher", errors.New("teacher cannot be nil"))
	}
	updated, err := r.membership.UpdateTeacher(ctx, schoolmembership.UpdateTeacher{ID: entity.ID, TeacherFields: teacherFieldsFromLegacy(entity)})
	if err != nil {
		return membershipError("update teacher", err)
	}
	applyTeacherToLegacy(entity, updated)
	return nil
}

func (r teacherMembershipRepository) Delete(ctx context.Context, id any) error {
	teacherID, err := membershipID(id)
	if err != nil {
		return usersRepo.WrapError("delete teacher", err)
	}
	return membershipError("delete teacher", r.membership.DeleteTeacher(ctx, teacherID))
}

func (r teacherMembershipRepository) FindByID(ctx context.Context, id any) (*userModels.Teacher, error) {
	teacherID, err := membershipID(id)
	if err != nil {
		return nil, usersRepo.WrapError("find teacher by id", err)
	}
	value, err := r.membership.FindTeacher(ctx, teacherID)
	if err != nil {
		return nil, membershipError("find teacher by id", err)
	}
	return toLegacyTeacher(value), nil
}

// FindByStaffID returns (nil, nil) when the staff member is not a teacher —
// the legacy contract every caller branches on.
func (r teacherMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	value, err := r.membership.FindTeacherByStaff(ctx, staffID)
	if err != nil {
		if errors.Is(err, schoolmembership.ErrTeacherNotFound) {
			return nil, nil
		}
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyTeacher(value), nil
}

func (r teacherMembershipRepository) FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	if len(staffIDs) == 0 {
		return make(map[int64]*userModels.Teacher), nil
	}
	values, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, membershipError("find by staff IDs", err)
	}
	result := make(map[int64]*userModels.Teacher, len(values))
	for _, teacher := range toLegacyTeacherList(values) {
		result[teacher.StaffID] = teacher
	}
	return result, nil
}

func (r teacherMembershipRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
	values, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{Specialization: specialization})
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
		switch field {
		case "id":
			id, err := membershipID(value)
			if err != nil {
				return filter, err
			}
			filter.IDs = append(filter.IDs, id)
		case "staff_id":
			id, err := membershipID(value)
			if err != nil {
				return filter, err
			}
			filter.StaffIDs = append(filter.StaffIDs, id)
		case "specialization":
			text, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("teacher filter %q must be a string", field)
			}
			filter.Specialization = text
		case "specialization_like":
			text, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("teacher filter %q must be a string", field)
			}
			filter.SpecializationContains = text
		case "role_like":
			text, ok := value.(string)
			if !ok {
				return filter, fmt.Errorf("teacher filter %q must be a string", field)
			}
			filter.RoleContains = text
		case "has_qualifications":
			has, ok := value.(bool)
			if !ok {
				return filter, fmt.Errorf("teacher filter %q must be a bool", field)
			}
			filter.HasQualifications = &has
		default:
			return filter, fmt.Errorf("unsupported teacher filter %q", field)
		}
	}
	return filter, nil
}

func (r teacherMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*userModels.Teacher, error) {
	filter, err := teacherFilterFromLegacy(filters)
	if err != nil {
		return nil, usersRepo.WrapError("list teachers", err)
	}
	values, err := r.membership.ListTeachers(ctx, filter)
	if err != nil {
		return nil, membershipError("list teachers", err)
	}
	return toLegacyTeacherList(values), nil
}

// FindByGroupID resolves the group's teacher assignments through the education
// owner and then loads those teachers from the membership owner.
func (r teacherMembershipRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Teacher, error) {
	const op = "find by group ID"
	assignments, err := r.deps.groupTeachers().FindByGroup(ctx, groupID)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	if len(assignments) == 0 {
		return []*userModels.Teacher{}, nil
	}
	ids := make([]int64, 0, len(assignments))
	for _, assignment := range assignments {
		ids = append(ids, assignment.TeacherID)
	}
	values, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, membershipError(op, err)
	}
	return toLegacyTeacherList(values), nil
}

// --- staff + person compositions ---

// hydrateTeacherStaff attaches the staff member and their person to every
// teacher. Staff rows are loaded WITH tombstones because the legacy join had
// no deleted_at predicate on users.staff.
func (r teacherMembershipRepository) hydrateTeacherStaff(ctx context.Context, teachers []*userModels.Teacher, liveStaffOnly bool) ([]*userModels.Teacher, error) {
	if len(teachers) == 0 {
		return teachers, nil
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffValues, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: !liveStaffOnly})
	if err != nil {
		return nil, err
	}
	staffByID := make(map[int64]*userModels.Staff, len(staffValues))
	personIDs := make([]int64, 0, len(staffValues))
	for _, value := range staffValues {
		staffByID[value.ID] = toLegacyStaff(value)
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := r.deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	// The legacy query INNER JOINed staff and persons, so a teacher without
	// either simply did not appear.
	result := make([]*userModels.Teacher, 0, len(teachers))
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

func (r teacherMembershipRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	const op = "find with staff and person"
	value, err := r.membership.FindTeacher(ctx, id)
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, []*userModels.Teacher{toLegacyTeacher(value)}, false)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	if len(teachers) == 0 {
		return nil, membershipNotFound(op)
	}
	return teachers[0], nil
}

func (r teacherMembershipRepository) ListAllWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
	const op = "list all with staff and person"
	values, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, toLegacyTeacherList(values), true)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	return teachers, nil
}

func (r teacherMembershipRepository) FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*userModels.Teacher, error) {
	const op = "find with staff and person by IDs"
	if len(ids) == 0 {
		return []*userModels.Teacher{}, nil
	}
	values, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{IDs: ids})
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, toLegacyTeacherList(values), false)
	if err != nil {
		return nil, usersRepo.WrapError(op, err)
	}
	// FindWithStaffAndPersonByIDs historically did not project the teacher's
	// tenant_id (Teacher.TenantID serializes as "tenant_id", so projecting it
	// would be a wire change).
	for _, teacher := range teachers {
		teacher.SetTenantID(0)
	}
	return teachers, nil
}

// --- caregivers ---

// caregiverRoleNames are the system roles that make a teacher an operational
// caregiver.
var caregiverRoleNames = []string{"user", "teacher"}

// activeCaregivers is the canonical caregiver composition: live teachers of the
// tenant whose staff member has a person with an active account, an active
// tenant mapping, and one of the system caregiver roles at that tenant.
func (r teacherMembershipRepository) activeCaregivers(ctx context.Context, accountID *int64) ([]*userModels.ActiveCaregiver, error) {
	teachers, err := r.membership.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	if err != nil {
		return nil, err
	}
	if len(teachers) == 0 {
		return []*userModels.ActiveCaregiver{}, nil
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffValues, err := r.membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, err
	}
	staffByID := make(map[int64]schoolmembership.Staff, len(staffValues))
	personIDs := make([]int64, 0, len(staffValues))
	for _, value := range staffValues {
		staffByID[value.ID] = value
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := r.deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	tenantID := usersRepo.TenantIDFromContext(ctx)
	candidates := make([]*userModels.ActiveCaregiver, 0, len(teachers))
	accountIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staff, found := staffByID[teacher.StaffID]
		if !found {
			continue
		}
		if tenantID > 0 && (teacher.TenantID != tenantID || staff.TenantID != tenantID) {
			continue
		}
		person, found := persons[staff.PersonID]
		if !found || person.AccountID == nil {
			continue
		}
		if tenantID > 0 && person.GetTenantID() != tenantID {
			continue
		}
		if accountID != nil && *person.AccountID != *accountID {
			continue
		}
		candidates = append(candidates, &userModels.ActiveCaregiver{
			AccountID: *person.AccountID,
			PersonID:  person.ID,
			StaffID:   staff.ID,
			TeacherID: teacher.ID,
			FirstName: person.FirstName,
			LastName:  person.LastName,
			CreatedAt: staff.CreatedAt,
			UpdatedAt: staff.UpdatedAt,
		})
		accountIDs = append(accountIDs, *person.AccountID)
	}
	if len(candidates) == 0 {
		return []*userModels.ActiveCaregiver{}, nil
	}

	allowed, err := r.deps.memberships.ListActiveAccountIDsForTenant(ctx, tenantID, accountIDs)
	if err != nil {
		return nil, err
	}
	active := make(map[int64]bool, len(allowed))
	for _, id := range allowed {
		active[id] = true
	}
	withRole, err := r.deps.roles.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, caregiverRoleNames, tenantID)
	if err != nil {
		return nil, err
	}
	caregiverAccounts := make(map[int64]bool, len(withRole))
	for _, id := range withRole {
		caregiverAccounts[id] = true
	}
	emails, err := r.deps.accounts.FindEmailsByAccountIDs(ctx, allowed)
	if err != nil {
		return nil, err
	}

	result := make([]*userModels.ActiveCaregiver, 0, len(candidates))
	for _, candidate := range candidates {
		if !active[candidate.AccountID] || !caregiverAccounts[candidate.AccountID] {
			continue
		}
		candidate.Email = emails[candidate.AccountID]
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].FirstName != result[j].FirstName {
			return result[i].FirstName < result[j].FirstName
		}
		if result[i].LastName != result[j].LastName {
			return result[i].LastName < result[j].LastName
		}
		return result[i].StaffID < result[j].StaffID
	})
	return result, nil
}

func (r teacherMembershipRepository) ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, nil)
	if err != nil {
		return nil, membershipError("list active caregivers", err)
	}
	return caregivers, nil
}

func (r teacherMembershipRepository) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, &accountID)
	if err != nil {
		return nil, membershipError("find active caregiver by account ID", err)
	}
	if len(caregivers) == 0 {
		return nil, nil
	}
	return caregivers[0], nil
}
