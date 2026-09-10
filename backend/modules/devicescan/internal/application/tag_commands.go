package application

import (
	"context"
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	"github.com/moto-nrw/project-phoenix/modules/devicescan/internal/ports"
)

func (q *tagAssignments) staffPerson(ctx context.Context, staffID int64) (*ports.StaffMember, *ports.Person, error) {
	staff, err := q.people.FindStaff(ctx, staffID)
	if err != nil || staff == nil {
		return nil, nil, devicescan.NotFound("staff not found")
	}
	person, err := q.people.FindPerson(ctx, staff.PersonID)
	if err != nil {
		return nil, nil, devicescan.Internal("failed to get person data for staff", err)
	}
	if person == nil {
		return nil, nil, devicescan.Internal("failed to get person data for staff", errors.New("person not found"))
	}
	return staff, person, nil
}

func (q *tagAssignments) AssignStaffTag(ctx context.Context, staffID int64, tag string) (devicescan.TagAssignmentChange, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return devicescan.TagAssignmentChange{}, devicescan.ErrDeviceUnauthorized
	}
	staff, person, err := q.staffPerson(ctx, staffID)
	if err != nil {
		return devicescan.TagAssignmentChange{}, err
	}
	previous := person.TagID
	if err := q.people.LinkTag(ctx, person.ID, tag); err != nil {
		return devicescan.TagAssignmentChange{}, err
	}
	response := devicescan.TagAssignmentChange{
		Success: true, StudentID: staff.ID, StudentName: person.FirstName + " " + person.LastName,
		RFIDTag: tag, PreviousTag: previous, Message: "RFID tag assigned successfully",
	}
	if previous != nil {
		response.Message = "RFID tag assigned successfully (previous tag replaced)"
	}
	q.logger.InfoContext(ctx, "RFID tag assigned to staff",
		slog.String("device_id", device.DeviceID),
		slog.Int64("staff_id", staffID),
		slog.String("tag", tag),
		slog.Any("previous_tag", previous),
	)
	return response, nil
}

func (q *tagAssignments) UnassignStaffTag(ctx context.Context, staffID int64) (devicescan.TagAssignmentChange, error) {
	device, ok := q.principals.Device(ctx)
	if !ok || device == nil {
		return devicescan.TagAssignmentChange{}, devicescan.ErrDeviceUnauthorized
	}
	staff, person, err := q.staffPerson(ctx, staffID)
	if err != nil {
		return devicescan.TagAssignmentChange{}, err
	}
	if person.TagID == nil {
		return devicescan.TagAssignmentChange{}, devicescan.NotFound("staff has no RFID tag assigned")
	}
	removed := *person.TagID
	if err := q.people.UnlinkTag(ctx, person.ID); err != nil {
		return devicescan.TagAssignmentChange{}, err
	}
	q.logger.InfoContext(ctx, "RFID tag unassigned from staff",
		slog.String("device_id", device.DeviceID),
		slog.Int64("staff_id", staffID),
		slog.String("tag", removed),
	)
	return devicescan.TagAssignmentChange{
		Success: true, StudentID: staff.ID, StudentName: person.FirstName + " " + person.LastName,
		RFIDTag: removed, Message: "RFID tag unassigned successfully",
	}, nil
}
