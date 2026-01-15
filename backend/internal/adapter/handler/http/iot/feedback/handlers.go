package feedback

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	iotCommon "github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/feedback"
)

// deviceSubmitFeedback handles feedback submission from RFID devices
func (rs *Resource) deviceSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx := device.DeviceFromCtx(r.Context())

	if deviceCtx == nil {
		if render.Render(w, r, device.ErrDeviceUnauthorized(device.ErrMissingAPIKey)) != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return
	}

	logger.Logger.WithFields(map[string]interface{}{
		"device_id":   deviceCtx.DeviceID,
		"device_db_id": deviceCtx.ID,
	}).Info("Starting feedback submission")

	// Parse request
	req := &IoTFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"device_id": deviceCtx.DeviceID,
			"error":     err.Error(),
		}).Error("Invalid feedback request")
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	logger.Logger.WithFields(map[string]interface{}{
		"student_id": req.StudentID,
		"value":      req.Value,
	}).Info("Received feedback")

	// Validate student exists before creating feedback
	student, err := rs.UsersService.GetStudentByID(r.Context(), req.StudentID)
	if err != nil {
		logger.Logger.WithFields(map[string]interface{}{
			"student_id": req.StudentID,
			"error":      err.Error(),
		}).Error("Failed to lookup student")
		if errors.Is(err, sql.ErrNoRows) {
			iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("student not found")))
		} else {
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		}
		return
	}

	logger.Logger.WithField("student_id", student.ID).Debug("Student validated")

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
		logger.Logger.WithField("error", err.Error()).Error("Failed to create feedback entry")
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	logger.Logger.WithFields(map[string]interface{}{
		"entry_id":   entry.ID,
		"student_id": req.StudentID,
	}).Info("Successfully created feedback entry")

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
