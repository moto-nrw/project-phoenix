package feedback

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	iotCommon "github.com/moto-nrw/project-phoenix/api/iot/common"
	"github.com/moto-nrw/project-phoenix/auth/device"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

	slog.Default().InfoContext(r.Context(), "starting feedback submission",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("device_db_id", deviceCtx.ID),
	)

	// Parse request
	req := &IoTFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		slog.Default().ErrorContext(r.Context(), "invalid feedback request",
			slog.String("device_id", deviceCtx.DeviceID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInvalidRequest(err))
		return
	}

	slog.Default().DebugContext(r.Context(), "received feedback",
		slog.Int64("student_id", req.StudentID),
		slog.String("value", req.Value),
	)

	// Validate student exists before creating feedback
	studentRepo := rs.UsersService.StudentRepository()
	student, err := studentRepo.FindByID(r.Context(), req.StudentID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Default().ErrorContext(r.Context(), "failed to lookup student",
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorInternalServer(err))
		return
	}

	if errors.Is(err, sql.ErrNoRows) {
		slog.Default().WarnContext(r.Context(), "student not found",
			slog.Int64("student_id", req.StudentID),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorNotFound(errors.New("student not found")))
		return
	}

	slog.Default().DebugContext(r.Context(), "student validated",
		slog.Int64("student_id", student.ID),
	)

	// Create feedback entry with server-side timestamps
	now := time.Now()
	entry := &feedback.Entry{
		StudentID:       req.StudentID,
		Value:           req.Value,
		Day:             timezone.DateOf(now), // Date only (Berlin timezone)
		Time:            now,                  // Full timestamp
		IsMensaFeedback: false,
	}

	// Create feedback entry (validation happens in service layer)
	if err = rs.FeedbackService.CreateEntry(r.Context(), entry); err != nil {
		slog.Default().ErrorContext(r.Context(), "failed to create feedback entry",
			slog.String("error", err.Error()),
		)
		iotCommon.RenderError(w, r, iotCommon.ErrorRenderer(err))
		return
	}

	slog.Default().InfoContext(r.Context(), "created feedback entry",
		slog.Int64("entry_id", entry.ID),
		slog.Int64("student_id", req.StudentID),
	)

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
