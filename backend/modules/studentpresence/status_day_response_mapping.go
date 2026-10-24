package studentpresence

import (
	absencerecords "github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
)

func NewStudentStatusDayResponse(entry *absencerecords.StudentStatusDay) StudentStatusDayResponse {
	return StudentStatusDayResponse{
		ID:         entry.ID,
		StudentID:  entry.StudentID,
		Date:       entry.Date.String(),
		Status:     entry.Status,
		Label:      StudentStatusDayLabel(entry.Status),
		ReportedAt: entry.ReportedAt,
		ClearedAt:  entry.ClearedAt,
		Source:     entry.Source,
		Note:       entry.Note,
		CreatedAt:  entry.CreatedAt,
		UpdatedAt:  entry.UpdatedAt,
	}
}

func StudentStatusDayLabel(status string) string {
	switch status {
	case absencerecords.StudentStatusDaySick:
		return "Krank"
	case absencerecords.StudentStatusDayClassTrip:
		return "Klassenfahrt"
	default:
		return "Entschuldigt"
	}
}

func NewStudentStatusDayResponses(entries []*absencerecords.StudentStatusDay) []StudentStatusDayResponse {
	responses := make([]StudentStatusDayResponse, 0, len(entries))
	for _, entry := range entries {
		responses = append(responses, NewStudentStatusDayResponse(entry))
	}
	return responses
}

// NewStudentStatusDayConflictResponses maps active rows for a 409 conflict
// payload. Free-text notes are omitted so proxy/server logs of the error body
// cannot capture sick-note content (GDPR).
func NewStudentStatusDayConflictResponses(entries []*absencerecords.StudentStatusDay) []StudentStatusDayResponse {
	responses := NewStudentStatusDayResponses(entries)
	for i := range responses {
		responses[i].Note = nil
	}
	return responses
}
