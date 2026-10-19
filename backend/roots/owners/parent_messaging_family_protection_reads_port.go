package owners

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentFamilyProtectionReads(source directory.GuardianFamilyProtectionReads) parentportal.FamilyProtectionReads {
	if source == nil {
		return nil
	}
	return parentportal.NewFamilyProtectionReads(parentportal.FamilyProtectionReadsCallbacks{CurrentForStudents: func(ctx context.Context, studentIDs []int64) (map[int64]*parentportal.FamilyProtectionEvent, error) {
		result0, result1 := source.GuardianFamilyProtectionRecords(ctx, studentIDs)
		return parentMessagingMap(result0, func(row *directory.GuardianFamilyProtectionRecord) *parentportal.FamilyProtectionEvent {
			return ParentMessagingPointer(row, parentMessagingFromOwnerFamilyProtectionEvent)
		}), result1
	},
	})
}
