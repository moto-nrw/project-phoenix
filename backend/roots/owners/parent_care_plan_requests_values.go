package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentCarePlanFromOwnerExcusedAbsenceRequest(value careplan.ExcusedAbsenceRequest) parentportal.ExcusedAbsenceRequestSnapshot {
	return parentportal.ExcusedAbsenceRequestSnapshot{
		ID:             value.ID,
		TenantID:       value.TenantID,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		StudentID:      value.StudentID,
		SubmittedBy:    value.SubmittedBy,
		Dates:          parentCarePlanSlice(value.Dates, func(date careplan.Date) calendar.Date { return calendar.Date(date) }),
		Note:           value.Note,
		AbsenceStatus:  value.AbsenceStatus,
		Status:         value.Status,
		DecisionReason: value.DecisionReason,
		ReviewedBy:     value.ReviewedBy,
		ReviewedAt:     value.ReviewedAt,
		AppliedAt:      value.AppliedAt,
	}
}

func parentCarePlanToOwnerExcusedRequestCreateInput(value parentportal.ExcusedRequestCreateInputSnapshot) careplan.ExcusedRequestCreateInput {
	return careplan.ExcusedRequestCreateInput{
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Dates:             parentCarePlanSlice(value.Dates, func(date calendar.Date) careplan.Date { return careplan.Date(date) }),
		Note:              value.Note,
		AbsenceStatus:     value.AbsenceStatus,
		NoteRequired:      value.NoteRequired,
	}
}

func parentCarePlanToOwnerExcusedRequestEditInput(value parentportal.ExcusedRequestEditInputSnapshot) careplan.ExcusedRequestEditInput {
	return careplan.ExcusedRequestEditInput{
		RequestID:         value.RequestID,
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		ExpectedVersion:   value.ExpectedVersion,
		NoteRequired:      value.NoteRequired,
		Dates:             parentCarePlanSlice(value.Dates, func(date calendar.Date) careplan.Date { return careplan.Date(date) }),
		Note:              value.Note,
	}
}

func parentCarePlanToOwnerGuardianAbsenceReport(value parentportal.GuardianAbsenceReportSnapshot) careplan.GuardianAbsenceReport {
	return careplan.GuardianAbsenceReport{
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Dates:             parentCarePlanSlice(value.Dates, func(date calendar.Date) careplan.Date { return careplan.Date(date) }),
		Status:            value.Status,
		Note:              value.Note,
		ReportedAt:        value.ReportedAt,
	}
}
