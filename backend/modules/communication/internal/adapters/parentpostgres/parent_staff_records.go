package parentpostgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	recordports "github.com/moto-nrw/project-phoenix/modules/communication/ports"
)

// WithInboxGroupNames names the group of every inbox row through groups.
func WithInboxGroupNames(reads recordports.ParentMessageReadRepository, groups recordports.GroupNames) recordports.ParentMessageReadRepository {
	return groupParentMessageReadRepository{ParentMessageReadRepository: reads, groups: groups}
}

// CurrentTenantStaffAccounts is the flat account set of the live staff
// visible in the caller's transaction. The staff-messaging surface is
// tenant-scoped, so every account belongs to the same school.
func CurrentTenantStaffAccounts(members recordports.StaffMembers, persons recordports.StaffPersons) func(context.Context) ([]int64, error) {
	return func(ctx context.Context) ([]int64, error) {
		byTenant, err := staffAccountsByTenant(ctx, members, persons, nil)
		if err != nil {
			return nil, err
		}
		accounts := make([]int64, 0, len(byTenant))
		for _, ids := range byTenant {
			accounts = append(accounts, ids...)
		}
		return accounts, nil
	}
}

// staffAccountsByTenant maps every tenant visible in the caller's transaction
// to the login accounts of its live staff. Under a tenant transaction that is
// one school; under an admin transaction (the cross-tenant guardian inbox) it
// is every school the caller may see, unless tenantIDs bounds the lookup to
// the schools the caller already knows — the guardian inbox does, so the
// query never scans the whole platform's staff.
func staffAccountsByTenant(ctx context.Context, listMembers recordports.StaffMembers, persons recordports.StaffPersons, tenantIDs []int64) (map[int64][]int64, error) {
	members, err := listMembers(ctx, tenantIDs)
	if err != nil {
		return nil, fmt.Errorf("list staff for message projection: %w", err)
	}
	if len(members) == 0 {
		return map[int64][]int64{}, nil
	}
	personIDs := make([]int64, 0, len(members))
	for _, member := range members {
		personIDs = append(personIDs, member.PersonID)
	}
	rows, err := persons(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("load staff persons for message projection: %w", err)
	}
	result := make(map[int64][]int64)
	seen := make(map[[2]int64]bool, len(members))
	for _, member := range members {
		person, found := rows[member.PersonID]
		if !found || person.AccountID == nil {
			continue
		}
		key := [2]int64{member.TenantID, *person.AccountID}
		if seen[key] {
			continue
		}
		seen[key] = true
		result[member.TenantID] = append(result[member.TenantID], *person.AccountID)
	}
	return result, nil
}

type parentMessageStaffRepository struct {
	recordports.ParentStaffMessageReads
	members recordports.StaffMembers
	persons recordports.StaffPersons
}

var _ recordports.ParentMessageReadRepository = parentMessageStaffRepository{}

func (r parentMessageStaffRepository) staffAccounts(ctx context.Context) (map[int64][]int64, error) {
	return staffAccountsByTenant(ctx, r.members, r.persons, nil)
}
func (r parentMessageStaffRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	staffAccounts, err := r.staffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return r.ListThreadsForGuardianStudentWithStaff(ctx, accountID, studentID, staffAccounts)
}
func (r parentMessageStaffRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	if len(tenantIDs) == 0 {
		return []*communication.PortalInboxThread{}, nil
	}
	staffAccounts, err := staffAccountsByTenant(ctx, r.members, r.persons, tenantIDs)
	if err != nil {
		return nil, err
	}
	return r.ListThreadsForGuardianTenantsWithStaff(ctx, accountID, tenantIDs, staffAccounts)
}
func (r parentMessageStaffRepository) LatestReadCursorByOther(ctx context.Context, threadID, excludeAccountID int64) (*communication.ParentReadCursorRecord, error) {
	staffAccounts, err := r.staffAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return r.LatestReadCursorByOtherStaff(ctx, threadID, excludeAccountID, staffAccounts)
}

type groupParentMessageReadRepository struct {
	recordports.ParentMessageReadRepository
	groups recordports.GroupNames
}

func (r groupParentMessageReadRepository) ListInboxForStaff(ctx context.Context, accountID int64, reach communication.ParentMessageStaffReach, onlyUnread bool) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListInboxForStaff(ctx, accountID, reach, onlyUnread)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}
func (r groupParentMessageReadRepository) ListThreadsForStudent(ctx context.Context, accountID, studentID int64, reach communication.ParentMessageStaffReach) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForStudent(ctx, accountID, studentID, reach)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}
func (r groupParentMessageReadRepository) ListThreadsForGuardianStudent(ctx context.Context, accountID, studentID int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianStudent(ctx, accountID, studentID)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}
func (r groupParentMessageReadRepository) ListThreadsForGuardianTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*communication.PortalInboxThread, error) {
	rows, err := r.ParentMessageReadRepository.ListThreadsForGuardianTenants(ctx, accountID, tenantIDs)
	return enrichInboxGroups(ctx, r.groups, rows, err)
}

// enrichInboxGroups fills each row's group name with one batched lookup. A
// group the caller cannot see leaves the name empty, mirroring the former
// LEFT JOIN; a lookup failure is returned, never swallowed.
func enrichInboxGroups(ctx context.Context, groupNames recordports.GroupNames, rows []*communication.PortalInboxThread, queryErr error) ([]*communication.PortalInboxThread, error) {
	if queryErr != nil {
		return nil, queryErr
	}
	if len(rows) == 0 {
		return rows, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, inboxGroupID(row))
	}
	names, err := groupNames(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("load groups for parent message inbox: %w", err)
	}
	for _, row := range rows {
		row.GroupName = names[inboxGroupID(row)]
	}
	return rows, nil
}
func inboxGroupID(row *communication.PortalInboxThread) int64 {
	if row.GroupID == nil {
		return 0
	}
	return *row.GroupID
}
func NewStaffParentMessageRecords(reads recordports.ParentStaffMessageReads, members recordports.StaffMembers, persons recordports.StaffPersons) recordports.ParentMessageReadRepository {
	return parentMessageStaffRepository{ParentStaffMessageReads: reads, members: members, persons: persons}
}
