package owners

import (
	"context"
	"sort"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	peopleValues "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// permissionStaff lists the live staff of the request's school whose active
// account holds a permission, with the shared wildcard rules (admin:*).
type permissionStaff interface {
	ListStaffWithPermission(ctx context.Context, permissionName string) ([]*peopleValues.StaffWithRoleInfo, error)
}

// effectiveAdmins lists the active accounts of the request's school with
// admin authority (an admin role, admin:* or *:*), with or without a staff
// record.
type effectiveAdmins interface {
	ListEffectiveAdminAccountIDs(ctx context.Context) ([]int64, error)
}

// ParentMessageLeadership binds Communication's leadership directory (#5200)
// to exactly the accounts that read the leadership conversations: live staff
// whose account holds both parent_messages:leadership and users:read through
// roles (admin:* included), and every school administrator account, which reads
// every child through admin:* even without a staff record. Such an account carries no
// name; the APIs count it as unnamed_count and the clients name it.
type ParentMessageLeadership struct {
	Staff  permissionStaff
	Admins effectiveAdmins
}

// ParentMessageLeaders lists the leadership persons, named ones by name,
// then the unnamed administrator accounts by id.
func (l ParentMessageLeadership) ParentMessageLeaders(ctx context.Context) ([]communication.ParentMessageLeader, error) {
	rows, err := l.Staff.ListStaffWithPermission(ctx, permissions.ParentMessagesLeadership)
	if err != nil {
		return nil, err
	}
	readers, err := l.Staff.ListStaffWithPermission(ctx, permissions.UsersRead)
	if err != nil {
		return nil, err
	}
	canRead := make(map[int64]bool, len(readers))
	for _, reader := range readers {
		if reader != nil {
			canRead[reader.AccountID] = true
		}
	}
	adminIDs, err := l.Admins.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	leaders := make([]communication.ParentMessageLeader, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row == nil || row.AccountID <= 0 || !canRead[row.AccountID] || seen[row.AccountID] {
			continue
		}
		seen[row.AccountID] = true
		leaders = append(leaders, communication.ParentMessageLeader{AccountID: row.AccountID, FirstName: row.FirstName, LastName: row.LastName})
	}
	for _, accountID := range adminIDs {
		if accountID > 0 && !seen[accountID] {
			seen[accountID] = true
			leaders = append(leaders, communication.ParentMessageLeader{AccountID: accountID})
		}
	}
	sort.SliceStable(leaders, func(i, j int) bool { return leaderBefore(leaders[i], leaders[j]) })
	return leaders, nil
}

// leaderBefore orders named persons by name before unnamed accounts by id.
func leaderBefore(a, b communication.ParentMessageLeader) bool {
	if (a.ShortName() == "") != (b.ShortName() == "") {
		return a.ShortName() != ""
	}
	if a.FirstName != b.FirstName {
		return a.FirstName < b.FirstName
	}
	if a.LastName != b.LastName {
		return a.LastName < b.LastName
	}
	return a.AccountID < b.AccountID
}

// LeadershipAccountIDs lists the leadership persons' accounts for Delivery's
// staff push on a leadership conversation.
func (l ParentMessageLeadership) LeadershipAccountIDs(ctx context.Context) ([]int64, error) {
	leaders, err := l.ParentMessageLeaders(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(leaders))
	for _, leader := range leaders {
		ids = append(ids, leader.AccountID)
	}
	return ids, nil
}

// ParentMessagePeopleWithLeaders is the staff side's People Directory slice
// together with the leadership persons (#5200).
type ParentMessagePeopleWithLeaders struct {
	communication.ParentMessagePeople
	ParentMessageLeadership
}
