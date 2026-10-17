package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// NewEngine serves the School Structure capability from the application
// service.
func NewEngine(service *Service) Engine {
	return Engine{service: service}
}

// Engine is the School Structure capability over the application service.
type Engine struct{ service *Service }

func (e Engine) ListGroups(ctx context.Context, limit int) ([]schoolstructure.Group, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, err
	}
	values, err := e.service.List(ctx, tenantID.Int64(), limit)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]schoolstructure.Group, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result, nil
}

func (e Engine) FindGroup(ctx context.Context, id int64) (schoolstructure.Group, error) {
	value, err := e.service.FindByID(ctx, id)
	return toPublic(value), MapError(err)
}

func (e Engine) ListGroupsByID(ctx context.Context, ids []int64) ([]schoolstructure.Group, error) {
	values, err := e.service.ListByIDs(ctx, ids)
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]schoolstructure.Group, 0, len(values))
	for _, value := range values {
		result = append(result, toPublic(value))
	}
	return result, nil
}

func toPublic(value domain.Group) schoolstructure.Group {
	return schoolstructure.Group{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Name: value.Name, RoomID: value.RoomID,
	}
}

func MapError(err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return schoolstructure.ErrGroupNotFound
	}
	return err
}
