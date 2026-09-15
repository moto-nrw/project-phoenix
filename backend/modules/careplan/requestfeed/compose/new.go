package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/requestfeed/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/parentrequestfeedprojection"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Dependencies struct {
	DB          *bun.DB
	FrontendURL string
	Now         func() time.Time
	NewToken    func() (string, string, error)
	HashToken   func(string) string
}

func New(dependencies Dependencies) (*requestfeed.Module, error) {
	if dependencies.DB == nil || dependencies.FrontendURL == "" || dependencies.Now == nil || dependencies.NewToken == nil || dependencies.HashToken == nil {
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
	service, err := application.New(
		postgres.New(database),
		accessAdapter{database: database},
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

type accessAdapter struct {
	database func(context.Context) (bun.IDB, error)
}

func (a accessAdapter) Resolve(ctx context.Context, tenantID, accountID int64) (domain.Access, error) {
	db, err := a.database(ctx)
	if err != nil {
		return domain.Access{}, err
	}
	access, found, err := parentrequestfeedprojection.ResolveAccess(ctx, db, tenantID, accountID)
	if err != nil || !found {
		return domain.Access{}, err
	}
	return domain.Access{
		Active: true, GeneralRequests: access.GeneralRequests, EnrollmentRequests: access.EnrollmentRequests,
		SchoolName: access.SchoolName, Subdomain: access.Subdomain,
	}, err
}

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
