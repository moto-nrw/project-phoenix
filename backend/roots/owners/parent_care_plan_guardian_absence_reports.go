package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentGuardianAbsenceReports(source careplan.GuardianAbsenceReports) parentportal.GuardianAbsenceReports {
	if source == nil {
		return nil
	}
	return parentportal.NewGuardianAbsenceReports(parentportal.GuardianAbsenceReportsCallbacks{ReportGuardianAbsence: func(ctx context.Context, report parentportal.GuardianAbsenceReportSnapshot) ([]parentportal.StudentStatusDaySnapshot, error) {
		value, err := source.ReportGuardianAbsence(ctx, parentCarePlanToOwnerGuardianAbsenceReport(report))
		return parentCarePlanSlice(value, parentCarePlanFromOwnerStudentStatusDay), err
	},
	})
}
