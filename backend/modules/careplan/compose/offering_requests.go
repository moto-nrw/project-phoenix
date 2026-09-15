package compose

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/uptrace/bun"
)

// NewOfferingChangeRequestQueries binds the owner read without mutation dependencies.
func NewOfferingChangeRequestQueries(db *bun.DB, observe func(Observation)) (careplan.OfferingChangeRequestQuery, error) {
	if db == nil || observe == nil {
		return nil, errors.New("care plan offering request queries: database and observer are required")
	}
	return offeringChangeRequestQueries{service: application.New(postgres.New(carePlanDatabase(db)), observe)}, nil
}

type offeringChangeRequestQueries struct {
	service *application.Service
}

func (q offeringChangeRequestQueries) ListOfferingChanges(ctx context.Context, filter careplan.OfferingChangeFilter) ([]careplan.OfferingChangeRequest, error) {
	values, err := q.service.ListOfferingChanges(ctx, domain.OfferingChangeFilter{
		IDs: filter.IDs, StudentID: filter.StudentID, StudentIDs: filter.StudentIDs, Statuses: filter.Statuses,
		UrgentOnly: filter.UrgentOnly, UrgentDate: filter.UrgentDate, BeforeInstant: filter.BeforeInstant,
		BeforeID: filter.BeforeID, Limit: filter.Limit, LockForUpdate: filter.LockForUpdate, Order: filter.Order,
	})
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]careplan.OfferingChangeRequest, 0, len(values))
	for _, value := range values {
		result = append(result, changeToPublic(value))
	}
	return result, nil
}
