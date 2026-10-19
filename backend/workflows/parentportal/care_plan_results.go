package parentportal

import (
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type PendingOfferingChange struct {
	ID              int64
	CreatedAt       time.Time
	EffectiveFrom   calendar.Date
	Note            string
	Diff            []OfferingChangeDiffEntrySnapshot
	SubmittedBySelf bool
}
type ChildCareOfferings struct {
	PeriodName            string
	PeriodStart           calendar.Date
	PeriodEnd             calendar.Date
	Offerings             []CareOfferingSelection
	CanRequest            bool
	ChangesDisabledReason string
	PendingRequest        *PendingOfferingChange
	LastDecision          *OfferingChangeDecisionSnapshot
	EarliestEffectiveFrom calendar.Date
}
type SickNoteResult struct {
	StatusDays     []*StudentStatusDaySnapshot
	PendingRequest *ExcusedAbsenceRequestSnapshot
}

func carePlanFromAppPendingOfferingChange(v care.PendingOfferingChange) PendingOfferingChange {
	return PendingOfferingChange{ID: v.ID, CreatedAt: v.CreatedAt, EffectiveFrom: v.EffectiveFrom, Note: v.Note, Diff: carePlanValueSlice(v.Diff, carePlanFromAppOfferingChangeDiffEntry), SubmittedBySelf: v.SubmittedBySelf}
}
func carePlanFromAppChildCareOfferings(v care.ChildCareOfferings) ChildCareOfferings {
	return ChildCareOfferings{PeriodName: v.PeriodName, PeriodStart: v.PeriodStart, PeriodEnd: v.PeriodEnd, Offerings: v.Offerings, CanRequest: v.CanRequest, ChangesDisabledReason: v.ChangesDisabledReason, PendingRequest: carePlanValuePointer(v.PendingRequest, carePlanFromAppPendingOfferingChange), LastDecision: carePlanValuePointer(v.LastDecision, carePlanFromAppOfferingChangeDecision), EarliestEffectiveFrom: v.EarliestEffectiveFrom}
}
func carePlanFromAppSickNoteResult(v care.SickNoteResult) SickNoteResult {
	out := SickNoteResult{PendingRequest: carePlanValuePointer(v.PendingRequest, carePlanFromAppExcusedAbsenceRequest)}
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
