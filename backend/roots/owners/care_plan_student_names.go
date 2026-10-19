package owners

import (
	"context"

	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// carePlanStudentNames serves Care Plan's companion-name port from a People
// Directory read the root adapts.
type carePlanStudentNames func(context.Context, []int64) ([]carePlanPorts.StudentName, error)

func (f carePlanStudentNames) ListStudentNamesByID(ctx context.Context, ids []int64) ([]carePlanPorts.StudentName, error) {
	return f(ctx, ids)
}
