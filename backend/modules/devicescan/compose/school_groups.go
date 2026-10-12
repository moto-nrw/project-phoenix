package compose

import (
	"context"

	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/contract"
)

// SchoolGroups is the group read port owned by this consumer.
type SchoolGroups interface {
	GetGroup(ctx context.Context, id int64) (*education.Group, error)
}
