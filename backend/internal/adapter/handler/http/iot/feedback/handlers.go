package feedback

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware"
	"github.com/moto-nrw/project-phoenix/internal/adapter/middleware/device"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
)

// deviceSubmitFeedback handles feedback submission from RFID devices
func (rs *Resource) deviceSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	event := middleware.GetWideEvent(r.Context())
	event.Action = "iot_feedback"

	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		event.ErrorType = "Unauthorized"
		event.ErrorMessage = device.ErrMissingAPIKey.Error()
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}
	event.UserID = deviceCtx.DeviceID
	event.UserRole = "device"

	// Parse request
	req := &IoTFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		event.ErrorType = "InvalidRequest"
		event.ErrorMessage = err.Error()
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}
	event.StudentID = strconv.FormatInt(req.StudentID, 10)

	// Validate student exists before creating feedback
	student, err := rs.UsersService.GetStudentByID(r.Context(), req.StudentID)
	if err != nil {
		event.ErrorType = "StudentLookupError"
		event.ErrorMessage = err.Error()
		if errors.Is(err, sql.ErrNoRows) {
			iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("student not found")))
		} else {
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		}
		return
	}
	event.StudentID = strconv.FormatInt(student.ID, 10)

	// Create feedback entry with server-side timestamps
	now := time.Now()
	entry := &feedback.Entry{
		StudentID:       req.StudentID,
		Value:           req.Value,
		Day:             now.Truncate(24 * time.Hour), // Date only
		Time:            now,                          // Full timestamp
		IsMensaFeedback: false,
	}

	// Create feedback entry (validation happens in service layer)
	if err = rs.FeedbackService.CreateEntry(r.Context(), entry); err != nil {
		event.ErrorType = "CreateFeedbackError"
		event.ErrorMessage = err.Error()
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	// Prepare response
	response := map[string]interface{}{
		"id":         entry.ID,
		"student_id": entry.StudentID,
		"value":      entry.Value,
		"day":        entry.GetFormattedDate(),
		"time":       entry.GetFormattedTime(),
		"created_at": entry.CreatedAt,
	}

	common.Respond(w, r, http.StatusCreated, response, "Feedback submitted successfully")
}
