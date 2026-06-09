package students

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/active"
)

func newStudentStatusDayResponse(entry *active.StudentStatusDay) StudentStatusDayResponse {
	label := "Krank"
	if entry.Status == active.StudentStatusDayExcused {
		label = "Entschuldigt"
	}

	return StudentStatusDayResponse{
		ID:         entry.ID,
		StudentID:  entry.StudentID,
		Date:       entry.Date.Format(dateFormatYYYYMMDD),
		Status:     entry.Status,
		Label:      label,
		ReportedAt: entry.ReportedAt,
		ClearedAt:  entry.ClearedAt,
		Source:     entry.Source,
		Note:       entry.Note,
		CreatedAt:  entry.CreatedAt,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func newStudentStatusDayResponses(entries []*active.StudentStatusDay) []StudentStatusDayResponse {
	responses := make([]StudentStatusDayResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, newStudentStatusDayResponse(entry))
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

func (rs *Resource) applyStatusDaysForDate(ctx context.Context, responses []StudentResponse, now time.Time) {
	if rs.StudentStatusDayRepo == nil || len(responses) == 0 {
		return
	}

	studentIDs := make([]int64, 0, len(responses))
	for _, response := range responses {
		studentIDs = append(studentIDs, response.ID)
	}
	rows, err := rs.StudentStatusDayRepo.FindActiveByStudentIDsAndDate(ctx, studentIDs, timezone.DateOfUTC(now))
	if err != nil {
		if rs.Logger != nil {
			rs.Logger.Warn("failed to apply student status days to responses", "error", err.Error())
		}
		return
	}
	applyEffectiveStatusDaysToResponses(responses, rows)
}

func (rs *Resource) applyStatusDaysForDateToResponse(ctx context.Context, response *StudentResponse, now time.Time) {
	if response == nil || rs.StudentStatusDayRepo == nil {
		return
	}
	date := timezone.DateOfUTC(now)
	rows, err := rs.StudentStatusDayRepo.FindActiveByStudentAndDateRange(ctx, response.ID, date, date)
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

	var sickRow *active.StudentStatusDay
	var excusedRow *active.StudentStatusDay
	for _, row := range statusRows {
		switch row.Status {
		case active.StudentStatusDaySick:
			if sickRow == nil || row.ReportedAt.After(sickRow.ReportedAt) {
				sickRow = row
			}
		case active.StudentStatusDayExcused:
			if excusedRow == nil || row.ReportedAt.After(excusedRow.ReportedAt) {
				excusedRow = row
			}
		}
	}

	if sickRow != nil {
		response.Sick = true
		response.SickSince = statusDayTimePtr(sickRow.ReportedAt)
		response.Excused = false
		response.ExcusedSince = nil
		return
	}

	if excusedRow != nil && !response.Sick {
		response.Excused = true
		response.ExcusedSince = statusDayTimePtr(excusedRow.ReportedAt)
	}
}

func statusDayTimePtr(v time.Time) *time.Time {
	return &v
}
