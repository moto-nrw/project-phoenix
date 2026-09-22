package services

import (
	"context"
	"time"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/uptrace/bun"
)

// DemoAccessExpiry is the Identity & Access capability the demo process runs
// (#3470): it deletes demo accesses 14 days after their last use and names
// the demo schools that no access enters any more.
type DemoAccessExpiry interface {
	ExpireDemoAccesses(ctx context.Context) (deleted int, orphanedSchoolSlugs []string, err error)
}

// NewDemoAccessExpiry composes the expiry on the demo process's own
// connection; now is the clock it reads.
func NewDemoAccessExpiry(db bun.IDB, now func() time.Time) (DemoAccessExpiry, error) {
	return identityaccessCompose.NewDemoAccessExpiry(db, now)
}
