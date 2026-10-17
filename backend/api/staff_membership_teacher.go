package api

import (
	"context"
	"errors"
	"fmt"
	"sort"

	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

// TeacherMembershipRepository serves users.TeacherRepository over the School
// Membership capability.
type TeacherMembershipRepository struct {
	Membership schoolmembership.Capability
	Deps       *staffMembershipDeps
}

var _ peoplePorts.TeacherRepository = TeacherMembershipRepository{}

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

func (r TeacherMembershipRepository) Create(ctx context.Context, entity *userModels.Teacher) error {
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

func (r TeacherMembershipRepository) Update(ctx context.Context, entity *userModels.Teacher) error {
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
	teacherID, err := membershipID(id)
	if err != nil {
		return repositoryError("delete teacher", err)
	}
	return membershipError("delete teacher", r.Membership.DeleteTeacher(ctx, teacherID))
}

func (r TeacherMembershipRepository) FindByID(ctx context.Context, id any) (*userModels.Teacher, error) {
	teacherID, err := membershipID(id)
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
func (r TeacherMembershipRepository) FindByStaffID(ctx context.Context, staffID int64) (*userModels.Teacher, error) {
	value, err := r.Membership.FindTeacherByStaff(ctx, staffID)
	if err != nil {
		if errors.Is(err, schoolmembership.ErrTeacherNotFound) {
			return nil, nil
		}
		return nil, membershipError("find by staff ID", err)
	}
	return toLegacyTeacher(value), nil
}

func (r TeacherMembershipRepository) FindByStaffIDs(ctx context.Context, staffIDs []int64) (map[int64]*userModels.Teacher, error) {
	if len(staffIDs) == 0 {
		return make(map[int64]*userModels.Teacher), nil
	}
	values, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{StaffIDs: staffIDs})
	if err != nil {
		return nil, membershipError("find by staff IDs", err)
	}
	result := make(map[int64]*userModels.Teacher, len(values))
	for _, teacher := range toLegacyTeacherList(values) {
		result[teacher.StaffID] = teacher
	}
	return result, nil
}

func (r TeacherMembershipRepository) FindBySpecialization(ctx context.Context, specialization string) ([]*userModels.Teacher, error) {
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
		id, err := membershipID(value)
		if err == nil {
			filter.IDs = append(filter.IDs, id)
		}
		return err
	},
	"staff_id": func(filter *schoolmembership.TeacherFilter, _ string, value any) error {
		id, err := membershipID(value)
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

func (r TeacherMembershipRepository) List(ctx context.Context, filters map[string]any) ([]*userModels.Teacher, error) {
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
func (r TeacherMembershipRepository) FindByGroupID(ctx context.Context, groupID int64) ([]*userModels.Teacher, error) {
	const op = "find by group ID"
	assignments, err := r.Deps.GroupTeachers().FindByGroup(ctx, groupID)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	if len(assignments) == 0 {
		return []*userModels.Teacher{}, nil
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

// --- staff + person compositions ---

// hydrateTeacherStaff attaches the staff member and their person to every
// teacher. Staff rows are loaded WITH tombstones because the legacy join had
// no deleted_at predicate on users.staff.
func (r TeacherMembershipRepository) hydrateTeacherStaff(ctx context.Context, teachers []*userModels.Teacher, liveStaffOnly bool) ([]*userModels.Teacher, error) {
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
	staffByID := make(map[int64]*userModels.Staff, len(staffValues))
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

func (r TeacherMembershipRepository) FindWithStaffAndPerson(ctx context.Context, id int64) (*userModels.Teacher, error) {
	const op = "find with staff and person"
	value, err := r.Membership.FindTeacher(ctx, id)
	if err != nil {
		return nil, membershipError(op, err)
	}
	teachers, err := r.hydrateTeacherStaff(ctx, []*userModels.Teacher{toLegacyTeacher(value)}, false)
	if err != nil {
		return nil, repositoryError(op, err)
	}
	if len(teachers) == 0 {
		return nil, membershipNotFound(op)
	}
	return teachers[0], nil
}

func (r TeacherMembershipRepository) ListAllWithStaffAndPerson(ctx context.Context) ([]*userModels.Teacher, error) {
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

func (r TeacherMembershipRepository) FindWithStaffAndPersonByIDs(ctx context.Context, ids []int64) ([]*userModels.Teacher, error) {
	const op = "find with staff and person by IDs"
	if len(ids) == 0 {
		return []*userModels.Teacher{}, nil
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

// --- caregivers ---

// caregiverRoleNames are the system roles that make a teacher an operational
// caregiver.
var caregiverRoleNames = []string{"user", "teacher"}

type activeCaregiverCandidate struct {
	caregiver *userModels.ActiveCaregiver
	tenantID  int64
}

// activeCaregivers is the canonical caregiver composition: live teachers of the
// tenant whose staff member has a person with an active account, an active
// tenant mapping, and one of the system caregiver roles at that tenant.
func (r TeacherMembershipRepository) activeCaregivers(ctx context.Context, accountID *int64) ([]*userModels.ActiveCaregiver, error) {
	candidates, accountIDsByTenant, err := r.caregiverCandidates(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return []*userModels.ActiveCaregiver{}, nil
	}
	facts, err := r.caregiverAccountFacts(ctx, accountIDsByTenant)
	if err != nil {
		return nil, err
	}
	emails, err := r.Deps.identity.ListAccountEmails(ctx, facts.activeAccountIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*userModels.ActiveCaregiver, 0, len(candidates))
	for _, candidate := range candidates {
		if !facts.active[candidate.tenantID][candidate.caregiver.AccountID] || !facts.caregiver[candidate.tenantID][candidate.caregiver.AccountID] {
			continue
		}
		candidate.caregiver.Email = emails[candidate.caregiver.AccountID]
		result = append(result, candidate.caregiver)
	}
	sortActiveCaregivers(result)
	return result, nil
}

// caregiverCandidates lists the live teachers of the tenant with their staff
// member and an account-holding person, and the candidate accounts per
// tenant. accountID narrows the list to one account.
func (r TeacherMembershipRepository) caregiverCandidates(ctx context.Context, accountID *int64) ([]activeCaregiverCandidate, map[int64][]int64, error) {
	teachers, err := r.Membership.ListTeachers(ctx, schoolmembership.TeacherFilter{})
	if err != nil || len(teachers) == 0 {
		return nil, nil, err
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffValues, err := r.Membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, nil, err
	}
	staffByID := make(map[int64]schoolmembership.Staff, len(staffValues))
	personIDs := make([]int64, 0, len(staffValues))
	for _, value := range staffValues {
		staffByID[value.ID] = value
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := r.Deps.persons.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, err
	}

	tenantID := peopleCompose.CallerTenantID(ctx)
	candidates := make([]activeCaregiverCandidate, 0, len(teachers))
	accountIDsByTenant := make(map[int64][]int64)
	for _, teacher := range teachers {
		candidate, ok := caregiverCandidate(teacher, staffByID, persons, tenantID, accountID)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
		accountIDsByTenant[teacher.TenantID] = append(accountIDsByTenant[teacher.TenantID], candidate.caregiver.AccountID)
	}
	return candidates, accountIDsByTenant, nil
}

// caregiverCandidate joins one teacher to its staff member and person, or
// reports that the teacher is no candidate of the tenant (or the account).
func caregiverCandidate(teacher schoolmembership.Teacher, staffByID map[int64]schoolmembership.Staff, persons map[int64]*userModels.Person, tenantID int64, accountID *int64) (activeCaregiverCandidate, bool) {
	staff, found := staffByID[teacher.StaffID]
	if !found {
		return activeCaregiverCandidate{}, false
	}
	if tenantID > 0 && (teacher.TenantID != tenantID || staff.TenantID != tenantID) {
		return activeCaregiverCandidate{}, false
	}
	person, found := persons[staff.PersonID]
	if !found || person.AccountID == nil {
		return activeCaregiverCandidate{}, false
	}
	if tenantID > 0 && person.GetTenantID() != tenantID {
		return activeCaregiverCandidate{}, false
	}
	if accountID != nil && *person.AccountID != *accountID {
		return activeCaregiverCandidate{}, false
	}
	return activeCaregiverCandidate{caregiver: &userModels.ActiveCaregiver{
		AccountID: *person.AccountID,
		PersonID:  person.ID,
		StaffID:   staff.ID,
		TeacherID: teacher.ID,
		FirstName: person.FirstName,
		LastName:  person.LastName,
		CreatedAt: staff.CreatedAt,
		UpdatedAt: staff.UpdatedAt,
	}, tenantID: teacher.TenantID}, true
}

// caregiverAccounts are the account facts of the candidates per tenant: which
// accounts are active there and which hold a caregiver role there.
type caregiverAccounts struct {
	active           map[int64]map[int64]bool
	caregiver        map[int64]map[int64]bool
	activeAccountIDs []int64
}

func (r TeacherMembershipRepository) caregiverAccountFacts(ctx context.Context, accountIDsByTenant map[int64][]int64) (caregiverAccounts, error) {
	facts := caregiverAccounts{
		active:    make(map[int64]map[int64]bool, len(accountIDsByTenant)),
		caregiver: make(map[int64]map[int64]bool, len(accountIDsByTenant)),
	}
	for candidateTenantID, accountIDs := range accountIDsByTenant {
		allowed, err := r.Deps.identity.ListActiveAccountIDsForTenant(ctx, candidateTenantID, accountIDs)
		if err != nil {
			return caregiverAccounts{}, err
		}
		facts.active[candidateTenantID] = idSet(allowed)
		facts.activeAccountIDs = append(facts.activeAccountIDs, allowed...)

		withRole, err := r.Deps.identity.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, caregiverRoleNames, candidateTenantID)
		if err != nil {
			return caregiverAccounts{}, err
		}
		facts.caregiver[candidateTenantID] = idSet(withRole)
	}
	return facts, nil
}

func idSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// sortActiveCaregivers keeps the picker order of the replaced query: first
// name, last name, staff ID under the database collation (en_US.utf8). German
// dictionary order keeps that order: "Özlem" sorts next to "Otto", not after
// "Zimmermann". Same settings as internal/collation, which the architecture
// policy keeps out of this package; one collator per call, it is not safe for
// concurrent use.
func sortActiveCaregivers(result []*userModels.ActiveCaregiver) {
	collator := collate.New(language.German, collate.IgnoreCase)
	sort.SliceStable(result, func(i, j int) bool {
		if c := collator.CompareString(result[i].FirstName, result[j].FirstName); c != 0 {
			return c < 0
		}
		if c := collator.CompareString(result[i].LastName, result[j].LastName); c != 0 {
			return c < 0
		}
		return result[i].StaffID < result[j].StaffID
	})
}

func (r TeacherMembershipRepository) ListActiveCaregivers(ctx context.Context) ([]*userModels.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, nil)
	if err != nil {
		return nil, membershipError("list active caregivers", err)
	}
	return caregivers, nil
}

func (r TeacherMembershipRepository) FindActiveCaregiverByAccountID(ctx context.Context, accountID int64) (*userModels.ActiveCaregiver, error) {
	caregivers, err := r.activeCaregivers(ctx, &accountID)
	if err != nil {
		return nil, membershipError("find active caregiver by account ID", err)
	}
	if len(caregivers) == 0 {
		return nil, nil
	}
	return caregivers[0], nil
}
