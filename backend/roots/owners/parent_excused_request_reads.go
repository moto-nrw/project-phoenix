package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentExcusedRequestReads binds the absence request read of the parent
// request sharing. Like the other request bindings, a missing row answers
// with the workflow's not-found.
func NewParentExcusedRequestReads(source ExcusedRequestReads) ExcusedRequestReads {
	return parentExcusedRequestReads{source: source}
}

type parentExcusedRequestReads struct {
	source ExcusedRequestReads
}

func (r parentExcusedRequestReads) FindByID(ctx context.Context, id any) (*absencerecords.ExcusedAbsenceRequest, error) {
	row, err := r.source.FindByID(ctx, id)
	if errors.Is(err, absencerecords.ErrExcusedRequestNotFound) {
		return nil, parentportal.ErrRequestSharingNotFound
	}
	return row, err
}
