package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// Store is the persistence port over users.persons. Reads honour the tenant
// in context when one is present; the platform-wide CountByTenant is the
// only deliberate exception and requires an admin transaction.
type Store interface {
	Create(context.Context, domain.CreatePerson) (domain.Person, domain.OperationStats, error)
	Update(context.Context, domain.UpdatePerson) (domain.Person, domain.OperationStats, error)
	FindByID(ctx context.Context, id int64, lock string) (domain.Person, bool, domain.OperationStats, error)
	FindByAccount(context.Context, int64) (domain.Person, bool, domain.OperationStats, error)
	FindByTag(context.Context, string) (domain.Person, bool, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]domain.Person, domain.OperationStats, error)
	ListByAccounts(context.Context, []int64) ([]domain.Person, domain.OperationStats, error)
	Search(context.Context, domain.Filter) ([]domain.Person, domain.OperationStats, error)
	CountByTenant(context.Context) (map[int64]int, domain.OperationStats, error)
	SoftDelete(context.Context, int64) (domain.OperationStats, error)
	SetAccount(ctx context.Context, personID int64, accountID *int64) (domain.OperationStats, error)
	SetTag(ctx context.Context, personID int64, tagID *string) (domain.OperationStats, error)
	LockHeldTags(context.Context, []int64) ([]domain.ReleasedTag, domain.OperationStats, error)
	ClearTags(context.Context, []int64) (domain.OperationStats, error)
	RestoreTag(ctx context.Context, personID int64, tagID string) (bool, domain.OperationStats, error)
}

type Transaction interface {
	// RunWrite joins the caller's transaction or opens one for the tenant
	// in context.
	RunWrite(context.Context, func(context.Context) error) error
	// RunRead joins the caller's transaction, else opens a tenant
	// transaction, else an admin transaction for cross-tenant readers.
	RunRead(context.Context, func(context.Context) error) error
	// RunSavepoint runs fn inside a savepoint of the current transaction so
	// a rejected statement leaves the outer transaction usable.
	RunSavepoint(context.Context, func(context.Context) error) error
	// RunAdminRead opens a separate admin transaction for the platform-wide
	// reads that must see every tenant's rows.
	RunAdminRead(context.Context, func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
