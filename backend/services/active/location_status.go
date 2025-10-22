package active

import (
	"context"
	"errors"
	"strings"

	locationModels "github.com/moto-nrw/project-phoenix/models/location"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// GetStudentLocationStatus resolves the structured location status for a student.
func (s *service) GetStudentLocationStatus(ctx context.Context, studentID int64) (*locationModels.Status, error) {
	// Default to HOME when we cannot determine a more specific state.
	status := locationModels.NewStatus(locationModels.StateHome, nil)

	attendance, err := s.GetStudentAttendanceStatus(ctx, studentID)
	if err != nil {
		return status, err
	}

	// If the student is not currently checked in, treat as HOME.
	if attendance == nil || attendance.Status != "checked_in" {
		return status, nil
	}

	// Attempt to load the active visit.
	visit, err := s.GetStudentCurrentVisit(ctx, studentID)
	if err != nil {
		var activeErr *ActiveError
		if errors.As(err, &activeErr) && errors.Is(activeErr.Err, ErrVisitNotFound) {
			return locationModels.NewStatus(locationModels.StateTransit, nil), nil
		}
		return status, err
	}

	if visit == nil || !visit.IsActive() {
		return locationModels.NewStatus(locationModels.StateTransit, nil), nil
	}

	// Load the active group with room metadata.
	activeGroup, err := s.groupRepo.FindByID(ctx, visit.ActiveGroupID)
	if err != nil || activeGroup == nil {
		return locationModels.NewStatus(locationModels.StateTransit, nil), nil
	}

	room := activeGroup.Room
	if room == nil {
		if fetchedRoom, fetchErr := s.roomRepo.FindByID(ctx, activeGroup.RoomID); fetchErr == nil {
			room = fetchedRoom
		}
	}

	// Derive owner type metadata.
	ownerType := locationModels.RoomOwnerActivity
	isGroupRoom := false

	var studentRecord *userModels.Student
	if student, studentErr := s.studentRepo.FindByID(ctx, studentID); studentErr == nil {
		studentRecord = student
	}

	if group, groupErr := s.educationGroupRepo.FindByID(ctx, activeGroup.GroupID); groupErr == nil && group != nil {
		ownerType = locationModels.RoomOwnerGroup
		if studentRecord != nil && studentRecord.GroupID != nil && room != nil && group.RoomID != nil && *studentRecord.GroupID == group.ID && *group.RoomID == room.ID {
			isGroupRoom = true
		}
	}

	if room == nil {
		// Without room metadata we cannot display a PRESENT state.
		return locationModels.NewStatus(locationModels.StateTransit, nil), nil
	}

	// Detect schoolyard based on known naming/category conventions.
	roomName := strings.ToLower(room.Name)
	roomCategory := strings.ToLower(room.Category)
	isSchoolyard := strings.Contains(roomName, "schulhof") || strings.Contains(roomCategory, "schoolyard") || strings.Contains(roomCategory, "hof")

	roomPayload := &locationModels.Room{
		ID:          room.ID,
		Name:        room.Name,
		IsGroupRoom: isGroupRoom,
		OwnerType:   ownerType,
	}

	if isSchoolyard {
		return locationModels.NewStatus(locationModels.StateSchoolyard, roomPayload), nil
	}

	return locationModels.NewStatus(locationModels.StatePresentInRoom, roomPayload), nil
}
