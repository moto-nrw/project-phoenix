package checkin

import (
	"context"
	"strconv"

	adaptermiddleware "github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
)

func recordEventAction(ctx context.Context, action string) {
	if action == "" {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	event.Action = action
}

func recordEventStudentID(ctx context.Context, studentID int64) {
	if studentID <= 0 {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	event.StudentID = strconv.FormatInt(studentID, 10)
}

func recordEventRoomID(ctx context.Context, roomID int64) {
	if roomID <= 0 {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	event.RoomID = strconv.FormatInt(roomID, 10)
}

func recordEventGroupID(ctx context.Context, groupID int64) {
	if groupID <= 0 {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	event.GroupID = strconv.FormatInt(groupID, 10)
}

func recordEventError(ctx context.Context, errorType, errorCode string, err error) {
	if errorType == "" {
		return
	}
	event := adaptermiddleware.GetWideEvent(ctx)
	if event == nil || event.Timestamp.IsZero() {
		return
	}
	if event.ErrorType != "" {
		return
	}
	event.ErrorType = errorType
	if errorCode != "" {
		event.ErrorCode = errorCode
	}
	if err != nil {
		event.ErrorMessage = err.Error()
	}
}
