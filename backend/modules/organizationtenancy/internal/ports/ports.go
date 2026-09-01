package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

type Store interface {
	Create(context.Context, domain.CreateOrganization) (domain.Organization, domain.OperationStats, error)
	Update(context.Context, domain.UpdateOrganization) (domain.Organization, domain.OperationStats, error)
	FindByID(context.Context, int64, string) (domain.Organization, bool, domain.OperationStats, error)
	FindBySlug(context.Context, string) (domain.Organization, bool, domain.OperationStats, error)
	List(context.Context) ([]domain.Organization, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Organization, domain.OperationStats, error)
	CountByIDs(context.Context, []int64) (int, domain.OperationStats, error)
	CountNonDeletedSchools(context.Context, int64) (int, domain.OperationStats, error)
	SoftDelete(context.Context, int64) (domain.OperationStats, error)
	Restore(context.Context, int64) (domain.OperationStats, error)
}

type Transaction interface {
	RunAdmin(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
