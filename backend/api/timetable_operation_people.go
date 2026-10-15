package api

import (
	"context"

	users "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose/vocabulary"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
)

// TimetableOperationPeople binds the timetable operations' person port. Its
// only staff read resolves the acting account's staff ID, so it asks School
// Membership for the membership row alone and skips the Workforce employment
// half (#2753). Every other person read goes to the users service unchanged.
type TimetableOperationPeople struct {
	timetableCompose.OperationPeople
	Membership schoolmembership.Capability
}

func (p TimetableOperationPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*users.Staff, error) {
	members, err := p.Membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: []int64{personID}, MembershipOnly: true})
	if err != nil || len(members) == 0 {
		return nil, err
	}
	staff := &users.Staff{PersonID: members[0].PersonID, DeletedAt: members[0].DeletedAt}
	staff.ID = members[0].ID
	staff.SetTenantID(members[0].TenantID)
	return staff, nil
}
