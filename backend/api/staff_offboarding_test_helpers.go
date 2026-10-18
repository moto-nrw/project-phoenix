//go:build testhooks

package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Timetable planning writes the People Directory offboarding suite races
// against the retirement of a staff member, in terms that suite may name.

// AssignInstanceStaffForTest plans a staff member onto a block through the
// Timetable owner's instance staff command.
func AssignInstanceStaffForTest(ctx context.Context, capability timetable.Capability, instanceID, staffID int64) error {
	_, err := capability.CreateInstanceStaff(ctx, timetable.InstanceStaffInput{InstanceID: instanceID, StaffID: staffID})
	return err
}

// MoveActivityInstanceForTest moves a block to another day through the
// retained activity instance rows.
func MoveActivityInstanceForTest(ctx context.Context, rows TimetableRepositories, instanceID int64, date calendar.Date) error {
	stored, err := rows.ActivityInstance.FindByID(ctx, instanceID)
	if err != nil {
		return err
	}
	stored.Date = timetableCompose.ActivityInstanceDate(date)
	return rows.ActivityInstance.Update(ctx, stored)
}
