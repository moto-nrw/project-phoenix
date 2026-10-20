package compose

import (
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
	database := postgres.NewDatabase(tenant.TransactionFromContext)
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
	return requestfeed.NewModule(service), nil
}

type tokenAdapter struct {
	newToken  func() (string, string, error)
	hashToken func(string) string
}

func (a tokenAdapter) New() (string, string, error) { return a.newToken() }
func (a tokenAdapter) Hash(raw string) string       { return a.hashToken(raw) }
