package owners

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// NewParentDataRequestReads retains the original owner queries and the two
// distinct missing-row mappings of unlocked and locked reads.
func NewParentDataRequestReads(source careplan.CareRequestsQuery) parentportal.DataRequestReads {
	return parentportal.NewDataRequestReads(parentportal.DataRequestReadCallbacks{
		Find: func(ctx context.Context, id int64, lock bool) (*parentportal.DataRequestSnapshot, error) {
			row, err := source.FindStudentDataRequest(ctx, id, lock)
			if err != nil {
				if lock {
					return nil, mapStudentDataRequestError(err)
				}
				if errors.Is(err, careplan.ErrStudentDataRequestNotFound) {
					return nil, RetainedNotFound("find by id")
				}
				return nil, err
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
		NotFoundError:   func() error { return mapStudentDataRequestError(careplan.ErrStudentDataRequestNotFound) },
		NotPendingError: func() error { return mapStudentDataRequestError(careplan.ErrStudentDataRequestNotPending) },
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
			return mapStudentDataRequestError(source.UpdatePendingStudentDataRequest(ctx, id, value))
		},
	})
}
