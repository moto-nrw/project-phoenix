package repositories

import (
	"context"
	"fmt"
	"slices"

	authRepo "github.com/moto-nrw/project-phoenix/database/repositories/auth"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

type accountTenantSchoolRows interface {
	ListAccountsBySchoolIDs(context.Context, []int64) ([]authModels.OrgAccountInfo, error)
}

type schoolAccountTenantRepository struct {
	authModels.AccountTenantRepository
	raw     accountTenantSchoolRows
	schools organizationtenancy.Query
}

func (r schoolAccountTenantRepository) ListTenantAccessByAccountID(ctx context.Context, accountID int64) ([]authModels.AccountTenantAccessInfo, error) {
	rows, err := r.AccountTenantRepository.ListTenantAccessByAccountID(ctx, accountID)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TenantID)
	}
	schools, err := schoolsByID(ctx, r.schools, ids)
	if err != nil {
		return nil, fmt.Errorf("load schools for account access: %w", err)
	}
	result := make([]authModels.AccountTenantAccessInfo, 0, len(rows))
	for _, row := range rows {
		school, found := schools[row.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for account access", row.TenantID)
		}
		if school.IsDeleted() {
			continue
		}
		row.SchoolName = school.Name
		row.SchoolSlug = school.Slug
		row.SchoolActive = school.Active
		row.OrganizationID = school.OrganizationID
		result = append(result, row)
	}
	slices.SortStableFunc(result, func(left, right authModels.AccountTenantAccessInfo) int {
		return compareStrings(left.SchoolName, right.SchoolName)
	})
	return result, nil
}

func (r schoolAccountTenantRepository) ListAccountsByOrganizationID(ctx context.Context, organizationID int64) ([]authModels.OrgAccountInfo, error) {
	schools, err := r.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("load organization schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, activeSchoolIDs(schools, false))
}

func (r schoolAccountTenantRepository) ListAllAccounts(ctx context.Context) ([]authModels.OrgAccountInfo, error) {
	schools, err := r.schools.ListNonDeletedSchools(ctx)
	if err != nil {
		return nil, fmt.Errorf("load schools for accounts: %w", err)
	}
	return r.listAccounts(ctx, activeSchoolIDs(schools, false))
}

func (r schoolAccountTenantRepository) listAccounts(ctx context.Context, schoolIDs []int64) ([]authModels.OrgAccountInfo, error) {
	rows, err := r.raw.ListAccountsBySchoolIDs(ctx, schoolIDs)
	if err != nil || len(rows) == 0 {
		return rows, err
	}
	schools, err := schoolsByID(ctx, r.schools, schoolIDs)
	if err != nil {
		return nil, fmt.Errorf("load school names for accounts: %w", err)
	}
	for index := range rows {
		school, found := schools[rows[index].SchoolID]
		if !found {
			return nil, fmt.Errorf("school %d missing for account row", rows[index].SchoolID)
		}
		rows[index].SchoolName = school.Name
	}
	slices.SortStableFunc(rows, func(left, right authModels.OrgAccountInfo) int {
		if order := compareStrings(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		if order := compareStrings(left.LastName, right.LastName); order != 0 {
			return order
		}
		return compareStrings(left.FirstName, right.FirstName)
	})
	return rows, nil
}

type schoolAccountRepository struct {
	authModels.AccountRepository
	schools organizationtenancy.Query
}

func (r schoolAccountRepository) FindManageableByID(ctx context.Context, id int64) (*authModels.Account, error) {
	ctx, err := r.withSchoolScope(ctx)
	if err != nil {
		return nil, err
	}
	return r.AccountRepository.FindManageableByID(ctx, id)
}

func (r schoolAccountRepository) ListManageable(ctx context.Context, filters map[string]interface{}) ([]*authModels.Account, error) {
	ctx, err := r.withSchoolScope(ctx)
	if err != nil {
		return nil, err
	}
	return r.AccountRepository.ListManageable(ctx, filters)
}

func (r schoolAccountRepository) FindByRole(ctx context.Context, role string) ([]*authModels.Account, error) {
	ctx, err := r.withSchoolScope(ctx)
	if err != nil {
		return nil, err
	}
	return r.AccountRepository.FindByRole(ctx, role)
}

func (r schoolAccountRepository) UpdateManageable(ctx context.Context, account *authModels.Account) error {
	ctx, err := r.withSchoolScope(ctx)
	if err != nil {
		return err
	}
	return r.AccountRepository.UpdateManageable(ctx, account)
}

func (r schoolAccountRepository) FindAccountsWithRolesAndPermissions(ctx context.Context, filters map[string]interface{}) ([]*authModels.Account, error) {
	ctx, err := r.withSchoolScope(ctx)
	if err != nil {
		return nil, err
	}
	return r.AccountRepository.FindAccountsWithRolesAndPermissions(ctx, filters)
}

func (r schoolAccountRepository) withSchoolScope(ctx context.Context) (context.Context, error) {
	organizationID, scoped := authRepo.OrganizationScope(ctx)
	if !scoped {
		return ctx, nil
	}
	schools, err := r.schools.ListSchoolsByOrganization(ctx, organizationID)
	if err != nil {
		return nil, fmt.Errorf("load manageable organization schools: %w", err)
	}
	return authRepo.WithManageableSchoolIDs(ctx, activeSchoolIDs(schools, true)), nil
}

func activeSchoolIDs(schools []organizationtenancy.School, activeOnly bool) []int64 {
	ids := make([]int64, 0, len(schools))
	for _, school := range schools {
		if school.IsDeleted() || (activeOnly && !school.Active) {
			continue
		}
		ids = append(ids, school.ID)
	}
	return ids
}

type schoolChildRepository struct {
	parentModels.ChildRepository
	schools organizationtenancy.Query
}

func (r schoolChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	children, err := r.ChildRepository.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := enrichChildrenWithSchools(ctx, r.schools, children); err != nil {
		return nil, err
	}
	slices.SortStableFunc(children, func(left, right *parentModels.ChildSummary) int {
		if order := compareStrings(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		if order := compareStrings(left.FirstName, right.FirstName); order != 0 {
			return order
		}
		return compareStrings(left.LastName, right.LastName)
	})
	return children, nil
}

func (r schoolChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	child, err := r.ChildRepository.FindForAccount(ctx, accountID, studentID)
	if err != nil || child == nil {
		return child, err
	}
	if err := enrichChildrenWithSchools(ctx, r.schools, []*parentModels.ChildSummary{child}); err != nil {
		return nil, err
	}
	return child, nil
}

type schoolEnrollablePhaseRepository struct {
	parentModels.EnrollablePhaseRepository
	schools organizationtenancy.Query
}

func (r schoolEnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	phases, err := r.EnrollablePhaseRepository.ListEnrollable(ctx, accountID)
	if err != nil || len(phases) == 0 {
		return phases, err
	}
	schools, err := schoolsByID(ctx, r.schools, phaseSchoolIDs(phases))
	if err != nil {
		return nil, fmt.Errorf("parent: load schools for enrollable phases: %w", err)
	}
	result := make([]*parentModels.EnrollablePhase, 0, len(phases))
	for _, phase := range phases {
		school, found := schools[phase.SchoolID]
		if !found {
			return nil, fmt.Errorf("parent: school %d missing for enrollable phase", phase.SchoolID)
		}
		if !school.Active || school.IsDeleted() || (school.Hidden && !phase.HasFamilyLink) {
			continue
		}
		phase.SchoolName = school.Name
		phase.SchoolSlug = school.Slug
		phase.SchoolSubdomain = school.Subdomain
		result = append(result, phase)
	}
	slices.SortStableFunc(result, func(left, right *parentModels.EnrollablePhase) int {
		if left.AlreadyLinked != right.AlreadyLinked {
			if left.AlreadyLinked {
				return -1
			}
			return 1
		}
		if order := compareStrings(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		return left.ServiceStartDate.Compare(right.ServiceStartDate)
	})
	return result, nil
}

type schoolEnrollmentRequestRepository struct {
	parentModels.EnrollmentRequestRepository
	schools organizationtenancy.Query
}

type schoolParentAnnouncementRepository struct {
	usersModels.ParentAnnouncementRepository
	schools organizationtenancy.Query
}

func (r schoolParentAnnouncementRepository) SchoolName(ctx context.Context, tenantID int64) (string, error) {
	school, err := r.schools.FindSchool(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("resolve school name: %w", err)
	}
	return school.Name, nil
}

func (r schoolParentAnnouncementRepository) ListFeedForAccount(ctx context.Context, accountID int64, scope usersModels.AnnouncementFeedScope) ([]*usersModels.AnnouncementFeedItem, error) {
	items, err := r.ParentAnnouncementRepository.ListFeedForAccount(ctx, accountID, scope)
	if err != nil || len(items) == 0 {
		return items, err
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.TenantID)
	}
	schools, err := schoolsByID(ctx, r.schools, ids)
	if err != nil {
		return nil, fmt.Errorf("load schools for parent announcement feed: %w", err)
	}
	for _, item := range items {
		school, found := schools[item.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for parent announcement feed", item.TenantID)
		}
		item.SchoolName = school.Name
	}
	return items, nil
}

type schoolParentMessageReadRepository struct {
	usersModels.ParentMessageReadRepository
	schools organizationtenancy.Query
}

func (r schoolParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, allStudents, onlyUnread bool) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListInboxForStaff(ctx, accountID, allStudents, onlyUnread)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForStudent(ctx, accountID, studentID)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func (r schoolParentMessageReadRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*usersModels.InboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	return enrichInboxSchools(ctx, r.schools, rows, err)
}

func enrichInboxSchools(ctx context.Context, query organizationtenancy.Query, rows []*usersModels.InboxThread, queryErr error) ([]*usersModels.InboxThread, error) {
	if queryErr != nil || len(rows) == 0 {
		return rows, queryErr
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.TenantID)
	}
	schools, err := schoolsByID(ctx, query, ids)
	if err != nil {
		return nil, fmt.Errorf("load schools for parent message inbox: %w", err)
	}
	for _, row := range rows {
		school, found := schools[row.TenantID]
		if !found {
			return nil, fmt.Errorf("school %d missing for parent message inbox", row.TenantID)
		}
		row.SchoolName = school.Name
	}
	return rows, nil
}

func (r schoolEnrollmentRequestRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	requests, err := r.EnrollmentRequestRepository.ListByAccount(ctx, accountID)
	if err != nil || len(requests) == 0 {
		return requests, err
	}
	ids := make([]int64, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, request.TenantID)
	}
	schools, err := schoolsByID(ctx, r.schools, ids)
	if err != nil {
		return nil, fmt.Errorf("parent: load schools for enrollment requests: %w", err)
	}
	result := make([]*parentModels.EnrollmentRequestSummary, 0, len(requests))
	for _, request := range requests {
		school, found := schools[request.TenantID]
		if !found {
			return nil, fmt.Errorf("parent: school %d missing for enrollment request", request.TenantID)
		}
		if school.IsDeleted() {
			continue
		}
		request.SchoolName = school.Name
		request.SchoolSlug = school.Slug
		result = append(result, request)
	}
	return result, nil
}

func enrichChildrenWithSchools(ctx context.Context, query organizationtenancy.Query, children []*parentModels.ChildSummary) error {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.TenantID)
	}
	schools, err := schoolsByID(ctx, query, ids)
	if err != nil {
		return fmt.Errorf("parent: load schools for children: %w", err)
	}
	for _, child := range children {
		school, found := schools[child.TenantID]
		if !found {
			return fmt.Errorf("parent: school %d missing for child", child.TenantID)
		}
		child.SchoolName = school.Name
		child.SchoolSlug = school.Slug
	}
	return nil
}

func phaseSchoolIDs(phases []*parentModels.EnrollablePhase) []int64 {
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.SchoolID)
	}
	return ids
}

func schoolsByID(ctx context.Context, query organizationtenancy.Query, ids []int64) (map[int64]organizationtenancy.School, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	values, err := query.ListSchoolsByID(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]organizationtenancy.School, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}

func compareStrings(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}
