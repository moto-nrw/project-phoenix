package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"
)

type RequestShareRecords interface{ messaging.RequestShareRecords }
type RequestShareRecordsCallbacks struct {
	Create            func(pA context.Context, pB *ParentRequestShareEvent) error
	CurrentForStudent func(pA context.Context, pB int64) ([]*ParentRequestShareEvent, error)
}

func NewRequestShareRecords(c RequestShareRecordsCallbacks) RequestShareRecords {
	return requestShareRecordsBinding{c}
}

type requestShareRecordsBinding struct{ callbacks RequestShareRecordsCallbacks }

func (p requestShareRecordsBinding) Create(pA context.Context, pB *messaging.ParentRequestShareEvent) error {
	nativePB := messagingValuePointer(pB, messagingFromAppParentRequestShareEvent)
	result0 := p.callbacks.Create(pA, nativePB)
	if pB != nil && nativePB != nil {
		*pB = messagingToAppParentRequestShareEvent(*nativePB)
	}
	return result0
}
func (p requestShareRecordsBinding) CurrentForStudent(pA context.Context, pB int64) ([]*messaging.ParentRequestShareEvent, error) {
	result0, result1 := p.callbacks.CurrentForStudent(pA, pB)
	return messagingValueSlice(result0, func(row *ParentRequestShareEvent) *messaging.ParentRequestShareEvent {
		return messagingValuePointer(row, messagingToAppParentRequestShareEvent)
	}), result1
}
