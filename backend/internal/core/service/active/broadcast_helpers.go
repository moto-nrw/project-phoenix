package active

import (
	"context"
	"fmt"

	activeModels "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	userModels "github.com/moto-nrw/project-phoenix/internal/core/domain/users"
	"github.com/moto-nrw/project-phoenix/internal/core/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
)

// broadcastVisitCheckout broadcasts an SSE event when a student checks out
func (s *service) broadcastVisitCheckout(ctx context.Context, endedVisit *activeModels.Visit) {
	if s.broadcaster == nil || endedVisit == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", endedVisit.ActiveGroupID)
	studentID := fmt.Sprintf("%d", endedVisit.StudentID)
	studentName, studentRec := s.getStudentDisplayData(ctx, endedVisit.StudentID)

	event := port.NewEvent(
		port.EventStudentCheckOut,
		activeGroupID,
		port.EventData{
			StudentID:   &studentID,
			StudentName: &studentName,
		},
	)

	s.broadcastWithLogging(activeGroupID, studentID, event, "student_checkout")
	s.broadcastToEducationalGroup(studentRec, event)
}

// broadcastToEducationalGroup mirrors active-group broadcasts to the student's OGS group topic
func (s *service) broadcastToEducationalGroup(student *userModels.Student, event port.Event) {
	if s.broadcaster == nil || student == nil || student.GroupID == nil {
		return
	}
	groupID := fmt.Sprintf("edu:%d", *student.GroupID)
	if err := s.broadcaster.BroadcastToGroup(groupID, event); err != nil {
		if logger.Logger != nil {
			studentID := ""
			if event.Data.StudentID != nil {
				studentID = *event.Data.StudentID
			}
			logger.Logger.WithFields(map[string]interface{}{
				"error":                 err.Error(),
				"event_type":            string(event.Type),
				"education_group_topic": groupID,
				"student_id":            studentID,
			}).Error(sseErrorMessage + " for educational topic")
		}
	}
}

// broadcastStudentCheckoutEvents sends checkout SSE events for each visit.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastStudentCheckoutEvents(sessionIDStr string, visitsToNotify []visitSSEData) {
	for _, visitData := range visitsToNotify {
		studentIDStr := fmt.Sprintf("%d", visitData.StudentID)
		studentName := visitData.Name

		checkoutEvent := port.NewEvent(
			port.EventStudentCheckOut,
			sessionIDStr,
			port.EventData{
				StudentID:   &studentIDStr,
				StudentName: &studentName,
			},
		)

		s.broadcastWithLogging(sessionIDStr, studentIDStr, checkoutEvent, "student_checkout")
		s.broadcastToEducationalGroup(visitData.Student, checkoutEvent)
	}
}

// broadcastActivityEndEvent sends the activity_end SSE event for a completed session.
// This helper reduces cognitive complexity in session timeout processing.
func (s *service) broadcastActivityEndEvent(ctx context.Context, sessionID int64, sessionIDStr string) {
	finalGroup, err := s.groupReadRepo.FindByID(ctx, sessionID)
	if err != nil || finalGroup == nil {
		return
	}

	roomIDStr := fmt.Sprintf("%d", finalGroup.RoomID)
	activityName := s.getActivityName(ctx, finalGroup.GroupID)
	roomName := s.getRoomName(ctx, finalGroup.RoomID)

	event := port.NewEvent(
		port.EventActivityEnd,
		sessionIDStr,
		port.EventData{
			ActivityName: &activityName,
			RoomID:       &roomIDStr,
			RoomName:     &roomName,
		},
	)

	s.broadcastWithLogging(sessionIDStr, "", event, "activity_end")
}

// broadcastWithLogging broadcasts an event and logs any errors.
func (s *service) broadcastWithLogging(activeGroupID, studentID string, event port.Event, eventType string) {
	if err := s.broadcaster.BroadcastToGroup(activeGroupID, event); err != nil {
		if logger.Logger != nil {
			fields := map[string]interface{}{
				"error":           err.Error(),
				"event_type":      eventType,
				"active_group_id": activeGroupID,
			}
			if studentID != "" {
				fields["student_id"] = studentID
			}
			logger.Logger.WithFields(fields).Error(sseErrorMessage)
		}
	}
}

// broadcastActivityStartEvent broadcasts SSE event for activity start
func (s *service) broadcastActivityStartEvent(ctx context.Context, group *activeModels.Group, supervisorIDs []int64) {
	if s.broadcaster == nil || group == nil {
		return
	}

	activeGroupID := fmt.Sprintf("%d", group.ID)
	roomIDStr := fmt.Sprintf("%d", group.RoomID)

	supervisorIDStrs := make([]string, len(supervisorIDs))
	for i, id := range supervisorIDs {
		supervisorIDStrs[i] = fmt.Sprintf("%d", id)
	}

	activityName := s.getActivityName(ctx, group.GroupID)
	roomName := s.getRoomName(ctx, group.RoomID)

	event := port.NewEvent(
		port.EventActivityStart,
		activeGroupID,
		port.EventData{
			ActivityName:  &activityName,
			RoomID:        &roomIDStr,
			RoomName:      &roomName,
			SupervisorIDs: &supervisorIDStrs,
		},
	)

	s.broadcastWithLogging(activeGroupID, "", event, "activity_start")
}

// getActivityName retrieves the activity name by group ID, returning empty string on error.
func (s *service) getActivityName(ctx context.Context, groupID int64) string {
	activity, err := s.activityGroupRepo.FindByID(ctx, groupID)
	if err != nil || activity == nil {
		return ""
	}
	return activity.Name
}

// getRoomName retrieves the room name by room ID, returning empty string on error.
func (s *service) getRoomName(ctx context.Context, roomID int64) string {
	room, err := s.roomRepo.FindByID(ctx, roomID)
	if err != nil || room == nil {
		return ""
	}
	return room.Name
}
