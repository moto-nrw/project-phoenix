package students

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
	activeService "github.com/moto-nrw/project-phoenix/services/active"
)

func newStudentStatusDayResponse(entry *active.StudentStatusDay) StudentStatusDayResponse {
	return StudentStatusDayResponse{
		ID:         entry.ID,
		StudentID:  entry.StudentID,
		Date:       entry.Date.String(),
		Status:     entry.Status,
		Label:      studentStatusDayLabel(entry.Status),
		ReportedAt: entry.ReportedAt,
		ClearedAt:  entry.ClearedAt,
		Source:     entry.Source,
		Note:       entry.Note,
		CreatedAt:  entry.CreatedAt,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func studentStatusDayLabel(status string) string {
	switch status {
	case active.StudentStatusDaySick:
		return "Krank"
	case active.StudentStatusDayClassTrip:
		return "Klassenfahrt"
	default:
		return "Entschuldigt"
	}
}

func newStudentStatusDayResponses(entries []*active.StudentStatusDay) []StudentStatusDayResponse {
	responses := make([]StudentStatusDayResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newStudentStatusDayResponse(entry))
	}
	return responses
}

// newStudentStatusDayConflictResponses maps active rows for a 409 conflict
// payload. Free-text notes are omitted so proxy/server logs of the error body
// cannot capture sick-note content (GDPR).
func newStudentStatusDayConflictResponses(entries []*active.StudentStatusDay) []StudentStatusDayResponse {
	responses := newStudentStatusDayResponses(entries)
	for i := range responses {
		responses[i].Note = nil
	}
	return responses
}

func applyEffectiveStatusDaysToResponses(responses []StudentResponse, statusRows []*active.StudentStatusDay) {
	if len(responses) == 0 || len(statusRows) == 0 {
		return
	}

	rowsByStudent := make(map[int64][]*active.StudentStatusDay, len(statusRows))
	for _, row := range statusRows {
		rowsByStudent[row.StudentID] = append(rowsByStudent[row.StudentID], row)
	}

	for i := range responses {
		applyEffectiveStatusDays(&responses[i], rowsByStudent[responses[i].ID])
	}
}

// applyStatusDaysForDate overlays the requested day's sick/excused/class-trip
// status rows onto the responses. The error is returned rather than swallowed:
// on a non-today view resetScheduledStatusFlags has already cleared the
// row-seeded flags, so a silent lookup failure would report a sick, excused, or
// class-trip child as expected. Callers must surface the error (500) instead of
// serving an incorrect attendance plan (#1939).
func (rs *Resource) applyStatusDaysForDate(ctx context.Context, responses []StudentResponse, now time.Time) error {
	if rs.StudentStatusDayService == nil || len(responses) == 0 {
		return nil
	}

	studentIDs := make([]int64, 0, len(responses))
	for _, response := range responses {
		studentIDs = append(studentIDs, response.ID)
	}
	rows, err := rs.StudentStatusDayService.GetActiveByStudentIDsAndDate(ctx, studentIDs, timezone.DateFromTime(now))
	if err != nil {
		return err
	}
	applyEffectiveStatusDaysToResponses(responses, rows)
	return nil
}

func (rs *Resource) applyStatusDaysForDateToResponse(ctx context.Context, response *StudentResponse, now time.Time) {
	if response == nil || rs.StudentStatusDayService == nil {
		return
	}
	date := timezone.DateFromTime(now)
	rows, err := rs.StudentStatusDayService.GetActiveByStudentAndDateRange(ctx, response.ID, date, date)
	if err != nil {
		if rs.Logger != nil {
			rs.Logger.Warn(
				"failed to apply student status days to response",
				"student_id", response.ID,
				"error", err.Error(),
			)
		}
		return
	}
	applyEffectiveStatusDays(response, rows)
}

func applyEffectiveStatusDays(response *StudentResponse, statusRows []*active.StudentStatusDay) {
	if response == nil || len(statusRows) == 0 {
		return
	}

	eff := activeService.ResolveEffectiveStatus(statusRows)
	if eff.Sick {
		response.Sick = true
		response.SickSince = eff.SickSince
		response.ClassTrip = false
		response.ClassTripSince = nil
		response.Excused = false
		response.ExcusedSince = nil
		return
	}

	if eff.ClassTrip {
		response.ClassTrip = true
		response.ClassTripSince = eff.ClassTripSince
		response.Sick = false
		response.SickSince = nil
		response.Excused = false
		response.ExcusedSince = nil
		return
	}

	if eff.Excused && !response.Sick {
		response.Excused = true
		response.ExcusedSince = eff.ExcusedSince
	}
}
