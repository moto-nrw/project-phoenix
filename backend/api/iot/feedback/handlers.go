package feedback

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/models/feedback"
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

	log.Printf("[FEEDBACK] Starting feedback submission - Device: %s (ID: %d)",
		deviceCtx.DeviceID, deviceCtx.ID)

	// Parse request
	req := &IoTFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		log.Printf("[FEEDBACK] ERROR: Invalid request from device %s: %v", deviceCtx.DeviceID, err)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	log.Printf("[FEEDBACK] Received feedback - StudentID: %d, Value: %s", req.StudentID, req.Value)

	// Validate student exists before creating feedback
	student, err := rs.UsersService.GetStudentByID(r.Context(), req.StudentID)
	if err != nil {
		log.Printf("[FEEDBACK] ERROR: Failed to lookup student %d: %v", req.StudentID, err)
		if errors.Is(err, sql.ErrNoRows) {
			iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("student not found")))
		} else {
			iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		}
		return
	}

	log.Printf("[FEEDBACK] Student %d validated", student.ID)

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
		log.Printf("[FEEDBACK] ERROR: Failed to create feedback entry: %v", err)
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	log.Printf("[FEEDBACK] Successfully created feedback entry ID: %d for student %d", entry.ID, req.StudentID)

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
