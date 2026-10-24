package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/tenant"
)

type Transaction struct{}

func (Transaction) Require(ctx context.Context) error {
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return err
	}
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("student presence: transaction is required")
	}
	return nil
}
