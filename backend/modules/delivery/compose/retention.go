package compose

import (
	"context"
	"errors"

	deliveryContract "github.com/moto-nrw/project-phoenix/modules/delivery/contract"

	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/delivery/internal/application"
	"github.com/uptrace/bun"
)

// NewRetention composes the delivery retention of the nightly cleanup
// (#3955). It runs on the tenant transaction of the school in context and
// needs neither a provider nor the people directory.
func NewRetention() deliveryContract.Retention {
	store := postgres.New(postgres.RequireTenantDatabase, func(context.Context, func(context.Context, bun.IDB) error) error {
		return errors.New("delivery retention: no administrative transaction")
	})
	return application.NewRetention(store)
}
