package owners

import (
	"context"

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

// BindParentMessageLeadership wires Communication's reader policy to the
// school's staff permission facts and administrator accounts.
func BindParentMessageLeadership(staff permissionStaff, admins effectiveAdmins) *communication.ParentMessageLeadership {
	return communication.NewParentMessageLeadership(parentMessageLeadershipStaff{staff: staff}, admins)
}

type parentMessageLeadershipStaff struct {
	staff permissionStaff
}

func (s parentMessageLeadershipStaff) StaffWithLeadershipPermission(ctx context.Context) ([]communication.ParentMessageLeader, error) {
	return s.list(ctx, permissions.ParentMessagesLeadership)
}

func (s parentMessageLeadershipStaff) StaffWithMessageReadPermission(ctx context.Context) ([]communication.ParentMessageLeader, error) {
	return s.list(ctx, permissions.UsersRead)
}

func (s parentMessageLeadershipStaff) list(ctx context.Context, permission string) ([]communication.ParentMessageLeader, error) {
	rows, err := s.staff.ListStaffWithPermission(ctx, permission)
	if err != nil {
		return nil, err
	}
	staff := make([]communication.ParentMessageLeader, 0, len(rows))
	for _, row := range rows {
		if row != nil {
			staff = append(staff, communication.ParentMessageLeader{AccountID: row.AccountID, FirstName: row.FirstName, LastName: row.LastName})
		}
	}
	return staff, nil
}

// ParentMessagePeopleWithLeaders is the staff side's People Directory slice
// together with the leadership persons (#5200).
type ParentMessagePeopleWithLeaders struct {
	communication.ParentMessagePeople
	*communication.ParentMessageLeadership
}
