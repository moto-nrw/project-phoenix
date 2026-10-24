package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// DemoSchools reaches the demo schools through Organisation & Tenancy inside
// the flow's administrative transaction (#3463). PrepareDemoSchool returns
// the slug a new access enters: a queued school of its own or the standing
// school, or identityaccess.ErrDemoCapacityReached when no further school
// may be queued (#3466). DemoSchoolEntry reports "preparing" until the school
// is seeded, exists and is active. MarkDemoSchoolUsed notes an entry into the
// school.
type DemoSchools interface {
	PrepareDemoSchool(ctx context.Context, schoolName, firstName, lastName string) (slug string, err error)
	DemoSchoolEntry(ctx context.Context, slug string) (identityaccess.DemoSchoolEntry, error)
	MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error
	// ReplaceDemoSchool hides the school and queues a fresh one with the
	// same names, returning its slug (#3470). The standing school reports
	// identityaccess.ErrDemoAccessInvalid: it is shared and never restarts.
	ReplaceDemoSchool(ctx context.Context, slug, schoolName, firstName, lastName string) (newSlug string, err error)
}
