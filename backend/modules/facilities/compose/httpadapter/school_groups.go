package httpadapter

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
)

// SchoolGroups is the group read port owned by this consumer.
type SchoolGroups interface {
	GetGroupsByIDs(ctx context.Context, ids []int64) (map[int64]*education.Group, error)
}
