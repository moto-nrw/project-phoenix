//go:build testhooks

package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
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
func MoveActivityInstanceForTest(ctx context.Context, rows owners.TimetableRepositories, instanceID int64, date calendar.Date) error {
	instances := rows.NativeInstances()
	stored, err := instances.FindByID(ctx, instanceID)
	if err != nil {
		return err
	}
	stored.Date = date
	return instances.Update(ctx, stored)
}

// ReassignInstanceStaffForTest moves a planned assignment to another staff
// member through the Timetable owner's instance staff command.
func ReassignInstanceStaffForTest(ctx context.Context, staff timetable.InstanceStaffCapability, assignmentID, staffID int64) error {
	stored, err := staff.FindInstanceStaff(ctx, assignmentID)
	if err != nil {
		return err
	}
	_, err = staff.UpdateInstanceStaff(ctx, stored.ID, timetable.InstanceStaffInput{
		InstanceID: stored.InstanceID, StaffID: staffID, RoomID: stored.RoomID, IsPrimary: stored.IsPrimary,
		IsSubstitute: stored.IsSubstitute, IsAbsent: stored.IsAbsent, AbsenceReason: stored.AbsenceReason, SickAbsenceID: stored.SickAbsenceID,
	})
	return err
}
