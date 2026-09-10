package data

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/modules/devicescan"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
)

// deviceSubmitFeedback handles feedback submission from RFID devices
func (rs *FeedbackResource) deviceSubmitFeedback(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	deviceCtx, ok := rs.runtime.requireDevice(w, r)
	if !ok {
		rs.ObserveResponse(http.StatusUnauthorized, "unauthorized")
		return
	}

	rs.Logger.InfoContext(r.Context(), "starting feedback submission",
		slog.String("device_id", deviceCtx.DeviceID),
		slog.Int64("device_db_id", deviceCtx.ID),
	)

	// Check if feedback is enabled for this tenant (defense in depth).
	// Resolution failures are operational failures, not permission to write.
	enabled, err := rs.FeedbackService.Available(r.Context())
	if err != nil {
		rs.Logger.ErrorContext(r.Context(), "failed to resolve feedback availability", slog.String("error", err.Error()))
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		rs.ObserveResponse(http.StatusInternalServerError, "internal_error")
		return
	}
	if !enabled {
		rs.runtime.Success(w, r, http.StatusOK, map[string]interface{}{
			"status": "skipped",
			"reason": "feedback_disabled",
		}, "Feedback is disabled for this tenant")
		rs.ObserveResponse(http.StatusOK, "feedback_disabled")
		return
	}

	// Parse request
	req := &IoTFeedbackRequest{}
	if err := render.Bind(r, req); err != nil {
		rs.Logger.ErrorContext(r.Context(), "invalid feedback request",
			slog.String("device_id", deviceCtx.DeviceID),
			slog.String("error", err.Error()),
		)
		rs.runtime.Failure(w, r, http.StatusBadRequest, err, "")
		rs.ObserveResponse(http.StatusBadRequest, "invalid_parameters")
		return
	}

	rs.Logger.DebugContext(r.Context(), "received feedback",
		slog.Int64("student_id", req.StudentID),
		slog.String("value", req.Value),
	)

	// Validate student exists before creating feedback. The read takes a FOR
	// UPDATE row lock held for this request's tenant transaction, because the
	// alumnus refusal below is only worth as much as the window between the check
	// and the INSERT: a grade transition apply locks exactly this row, flips it to
	// alumnus and reconciles the child's records, so a status read without the
	// lock is already stale when the feedback row lands. The enrollment FK is a
	// soft-delete reference and happily accepts a graduate, so nothing downstream
	// would catch it. Under the lock the two serialize — either graduation
	// commits first and we see the alumnus status, or we hold the row and it
	// waits for our entry to be visible to its own pass (#405 review).
	student, err := rs.Students.LockFeedbackStudent(r.Context(), req.StudentID)
	if err != nil && !errors.Is(err, devicescan.ErrFeedbackStudentNotFound) {
		rs.Logger.ErrorContext(r.Context(), "failed to lookup student",
			slog.Int64("student_id", req.StudentID),
			slog.String("error", err.Error()),
		)
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		rs.ObserveResponse(http.StatusInternalServerError, "internal_error")
		return
	}

	if errors.Is(err, devicescan.ErrFeedbackStudentNotFound) {
		rs.Logger.WarnContext(r.Context(), "student not found",
			slog.Int64("student_id", req.StudentID),
		)
		rs.runtime.Failure(w, r, http.StatusNotFound, errors.New("student not found"), "")
		rs.ObserveResponse(http.StatusNotFound, "student_not_found")
		return
	}

	// Alumni (graduated, soft-deleted) are removed from every kiosk and staff
	// workflow, so a feedback submission that arrives after the graduation
	// commits — a kiosk holding a stale roster, or a scan queued before the
	// apply — must not write a new row against them. Decided on the LOCKED row
	// read above. Answered with the same "student not found" 404 the
	// unknown-student branch above returns, so PyrePortal needs no new error
	// mapping (#405).
	if student.Alumnus {
		rs.Logger.InfoContext(r.Context(), "feedback rejected: student graduated",
			slog.Int64("student_id", req.StudentID),
		)
		rs.runtime.Failure(w, r, http.StatusNotFound, errors.New("student not found"), "")
		rs.ObserveResponse(http.StatusNotFound, "student_not_found")
		return
	}

	rs.Logger.DebugContext(r.Context(), "student validated",
		slog.Int64("student_id", student.ID),
	)

	// Create feedback entry with server-side timestamps
	now := time.Now()
	day, feedbackTime := feedbackModule.TimestampParts(now)
	input := feedbackModule.CreateEntry{
		StudentID:       req.StudentID,
		Value:           req.Value,
		Day:             day,
		Time:            feedbackTime,
		IsMensaFeedback: false,
	}

	// Create feedback entry (validation happens in service layer)
	entry, err := rs.FeedbackService.Submit(r.Context(), input)
	if err != nil {
		rs.Logger.ErrorContext(r.Context(), "failed to create feedback entry",
			slog.String("error", err.Error()),
		)
		status := feedbackErrorStatus(err)
		rs.runtime.Failure(w, r, status, err, "")
		rs.ObserveResponse(status, feedbackModule.ErrorCode(err))
		return
	}

	rs.Logger.InfoContext(r.Context(), "created feedback entry",
		slog.Int64("entry_id", entry.ID),
		slog.Int64("student_id", req.StudentID),
	)

	// Prepare response
	response := map[string]interface{}{
		"id":         entry.ID,
		"student_id": entry.StudentID,
		"value":      entry.Value,
		"day":        string(entry.Day),
		"time":       entry.Time,
		"created_at": entry.CreatedAt,
	}

	rs.runtime.Success(w, r, http.StatusCreated, response, "Feedback submitted successfully")
	rs.ObserveResponse(http.StatusCreated, "none")
}
