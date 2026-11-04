package owners

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/careexit"
)

// NewCareResumption binds the membership read/command and Care Plan ledger to
// the care-exit workflow. All operations join its one ambient tenant unit.
func NewCareResumption(students carePlanCompose.CareStudentDirectory, records careplan.Capability,
	trail carePlanCompose.CareEndRecorder, today func() calendar.Date, logger *slog.Logger,
) *careexit.Resumption {
	return careexit.NewResumption(careResumptionStudents{students}, records, trail,
		tenant.NewTransactionRunner().RunInTx, today, logger)
}

type careResumptionStudents struct {
	carePlanCompose.CareStudentDirectory
}

func (s careResumptionStudents) LockResumptionStudent(ctx context.Context, id int64) (careexit.ResumptionStudent, bool, error) {
	students, err := s.FindCareStudents(ctx, []int64{id}, true)
	if err != nil {
		return careexit.ResumptionStudent{}, false, err
	}
	student, found := students[id]
	return careexit.ResumptionStudent{Status: student.Status, EnrolledUntil: student.EnrolledUntil}, found, nil
}

// NewCarePlanRestoration binds the care lifecycle and Care Plan's student
// roster resync to the care-exit workflow that re-plans a restored care plan
// (#5235). The lifecycle's transaction joins the workflow's tenant unit.
func NewCarePlanRestoration(lifecycle careexit.RestoringCareExits, rosters careexit.RestoredPlanResync, today func() calendar.Date) *careexit.PlanRestoration {
	return careexit.NewPlanRestoration(lifecycle, rosters, tenant.NewTransactionRunner().RunInTx, today)
}
