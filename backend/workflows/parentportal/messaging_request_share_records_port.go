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
	return p.callbacks.Create(pA, pB)
}
func (p requestShareRecordsBinding) CurrentForStudent(pA context.Context, pB int64) ([]*messaging.ParentRequestShareEvent, error) {
	return p.callbacks.CurrentForStudent(pA, pB)
}
