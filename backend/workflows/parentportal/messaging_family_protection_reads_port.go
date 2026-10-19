package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type FamilyProtectionReads interface {
	messaging.FamilyProtectionReads
}
type FamilyProtectionReadsCallbacks struct {
	CurrentForStudents func(ctx context.Context, studentIDs []int64) (map[int64]*FamilyProtectionEvent, error)
}

func NewFamilyProtectionReads(c FamilyProtectionReadsCallbacks) FamilyProtectionReads {
	return familyProtectionReadsBinding{c}
}

type familyProtectionReadsBinding struct {
	callbacks FamilyProtectionReadsCallbacks
}

func (p familyProtectionReadsBinding) CurrentForStudents(ctx context.Context, studentIDs []int64) (map[int64]*messaging.FamilyProtectionEvent, error) {
	result0, result1 := p.callbacks.CurrentForStudents(ctx, studentIDs)
	return messagingValueMap(result0, func(row *FamilyProtectionEvent) *messaging.FamilyProtectionEvent {
		return messagingValuePointer(row, messagingToAppFamilyProtectionEvent)
	}), result1
}
