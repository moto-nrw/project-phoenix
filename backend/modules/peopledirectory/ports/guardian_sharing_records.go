package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

type FamilyProtectionRecords interface {
	peopledirectory.GuardianFamilyProtectionReads
	Create(context.Context, *peopledirectory.GuardianFamilyProtectionRecord) error
}
type ParentRequestShareRecords interface {
	peopledirectory.ParentRequestShares
}
