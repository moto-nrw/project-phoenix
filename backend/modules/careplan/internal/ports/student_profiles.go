package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// StudentProfileStore is the persistence port over the Care Plan's student
// profile columns: care profile and departure plan, live status flags and
// health information.
type StudentProfileStore interface {
	SaveStudentCareProfile(ctx context.Context, profile domain.StudentCareProfile, plan *domain.StudentDeparturePlan) (domain.OperationStats, error)
	ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, domain.OperationStats, error)
	SetStudentLiveStatus(ctx context.Context, input domain.StudentLiveStatus) (int64, domain.OperationStats, error)
	SetStudentHealthInfo(ctx context.Context, studentID int64, healthInfo *string) (int64, domain.OperationStats, error)
}
