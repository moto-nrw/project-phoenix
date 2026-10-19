package compose

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	identityPorts "github.com/moto-nrw/project-phoenix/modules/identityaccess/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

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

// composeDemoAccess composes the demo access of the public demo (#3462); the
// token functions are the token adapter's opaque capability token, so only a
// SHA-256 fingerprint is stored. Callers supply the unit of work on the
// request context, as every public route does.
func composeDemoAccess(db *bun.DB, sessions ports.DemoSessions, deps *identityPorts.DemoDependencies) (*identityaccess.DemoAccess, error) {
	return application.NewDemoAccessBinding(db != nil, sessions, deps, postgres.NewStore(db), tenant.WithinAdmin)
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
	return postgres.ExpireDemoAccessesBinding(ctx, e.db, e.now, tenant.WithAdminTx)
}
