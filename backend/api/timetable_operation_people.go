package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
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

func (p TimetableOperationPeople) GetStaffByPersonID(ctx context.Context, personID int64) (*timetable.StaffMember, error) {
	members, err := p.Membership.ListStaff(ctx, schoolmembership.StaffFilter{PersonIDs: []int64{personID}, MembershipOnly: true})
	if err != nil || len(members) == 0 {
		return nil, err
	}
	return &timetable.StaffMember{ID: members[0].ID, PersonID: members[0].PersonID}, nil
}
