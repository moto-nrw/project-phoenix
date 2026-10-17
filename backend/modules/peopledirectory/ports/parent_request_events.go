package ports

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// ParentRequestEventRecords advertises the native owner facet alongside the
// retained callers, without adding a foreign import to the model contract.
type ParentRequestEventRecords interface {
	users.ParentRequestEventRepository
	peopledirectory.ParentRequestEvents
}
