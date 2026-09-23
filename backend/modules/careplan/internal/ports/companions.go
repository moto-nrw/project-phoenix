package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// CompanionStudents is the People Directory half of the "läuft mit" graph:
// the child rows every edge writer locks, the departure plans the rules read,
// and the one write into another child's plan a confirmed extension makes.
type CompanionStudents interface {
	// FindCompanionStudent reads one child; nil when the row is gone.
	FindCompanionStudent(ctx context.Context, studentID int64) (*domain.CompanionStudent, error)
	// FindCompanionStudents reads the tenant's rows of the given children. A
	// missing child is absent from the map.
	FindCompanionStudents(ctx context.Context, studentIDs []int64) (map[int64]domain.CompanionStudent, error)
	// LockCompanionStudent takes one child's row lock. A missing row is not
	// an error. With noWait a held lock fails at once with
	// careplan.ErrCompanionLockBusy instead of waiting.
	LockCompanionStudent(ctx context.Context, studentID int64, noWait bool) error
	// ExtendAccompaniedDays widens one child's allowed departure modes so the
	// days permit leaving with another child, re-reading the row under its
	// lock and recording the change for the acting account. Purely additive.
	// It fails with careplan.ErrCompanionNotFound when the child is gone.
	ExtendAccompaniedDays(ctx context.Context, studentID int64, days []string, actorAccountID int64) error
}
