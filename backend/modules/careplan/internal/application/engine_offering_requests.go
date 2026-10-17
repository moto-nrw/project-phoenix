package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

// NewOfferingChangeRequestQueries serves the offering change reads over
// service.
func NewOfferingChangeRequestQueries(service *Service) careplan.OfferingChangeRequestQuery {
	return offeringChangeRequestQueries{service: service}
}

type offeringChangeRequestQueries struct {
	service *Service
}

func (q offeringChangeRequestQueries) ListOfferingChanges(ctx context.Context, filter careplan.OfferingChangeFilter) ([]careplan.OfferingChangeRequest, error) {
	values, err := q.service.ListOfferingChanges(ctx, domain.OfferingChangeFilter{
		IDs: filter.IDs, StudentID: filter.StudentID, StudentIDs: filter.StudentIDs, Statuses: filter.Statuses,
		UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate, BeforeInstant: filter.BeforeInstant,
		BeforeID: filter.BeforeID, Limit: filter.Limit, LockForUpdate: filter.LockForUpdate, Order: filter.Order,
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]careplan.OfferingChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, changeToPublic(value))
	}
	return result, nil
}
