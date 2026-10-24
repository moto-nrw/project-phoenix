package schoolmembership

import (
	"context"
	"errors"
	"time"
)

// caregiverRoleNames are the system roles that make a teacher an operational
// caregiver.
var caregiverRoleNames = []string{"user", "teacher"}

// ActiveCaregiver is an operational caregiver of a school: a live teacher
// whose staff member has a person with an account that is active at the
// school and holds a caregiver role there. Email is the account's address.
type ActiveCaregiver struct {
	AccountID int64
	PersonID  int64
	StaffID   int64
	TeacherID int64
	FirstName string
	LastName  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// CaregiverPerson is the People Directory half of a caregiver candidate.
type CaregiverPerson struct {
	ID        int64
	TenantID  int64
	AccountID *int64
	FirstName string
	LastName  string
}

// CaregiverDirectory reads the people and account facts a caregiver needs
// from their owners, the People Directory and Identity & Access.
type CaregiverDirectory interface {
	FindCaregiverPersons(ctx context.Context, personIDs []int64) (map[int64]CaregiverPerson, error)
	ListActiveAccountIDsForTenant(ctx context.Context, tenantID int64, accountIDs []int64) ([]int64, error)
	ListAccountIDsWithSystemRoleNames(ctx context.Context, accountIDs []int64, roleNames []string, tenantID int64) ([]int64, error)
	ListAccountEmails(ctx context.Context, accountIDs []int64) (map[int64]string, error)
}

// CaregiverMembership lists the teachers and staff members caregivers come
// from.
type CaregiverMembership interface {
	ListTeachers(context.Context, TeacherFilter) ([]Teacher, error)
	ListStaff(context.Context, StaffFilter) ([]Staff, error)
}

// ActiveCaregivers lists the active caregivers of one required school,
// optionally of one account only. The order is the caller's.
func ActiveCaregivers(ctx context.Context, membership CaregiverMembership, directory CaregiverDirectory, tenantID int64, accountID *int64) ([]ActiveCaregiver, error) {
	if tenantID <= 0 {
		return nil, errors.New("caregiver directory: tenant is required")
	}
	return activeCaregivers(ctx, membership, directory, caregiverScope{schoolID: tenantID}, accountID)
}

// CrossSchoolActiveCaregivers is the explicit cross-school projection. Roots
// bind it only while holding their privileged account/operator transaction.
func CrossSchoolActiveCaregivers(ctx context.Context, membership CaregiverMembership, directory CaregiverDirectory, accountID *int64) ([]ActiveCaregiver, error) {
	return activeCaregivers(ctx, membership, directory, caregiverScope{acrossSchools: true}, accountID)
}

type caregiverScope struct {
	schoolID      int64
	acrossSchools bool
}

func activeCaregivers(ctx context.Context, membership CaregiverMembership, directory CaregiverDirectory, scope caregiverScope, accountID *int64) ([]ActiveCaregiver, error) {
	candidates, accountIDsByTenant, err := caregiverCandidates(ctx, membership, directory, scope, accountID)
	if err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return []ActiveCaregiver{}, nil
	}
	facts, err := caregiverAccountFacts(ctx, directory, accountIDsByTenant)
	if err != nil {
		return nil, err
	}
	emails, err := directory.ListAccountEmails(ctx, facts.activeAccountIDs)
	if err != nil {
		return nil, err
	}
	result := make([]ActiveCaregiver, 0, len(candidates))
	for _, candidate := range candidates {
		if !facts.active[candidate.tenantID][candidate.caregiver.AccountID] || !facts.caregiver[candidate.tenantID][candidate.caregiver.AccountID] {
			continue
		}
		candidate.caregiver.Email = emails[candidate.caregiver.AccountID]
		result = append(result, candidate.caregiver)
	}
	return result, nil
}

type caregiverCandidate struct {
	caregiver ActiveCaregiver
	tenantID  int64
}

// caregiverCandidates lists the live teachers of the school with their staff
// member and an account-holding person, and the candidate accounts per
// school.
func caregiverCandidates(ctx context.Context, membership CaregiverMembership, directory CaregiverDirectory, scope caregiverScope, accountID *int64) ([]caregiverCandidate, map[int64][]int64, error) {
	teachers, err := membership.ListTeachers(ctx, TeacherFilter{})
	if err != nil || len(teachers) == 0 {
		return nil, nil, err
	}
	staffIDs := make([]int64, 0, len(teachers))
	for _, teacher := range teachers {
		staffIDs = append(staffIDs, teacher.StaffID)
	}
	staffValues, err := membership.ListStaff(ctx, StaffFilter{IDs: staffIDs, IncludeDeleted: true})
	if err != nil {
		return nil, nil, err
	}
	staffByID := make(map[int64]Staff, len(staffValues))
	personIDs := make([]int64, 0, len(staffValues))
	for _, value := range staffValues {
		staffByID[value.ID] = value
		personIDs = append(personIDs, value.PersonID)
	}
	persons, err := directory.FindCaregiverPersons(ctx, personIDs)
	if err != nil {
		return nil, nil, err
	}
	candidates := make([]caregiverCandidate, 0, len(teachers))
	accountIDsByTenant := make(map[int64][]int64)
	for _, teacher := range teachers {
		candidate, ok := teacherCaregiverCandidate(teacher, staffByID, persons, scope, accountID)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate)
		accountIDsByTenant[teacher.TenantID] = append(accountIDsByTenant[teacher.TenantID], candidate.caregiver.AccountID)
	}
	return candidates, accountIDsByTenant, nil
}

// teacherCaregiverCandidate joins one teacher to its staff member and
// person, or reports that the teacher is no candidate of the school (or the
// account).
func teacherCaregiverCandidate(teacher Teacher, staffByID map[int64]Staff, persons map[int64]CaregiverPerson, scope caregiverScope, accountID *int64) (caregiverCandidate, bool) {
	staff, found := staffByID[teacher.StaffID]
	// An external caregiver never takes over a group or a substitution, even
	// with an account in a caregiver role (#4573).
	if !found || staff.HasExternalCaregiverRecord {
		return caregiverCandidate{}, false
	}
	if teacher.TenantID != staff.TenantID || (!scope.acrossSchools && teacher.TenantID != scope.schoolID) {
		return caregiverCandidate{}, false
	}
	person, found := persons[staff.PersonID]
	if !found || person.AccountID == nil {
		return caregiverCandidate{}, false
	}
	if person.TenantID != teacher.TenantID {
		return caregiverCandidate{}, false
	}
	if accountID != nil && *person.AccountID != *accountID {
		return caregiverCandidate{}, false
	}
	return caregiverCandidate{caregiver: ActiveCaregiver{
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

// caregiverAccounts are the account facts of the candidates per school: which
// accounts are active there and which hold a caregiver role there.
type caregiverAccounts struct {
	active           map[int64]map[int64]bool
	caregiver        map[int64]map[int64]bool
	activeAccountIDs []int64
}

func caregiverAccountFacts(ctx context.Context, directory CaregiverDirectory, accountIDsByTenant map[int64][]int64) (caregiverAccounts, error) {
	facts := caregiverAccounts{
		active:    make(map[int64]map[int64]bool, len(accountIDsByTenant)),
		caregiver: make(map[int64]map[int64]bool, len(accountIDsByTenant)),
	}
	for candidateTenantID, accountIDs := range accountIDsByTenant {
		allowed, err := directory.ListActiveAccountIDsForTenant(ctx, candidateTenantID, accountIDs)
		if err != nil {
			return caregiverAccounts{}, err
		}
		facts.active[candidateTenantID] = idSet(allowed)
		facts.activeAccountIDs = append(facts.activeAccountIDs, allowed...)

		withRole, err := directory.ListAccountIDsWithSystemRoleNames(ctx, accountIDs, caregiverRoleNames, candidateTenantID)
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

// CaregiverStatus is what an account listing says about a person's caregiver
// capability: a teacher profile makes a caregiver profile, and it is active
// once the account also holds the user role at the school.
func CaregiverStatus(hasTeacher, hasUserRole bool) (hasProfile, active bool) {
	return hasTeacher, hasTeacher && hasUserRole
}
