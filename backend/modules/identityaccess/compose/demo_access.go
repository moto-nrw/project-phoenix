package compose

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// DemoDependencies are the Organisation & Tenancy facts the demo flow needs
// when the Identity & Access module is composed for the demo environment.
type DemoDependencies struct {
	Schools     identityPorts.DemoSchools
	Mail        identityaccess.DemoAccessMail
	NewToken    func() (raw, fingerprint string, err error)
	Fingerprint func(raw string) string
	// OperatorWithoutSecondFactor lets the operator sign in with the password
	// alone (#3460). The demo's mail lock drops the sign-in code, so the demo
	// process that seeds every demo school could never finish the second
	// factor; the demo host keeps the operator surface off the internet.
	OperatorWithoutSecondFactor bool
}

// composeDemoAccess composes the demo access of the public demo (#3462); the
// token functions are the token adapter's opaque capability token, so only a
// SHA-256 fingerprint is stored. Callers supply the unit of work on the
// request context, as every public route does.
func composeDemoAccess(db *bun.DB, sessions ports.DemoSessions, deps *DemoDependencies) (*identityaccess.DemoAccess, error) {
	if deps == nil {
		return nil, nil
	}
	if db == nil || sessions == nil || deps.Schools == nil || deps.Mail == nil || deps.NewToken == nil || deps.Fingerprint == nil {
		return nil, errors.New("identity access compose: every demo access dependency is required")
	}
	flows, err := application.NewDemoAccess(application.DemoAccessDependencies{
		Sessions: sessions, Store: newStore(db), Schools: application.DemoSchoolsPort{Schools: deps.Schools},
		Tokens:  application.DemoAccessTokens{Mint: deps.NewToken, FingerprintFunc: deps.Fingerprint},
		Mail:    application.DemoAccessMail{Mail: deps.Mail},
		AdminTx: tenant.WithinAdmin,
	})
	if err != nil {
		return nil, err
	}
	return identityaccess.NewDemoAccess(application.DemoAccessEngine{Flows: flows}), nil
}

// NewDemoAccessExpiry composes the demo process's expiry (#3470) on that
// process's own connection; now is the clock the process injects.
func NewDemoAccessExpiry(db bun.IDB, now func() time.Time) (*identityaccess.DemoAccessExpiry, error) {
	if db == nil {
		return nil, errors.New("identity access compose: demo access expiry requires a database")
	}
	if now == nil {
		return nil, errors.New("identity access compose: demo access expiry requires a clock")
	}
	return identityaccess.NewDemoAccessExpiry(demoAccessExpiryEngine{db: db, now: now}), nil
}

// demoAccessExpiryEngine deletes demo accesses 14 days after their last use
// (#3470). A use extends the access, so an access past its end has not been
// used for its whole lifetime; deleting it removes the prospect's contact
// data. The schools left without any access go back to the caller, which
// hides them through their owner.
type demoAccessExpiryEngine struct {
	db  bun.IDB
	now func() time.Time
}

func (e demoAccessExpiryEngine) ExpireDemoAccesses(ctx context.Context) (int, []string, error) {
	var count int
	var slugs []string
	err := tenant.WithAdminTx(ctx, e.db, func(txCtx context.Context, tx bun.Tx) error {
		var err error
		count, slugs, err = postgres.NewDemoAccessExpiryStore(tx).DeleteExpiredDemoAccesses(txCtx, e.now())
		return err
	})
	return count, slugs, err
}
