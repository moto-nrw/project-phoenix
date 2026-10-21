//go:build testhooks

package fixturetest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// InstanceStaffRows write and read fixture assignments.
type InstanceStaffRows interface {
	Create(context.Context, *InstanceStaff) error
}

// InstanceStaffRowsOf writes fixture assignments through the owner's
// instance staff command, with the retained row checks.
func InstanceStaffRowsOf(staff timetable.InstanceStaffCommand) InstanceStaffRows {
	return instanceStaffRows{staff: staff}
}

type instanceStaffRows struct {
	staff timetable.InstanceStaffCommand
}

func (r instanceStaffRows) Create(ctx context.Context, row *InstanceStaff) error {
	if err := row.Validate(); err != nil {
		return err
	}
	created, err := r.staff.CreateInstanceStaff(ctx, timetable.InstanceStaffInput{
		InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID, IsPrimary: row.IsPrimary,
		IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent, AbsenceReason: row.AbsenceReason, SickAbsenceID: row.SickAbsenceID,
	})
	if err != nil {
		return err
	}
	row.ID, row.CreatedAt, row.UpdatedAt = created.ID, created.CreatedAt, created.UpdatedAt
	row.SetTenantID(created.TenantID)
	return nil
}

// PublicInstanceStaff is the owner's assignment of a fixture row.
func PublicInstanceStaff(row *InstanceStaff) timetable.InstanceStaff {
	return timetable.InstanceStaff{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID, IsPrimary: row.IsPrimary,
		IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent, AbsenceReason: row.AbsenceReason, SickAbsenceID: row.SickAbsenceID,
	}
}
