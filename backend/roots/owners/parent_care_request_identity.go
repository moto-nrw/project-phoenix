package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentCareRequestReads binds the owner's decoded lookup to the workflow's
// three scalar identity facts, without exposing the owner's stored row.
func NewParentCareRequestReads(source interface {
	FindCareScheduleRequest(context.Context, int64, bool) (careplan.CareScheduleChangeRequest, error)
}) interface {
	FindIdentity(context.Context, int64) (int64, int64, string, error)
} {
	if source == nil {
		return nil
	}
	return parentCareRequestReads{source: careplanCompose.NewParentCareRequestQueries(source, func() error { return transactionruntime.RecordErrors{}.MissingRecordError("find by id") })}
}

type parentCareRequestReads struct {
	source carerequests.ParentCareRequestQueries
}

func (r parentCareRequestReads) FindIdentity(ctx context.Context, id int64) (int64, int64, string, error) {
	row, err := r.source.CareRequestIdentity(ctx, id)
	if err != nil {
		return 0, 0, "", err
	}
	if row == nil {
		return 0, 0, "", parentportal.ErrRequestSharingNotFound
	}
	return row.SubmittedBy, row.StudentID, row.RequestKind, nil
}
