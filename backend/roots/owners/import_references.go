package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// NewReferences binds import name matching to the native owner queries.
// The 1000-group bound preserves the former import preload contract.
func NewReferences(groups schoolstructure.GroupListing, rooms facilities.Query) dataimport.References {
	return dataimport.References{
		Groups: NewGroupReferenceReader(groups),
		Rooms:  NewRoomReferenceReader(rooms),
	}
}
