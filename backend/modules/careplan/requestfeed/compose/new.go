package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/application"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// Database is the request feed's transaction; the root builds the feed
// source over it.
type Database = postgres.Database

type Dependencies struct {
	DB          *bun.DB
	Feed        func(Database) requestfeed.FeedSource
	FrontendURL string
	Now         func() time.Time
	NewToken    func() (string, string, error)
	HashToken   func(string) string
}

func New(dependencies Dependencies) (*requestfeed.Module, error) {
	if dependencies.DB == nil || dependencies.Feed == nil || dependencies.FrontendURL == "" || dependencies.Now == nil || dependencies.NewToken == nil || dependencies.HashToken == nil {
		return nil, errors.New("request feed compose: all dependencies are required")
	}
	database := func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return nil, errors.New("request feed compose: transaction is required")
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return tx, nil
			}
		}
		return nil, errors.New("request feed compose: unsupported transaction")
	}
	feed := dependencies.Feed(database)
	service, err := application.New(
		postgres.New(database, feed),
		postgres.NewAccessResolver(feed),
		tokenAdapter{newToken: dependencies.NewToken, hashToken: dependencies.HashToken},
		application.WithinAdmin(tenant.WithinAdmin),
		dependencies.FrontendURL,
		dependencies.Now,
	)
	if err != nil {
		return nil, err
	}
	return requestfeed.NewModule(engine{service: service}), nil
}

type tokenAdapter struct {
	newToken  func() (string, string, error)
	hashToken func(string) string
}

func (a tokenAdapter) New() (string, string, error) { return a.newToken() }
func (a tokenAdapter) Hash(raw string) string       { return a.hashToken(raw) }

type engine struct{ service *application.Service }

func (e engine) Status(ctx context.Context, tenantID, accountID int64) (requestfeed.Status, error) {
	return e.service.Status(ctx, tenantID, accountID)
}

func (e engine) Provision(ctx context.Context, tenantID, accountID int64) (requestfeed.Created, error) {
	return e.service.Provision(ctx, tenantID, accountID)
}

func (e engine) Rotate(ctx context.Context, tenantID, accountID int64) (requestfeed.Created, error) {
	return e.service.Rotate(ctx, tenantID, accountID)
}

func (e engine) ByToken(ctx context.Context, token string) (requestfeed.Feed, error) {
	return e.service.ByToken(ctx, token)
}
