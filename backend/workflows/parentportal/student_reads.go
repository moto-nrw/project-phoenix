package parentportal

import (
	"context"

	"github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"
)

// StudentSnapshot is defined from the workflow's genuine native DTO.
type StudentSnapshot care.PortalStudent

type StudentReadCallbacks struct {
	Find func(context.Context, int64) (*StudentSnapshot, error)
	Lock func(context.Context, int64) (*StudentSnapshot, error)
}

// NewStudentReads binds public-root callbacks to the native workflow port.
type StudentReads interface {
	FindByID(context.Context, int64) (*care.PortalStudent, error)
	FindByIDForUpdate(context.Context, int64) (*care.PortalStudent, error)
}

func NewStudentReads(callbacks StudentReadCallbacks) StudentReads {
	return portalStudentReads{callbacks}
}

type portalStudentReads struct{ callbacks StudentReadCallbacks }

func (r portalStudentReads) FindByID(ctx context.Context, id int64) (*care.PortalStudent, error) {
	value, err := r.callbacks.Find(ctx, id)
	return (*care.PortalStudent)(value), err
}
func (r portalStudentReads) FindByIDForUpdate(ctx context.Context, id int64) (*care.PortalStudent, error) {
	value, err := r.callbacks.Lock(ctx, id)
	return (*care.PortalStudent)(value), err
}

// WithStudentPerson preserves an already-loaded person relation.
func WithStudentPerson(value *StudentSnapshot, person *PersonSnapshot) *StudentSnapshot {
	if value != nil {
		value.Person = (*care.Person)(person)
	}
	return value
}
