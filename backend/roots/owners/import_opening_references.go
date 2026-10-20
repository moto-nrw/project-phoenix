package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

func NewOpeningReferences(membership schoolmembership.Capability, people peopledirectory.Capability, ledger workforce.Capability) dataimport.OpeningReferences {
	return dataimport.OpeningReferences{
		Staff:    NewOpeningStaffReader(membership, people),
		Hours:    NewOpeningStaffIDsReader(ledger),
		Vacation: NewOpeningYearsReader(ledger),
	}
}
