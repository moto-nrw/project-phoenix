package usercontext

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

type SSEPresence interface {
	ListSSEGroups(context.Context) ([]studentpresence.LiveGroup, error)
	GetStaffActiveGroupIDs(context.Context, int64) ([]int64, error)
}
