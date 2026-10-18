package api

import (
	"context"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func BindParentRequestShareRecords(source directory.ParentRequestShares) parentportal.RequestShareRecords {
	if source == nil {
		return nil
	}
	return parentportal.NewRequestShareRecords(parentportal.RequestShareRecordsCallbacks{Create: func(pA context.Context, pB *parentportal.ParentRequestShareEvent) error {
		ownerPB := parentMessagingPointer(pB, parentMessagingToOwnerParentRequestShareEvent)
		result0 := source.AppendParentRequestShare(pA, ownerPB)
		if pB != nil && ownerPB != nil {
			*pB = parentMessagingFromOwnerParentRequestShareEvent(*ownerPB)
		}
		return result0
	},
		CurrentForStudent: func(pA context.Context, pB int64) ([]*parentportal.ParentRequestShareEvent, error) {
			result0, result1 := source.ParentRequestSharesForStudent(pA, pB)
			return parentMessagingSlice(result0, func(row *directory.ParentRequestShareRecord) *parentportal.ParentRequestShareEvent {
				return parentMessagingPointer(row, parentMessagingFromOwnerParentRequestShareEvent)
			}), result1
		},
	})
}
