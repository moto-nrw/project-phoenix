package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

type GuardianAbsenceReports interface{ care.GuardianAbsenceReports }
type GuardianAbsenceReportsCallbacks struct {
	ReportGuardianAbsence func(ctx context.Context, report GuardianAbsenceReportSnapshot) ([]StudentStatusDaySnapshot, error)
}

func NewGuardianAbsenceReports(callbacks GuardianAbsenceReportsCallbacks) GuardianAbsenceReports {
	return guardianAbsenceReports{callbacks}
}

type guardianAbsenceReports struct {
	callbacks GuardianAbsenceReportsCallbacks
}

func (a guardianAbsenceReports) ReportGuardianAbsence(ctx context.Context, report care.GuardianAbsenceReport) ([]care.StudentStatusDay, error) {
	return a.callbacks.ReportGuardianAbsence(ctx, report)
}
