package ports

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type FamilyProtectionRecords interface {
	users.FamilyProtectionEventRepository
	peopledirectory.GuardianFamilyProtectionReads
}
type ParentRequestShareRecords interface {
	users.ParentRequestShareEventRepository
	peopledirectory.ParentRequestShares
}
