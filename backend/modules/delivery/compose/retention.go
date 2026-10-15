package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewRetention composes the delivery retention of the nightly cleanup
// (#3955). It runs on the tenant transaction of the school in context and
// needs neither a provider nor the people directory.
func NewRetention() delivery.Retention {
	store := postgres.New(tenantDatabase, func(context.Context, func(context.Context, bun.IDB) error) error {
		return errors.New("delivery retention: no administrative transaction")
	})
	return retention{retention: application.NewRetention(store)}
}

type retention struct{ retention *application.Retention }

func (r retention) DeleteExpiredDeliveries(ctx context.Context) (int, int, error) {
	tenantID, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("delivery retention: tenant is required: %w", err)
	}
	result, err := r.retention.DeleteExpired(ctx, tenantID.Int64())
	if err != nil {
		return 0, 0, err
	}
	return result.Total(), int(domain.RetentionWindow.Hours() / 24), nil
}
