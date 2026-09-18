package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentCompanionEdge is one stored "läuft mit" link as the composition root
// hands it over. Care Plan owns the row; this is the shape the directory needs
// to reconcile a narrowed departure plan against it.
type StudentCompanionEdge struct {
	ID                 int64
	StudentID          int64
	CompanionStudentID int64
	Weekday            int
}

// StudentCompanions is the Care Plan seam the directory's write path needs.
// The composition root binds modules/careplan behind it; the directory never
// reaches users.student_companions itself.
type StudentCompanions interface {
	ListCompanionEdgesForStudent(ctx context.Context, studentID int64) ([]StudentCompanionEdge, error)
	CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error)
	DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error
}

// studentCompanions adapts the composition root's public-typed seam to the
// module's internal port.
type studentCompanions struct{ seam StudentCompanions }

func (a studentCompanions) ListForStudent(ctx context.Context, studentID int64) ([]domain.CompanionEdge, error) {
	edges, err := a.seam.ListCompanionEdgesForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]domain.CompanionEdge, 0, len(edges))
	for _, edge := range edges {
		result = append(result, domain.CompanionEdge(edge))
	}
	return result, nil
}

func (a studentCompanions) DaysCoveredExcluding(
	ctx context.Context,
	studentIDs []int64,
	excludeID int64,
) (map[int64]map[string]bool, error) {
	return a.seam.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
}

func (a studentCompanions) DeleteEdges(ctx context.Context, edgeIDs []int64) error {
	return a.seam.DeleteCompanionEdges(ctx, edgeIDs)
}
