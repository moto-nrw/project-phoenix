package api

import (
	"context"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
)

// carePlanStudentNames serves Care Plan's companion-name port from a People
// Directory read the root adapts.
type carePlanStudentNames func(context.Context, []int64) ([]carePlanCompose.StudentName, error)

func (f carePlanStudentNames) ListStudentNamesByID(ctx context.Context, ids []int64) ([]carePlanCompose.StudentName, error) {
	return f(ctx, ids)
}
