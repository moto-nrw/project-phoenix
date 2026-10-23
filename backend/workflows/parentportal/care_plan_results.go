package parentportal

import (
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type (
	PendingOfferingChange = care.PendingOfferingChange
	ChildCareOfferings    = care.ChildCareOfferings
)

// SickNoteResult keeps its own shape: its status days are the portal's
// snapshots of the Care Plan rows the application returns.
type SickNoteResult struct {
	StatusDays     []*StudentStatusDaySnapshot
	PendingRequest *ExcusedAbsenceRequestSnapshot
}

func carePlanFromAppSickNoteResult(v care.SickNoteResult) SickNoteResult {
	out := SickNoteResult{PendingRequest: v.PendingRequest}
	if v.StatusDays != nil {
		out.StatusDays = make([]*StudentStatusDaySnapshot, len(v.StatusDays))
		for i, row := range v.StatusDays {
			if row != nil {
				out.StatusDays[i] = &StudentStatusDaySnapshot{ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, StudentID: row.StudentID, Date: row.Date, Status: row.Status, ReportedAt: row.ReportedAt, ClearedAt: row.ClearedAt, Source: row.Source, GuardianAccountID: row.GuardianAccountID, Note: row.Note}
			}
		}
	}
	return out
}
