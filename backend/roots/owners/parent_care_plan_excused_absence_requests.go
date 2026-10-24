package owners

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentExcusedAbsenceRequests(source careplan.ExcusedAbsenceRequests) parentportal.ExcusedAbsenceRequests {
	if source == nil {
		return nil
	}
	return parentportal.NewExcusedAbsenceRequests(parentportal.ExcusedAbsenceRequestsCallbacks{
		EditRequest: func(ctx context.Context, input parentportal.ExcusedRequestEditInputSnapshot) (*parentportal.ExcusedAbsenceRequestSnapshot, error) {
			value, err := source.EditRequest(ctx, parentCarePlanToOwnerExcusedRequestEditInput(input))
			return parentCarePlanPointer(value, parentCarePlanFromOwnerExcusedAbsenceRequest), err
		},
		ListForStudent: func(ctx context.Context, id int64, since time.Time) ([]*parentportal.ExcusedAbsenceRequestSnapshot, error) {
			value, err := source.ListForStudent(ctx, id, since)
			return parentCarePlanSlice(value, func(value *careplan.ExcusedAbsenceRequest) *parentportal.ExcusedAbsenceRequestSnapshot {
				return parentCarePlanPointer(value, parentCarePlanFromOwnerExcusedAbsenceRequest)
			}), err
		},
		Submit: func(ctx context.Context, input parentportal.ExcusedRequestCreateInputSnapshot) (*parentportal.ExcusedAbsenceRequestSnapshot, error) {
			value, err := source.Submit(ctx, parentCarePlanToOwnerExcusedRequestCreateInput(input))
			return parentCarePlanPointer(value, parentCarePlanFromOwnerExcusedAbsenceRequest), err
		},
	})
}
