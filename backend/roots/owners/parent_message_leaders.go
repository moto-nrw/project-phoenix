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

// ParentMessageLeadership binds Communication's leadership directory (#5200):
// the staff whose account holds parent_messages:leadership through a role or
// admin:*. They read the conversations parents address to the leadership.
type ParentMessageLeadership struct {
	Staff permissionStaff
}

// ParentMessageLeaders lists the leadership persons, ordered by name.
func (l ParentMessageLeadership) ParentMessageLeaders(ctx context.Context) ([]communication.ParentMessageLeader, error) {
	rows, err := l.Staff.ListStaffWithPermission(ctx, permissions.ParentMessagesLeadership)
	if err != nil {
		return nil, err
	}
	leaders := make([]communication.ParentMessageLeader, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row == nil || row.AccountID <= 0 || seen[row.AccountID] {
			continue
		}
		seen[row.AccountID] = true
		leaders = append(leaders, communication.ParentMessageLeader{AccountID: row.AccountID, FirstName: row.FirstName, LastName: row.LastName})
	}
	sort.SliceStable(leaders, func(i, j int) bool {
		a, b := leaders[i], leaders[j]
		if a.FirstName != b.FirstName {
			return a.FirstName < b.FirstName
		}
		if a.LastName != b.LastName {
			return a.LastName < b.LastName
		}
		return a.AccountID < b.AccountID
	})
	return leaders, nil
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
