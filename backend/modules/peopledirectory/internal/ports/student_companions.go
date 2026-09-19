package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentCompanions is what this owner needs from Care Plan's "läuft mit"
// edges. Care Plan owns users.student_companions; the directory reaches it
// through this consumer-owned port because narrowing a child's departure plan
// has to drop the links that plan no longer allows, in the one write path every
// writer passes through.
type StudentCompanions interface {
	// ListForStudent returns every stored edge touching the child.
	ListForStudent(ctx context.Context, studentID int64) ([]domain.CompanionEdge, error)
	// DaysCoveredExcluding answers, per child, the weekdays on which a link
	// other than the excluded child's still covers them. Passing 0 excludes
	// nobody, which is what the deferred batch verdict needs.
	DaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error)
	// DeleteEdges removes the given edges.
	DeleteEdges(ctx context.Context, edgeIDs []int64) error
}
