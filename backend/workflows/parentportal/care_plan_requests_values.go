package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

func carePlanToAppExcusedAbsenceRequest(value ExcusedAbsenceRequestSnapshot) care.ExcusedAbsenceRequest {
	return care.ExcusedAbsenceRequest{
		ID:             value.ID,
		TenantID:       value.TenantID,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		StudentID:      value.StudentID,
		SubmittedBy:    value.SubmittedBy,
		Dates:          value.Dates,
		Note:           value.Note,
		AbsenceStatus:  value.AbsenceStatus,
		Status:         value.Status,
		DecisionReason: value.DecisionReason,
		ReviewedBy:     value.ReviewedBy,
		ReviewedAt:     value.ReviewedAt,
		AppliedAt:      value.AppliedAt,
	}
}
func carePlanFromAppExcusedAbsenceRequest(value care.ExcusedAbsenceRequest) ExcusedAbsenceRequestSnapshot {
	return ExcusedAbsenceRequestSnapshot{
		ID:             value.ID,
		TenantID:       value.TenantID,
		CreatedAt:      value.CreatedAt,
		UpdatedAt:      value.UpdatedAt,
		StudentID:      value.StudentID,
		SubmittedBy:    value.SubmittedBy,
		Dates:          value.Dates,
		Note:           value.Note,
		AbsenceStatus:  value.AbsenceStatus,
		Status:         value.Status,
		DecisionReason: value.DecisionReason,
		ReviewedBy:     value.ReviewedBy,
		ReviewedAt:     value.ReviewedAt,
		AppliedAt:      value.AppliedAt,
	}
}

func carePlanFromAppExcusedRequestCreateInput(value care.ExcusedRequestCreateInput) ExcusedRequestCreateInputSnapshot {
	return ExcusedRequestCreateInputSnapshot{
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Dates:             value.Dates,
		Note:              value.Note,
		AbsenceStatus:     value.AbsenceStatus,
		NoteRequired:      value.NoteRequired,
	}
}

func carePlanFromAppExcusedRequestEditInput(value care.ExcusedRequestEditInput) ExcusedRequestEditInputSnapshot {
	return ExcusedRequestEditInputSnapshot{
		RequestID:         value.RequestID,
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		ExpectedVersion:   value.ExpectedVersion,
		NoteRequired:      value.NoteRequired,
		Dates:             value.Dates,
		Note:              value.Note,
	}
}

func carePlanFromAppGuardianAbsenceReport(value care.GuardianAbsenceReport) GuardianAbsenceReportSnapshot {
	return GuardianAbsenceReportSnapshot{
		StudentID:         value.StudentID,
		GuardianAccountID: value.GuardianAccountID,
		Dates:             value.Dates,
		Status:            value.Status,
		Note:              value.Note,
		ReportedAt:        value.ReportedAt,
	}
}
