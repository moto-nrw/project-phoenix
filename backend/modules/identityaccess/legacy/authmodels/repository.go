package authmodels

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// AccountRepository defines operations for managing accounts
type AccountRepository interface {
	base.CRUDRepository[*Account]
	FindByEmail(ctx context.Context, email string) (*Account, error)
}
