package compose

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// NewDemoSchoolQueue composes the demo process's side of the queue on its
// privileged connection, like NewDemoSchools.
func NewDemoSchoolQueue(db *bun.DB) (*organizationtenancy.DemoSchoolQueue, error) {
	if db == nil {
		return nil, fmt.Errorf("demo school queue requires a database")
	}
	return organizationtenancy.NewDemoSchoolQueue(demoQueueEngine{store: postgres.NewDemoStateStore(db)}), nil
}

type demoQueueEngine struct{ store *postgres.DemoStateStore }

func (e demoQueueEngine) ReleaseDemoSchoolOrders(ctx context.Context) error {
	return e.store.ReleaseClaims(ctx)
}

func (e demoQueueEngine) ClaimDemoSchoolOrder(ctx context.Context) (*organizationtenancy.DemoSchoolOrder, error) {
	order, err := e.store.Claim(ctx)
	if err != nil || order == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolOrder{
		Slug: order.Name, SchoolName: order.SchoolName, PersonName: order.PersonName, Attempts: order.Attempts, Seeded: order.Seeded,
	}, nil
}

func (e demoQueueEngine) FinishDemoSchoolOrder(ctx context.Context, slug string, visitorAccountID int64) error {
	return e.store.Finish(ctx, slug, visitorAccountID)
}

func (e demoQueueEngine) FailDemoSchoolOrder(ctx context.Context, slug string, maxAttempts int) (bool, error) {
	return e.store.Fail(ctx, slug, maxAttempts)
}

func (e demoQueueEngine) ReadyDemoSchools(ctx context.Context) ([]string, error) {
	return e.store.ReadyNames(ctx)
}

// NewDemoSchoolOrders composes the serving backend's side. Every call needs
// the caller's administrative transaction in ctx. random is the root's secure
// source; it makes the slug suffix unguessable.
func NewDemoSchoolOrders(random io.Reader) *organizationtenancy.DemoSchoolOrders {
	if random == nil {
		panic("demo school orders require a random source")
	}
	store := postgres.NewDemoOrderStore(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("organization tenancy postgres: transaction is required")
		}
		tx, ok := transaction.(bun.Tx)
		if !ok {
			return nil, fmt.Errorf("organization tenancy postgres: unsupported transaction %T", transaction)
		}
		return tx, nil
	})
	return organizationtenancy.NewDemoSchoolOrders(demoOrderEngine{store: store, random: random})
}

type demoOrderEngine struct {
	store  *postgres.DemoOrderStore
	random io.Reader
}

const demoSlugAlphabet = "abcdefghijkmnpqrstuvwxyz23456789"

func (e demoOrderEngine) OrderDemoSchool(ctx context.Context, schoolName, personName string) (string, error) {
	suffix := make([]byte, 6)
	if _, err := io.ReadFull(e.random, suffix); err != nil {
		return "", fmt.Errorf("demo school slug: %w", err)
	}
	for i, b := range suffix {
		suffix[i] = demoSlugAlphabet[int(b)%len(demoSlugAlphabet)]
	}
	slug := organizationtenancy.DemoSchoolSlug(schoolName, string(suffix))
	if err := e.store.Enqueue(ctx, slug, schoolName, personName); err != nil {
		return "", err
	}
	return slug, nil
}

func (e demoOrderEngine) DemoSchoolProgress(ctx context.Context, slug string) (*organizationtenancy.DemoSchoolProgress, error) {
	progress, err := e.store.Progress(ctx, slug)
	if err != nil || progress == nil {
		return nil, err
	}
	return &organizationtenancy.DemoSchoolProgress{
		Status: progress.Status, SchoolID: progress.TenantID, VisitorAccountID: progress.VisitorAccountID,
	}, nil
}

func (e demoQueueEngine) ReturnDemoSchoolOrder(ctx context.Context, slug string) error {
	return e.store.Return(ctx, slug)
}
