package owners

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentDataRequestReads retains the original owner queries and answers a
// missing or decided request with the workflow's own values, which the parent
// routes render as 404 and 409.
func NewParentDataRequestReads(source careplan.CareRequestsQuery) parentportal.DataRequestReads {
	return parentportal.NewDataRequestReads(parentportal.DataRequestReadCallbacks{
		Find: func(ctx context.Context, id int64, lock bool) (*parentportal.DataRequestSnapshot, error) {
			row, err := source.FindStudentDataRequest(ctx, id, lock)
			if err != nil {
				return nil, mapParentDataRequestError(err)
			}
			value := parentportal.DataRequestSnapshot(row)
			return &value, nil
		},
		Pending: source.HasPendingStudentDataRequest,
		List: func(ctx context.Context, id int64, statuses []string, limit int, visible bool) ([]*parentportal.DataRequestSnapshot, error) {
			rows, err := source.ListStudentDataRequests(ctx, careplan.StudentDataRequestFilter{StudentID: id, Statuses: statuses, Limit: limit, ParentVisible: visible})
			if err != nil {
				return nil, err
			}
			result := make([]*parentportal.DataRequestSnapshot, len(rows))
			for i, row := range rows {
				value := parentportal.DataRequestSnapshot(row)
				result[i] = &value
			}
			return result, nil
		},
		NotFoundError:   func() error { return parentportal.ErrMasterDataRequestNotFound },
		NotPendingError: func() error { return parentportal.ErrMasterDataRequestNotPending },
	})
}

func NewParentDataRequestCommands(source careplan.CareRequestsCommand) parentportal.DataRequestCommands {
	if source == nil {
		return nil
	}
	return parentportal.NewDataRequestCommands(parentportal.DataRequestCommandCallbacks{
		Create: func(ctx context.Context, value parentportal.DataRequestSnapshot) (parentportal.DataRequestSnapshot, error) {
			created, err := source.CreateStudentDataRequest(ctx, careplan.StudentDataChangeRequest(value))
			return parentportal.DataRequestSnapshot(created), err
		},
		Update: func(ctx context.Context, id int64, value json.RawMessage) error {
			return mapParentDataRequestError(source.UpdatePendingStudentDataRequest(ctx, id, value))
		},
	})
}

// mapParentDataRequestError maps Care Plan's missing and decided request to
// the workflow's values; the staff review keeps its own mapping.
func mapParentDataRequestError(err error) error {
	switch {
	case errors.Is(err, careplan.ErrStudentDataRequestNotFound):
		return parentportal.ErrMasterDataRequestNotFound
	case errors.Is(err, careplan.ErrStudentDataRequestNotPending):
		return parentportal.ErrMasterDataRequestNotPending
	default:
		return err
	}
}
