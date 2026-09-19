package usercontext

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type VisitReader interface {
	ListVisits(context.Context, studentpresence.VisitFilter) ([]studentpresence.Visit, error)
}
