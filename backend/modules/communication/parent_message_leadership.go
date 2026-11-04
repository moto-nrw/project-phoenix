package communication

import (
	"context"
	"sort"
)

// ParentMessageLeadershipStaff supplies active staff of the current school
// with each effective permission. Communication decides which accounts qualify
// as readers; the root only maps the directory's facts into these values.
type ParentMessageLeadershipStaff interface {
	StaffWithLeadershipPermission(context.Context) ([]ParentMessageLeader, error)
	StaffWithMessageReadPermission(context.Context) ([]ParentMessageLeader, error)
}

// ParentMessageLeadershipAdmins supplies active school administrator accounts,
// including accounts without a staff record.
type ParentMessageLeadershipAdmins interface {
	ListEffectiveAdminAccountIDs(context.Context) ([]int64, error)
}

// ParentMessageLeadership owns the reader eligibility shared by staff and
// parent conversations, the leadership option, and notification recipients.
type ParentMessageLeadership struct {
	staff  ParentMessageLeadershipStaff
	admins ParentMessageLeadershipAdmins
}

func NewParentMessageLeadership(staff ParentMessageLeadershipStaff, admins ParentMessageLeadershipAdmins) *ParentMessageLeadership {
	return &ParentMessageLeadership{staff: staff, admins: admins}
}

// ParentMessageLeaders requires both staff messaging permissions, adds school
// administrators, and returns each account once, named readers before unnamed
// administration accounts. All reads join the caller's tenant transaction.
func (l *ParentMessageLeadership) ParentMessageLeaders(ctx context.Context) ([]ParentMessageLeader, error) {
	rows, err := l.staff.StaffWithLeadershipPermission(ctx)
	if err != nil {
		return nil, err
	}
	readers, err := l.staff.StaffWithMessageReadPermission(ctx)
	if err != nil {
		return nil, err
	}
	canRead := make(map[int64]bool, len(readers))
	for _, reader := range readers {
		canRead[reader.AccountID] = true
	}
	adminIDs, err := l.admins.ListEffectiveAdminAccountIDs(ctx)
	if err != nil {
		return nil, err
	}
	leaders := make([]ParentMessageLeader, 0, len(rows))
	seen := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row.AccountID <= 0 || !canRead[row.AccountID] || seen[row.AccountID] {
			continue
		}
		seen[row.AccountID] = true
		leaders = append(leaders, row)
	}
	for _, accountID := range adminIDs {
		if accountID > 0 && !seen[accountID] {
			seen[accountID] = true
			leaders = append(leaders, ParentMessageLeader{AccountID: accountID})
		}
	}
	sort.SliceStable(leaders, func(i, j int) bool { return leaderBefore(leaders[i], leaders[j]) })
	return leaders, nil
}

// leaderBefore orders named persons by name before unnamed accounts by id.
func leaderBefore(a, b ParentMessageLeader) bool {
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

// LeadershipAccountIDs uses exactly the conversation readers for staff push.
func (l *ParentMessageLeadership) LeadershipAccountIDs(ctx context.Context) ([]int64, error) {
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
