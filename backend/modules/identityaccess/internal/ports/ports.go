// Package ports declares the persistence and transaction seams the Identity &
// Access application needs. Composition binds them.
package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// Store is the persistence port over the platform account table and the
// tenant-scoped mapping and role tables.
type Store interface {
	FindAccount(ctx context.Context, id int64) (domain.Account, bool, domain.OperationStats, error)
	FindAccountByEmail(ctx context.Context, email string) (domain.Account, bool, domain.OperationStats, error)
	// EnsureActiveTenantMapping inserts the account's mapping for the tenant or
	// reactivates an existing one, clearing its deactivation.
	EnsureActiveTenantMapping(ctx context.Context, accountID, tenantID int64) (domain.OperationStats, error)
	// FindRoleByName resolves a role visible to the tenant, preferring the
	// tenant's own role over the system role of the same name.
	FindRoleByName(ctx context.Context, name string, tenantID int64) (int64, bool, domain.OperationStats, error)
	// AssignAccountRole assigns the role for the tenant once and reports
	// whether this call created the assignment; an existing assignment is
	// left untouched.
	AssignAccountRole(ctx context.Context, accountID, roleID, tenantID int64) (bool, domain.OperationStats, error)
}

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction, else an admin transaction for tenantless readers.
	RunRead(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
