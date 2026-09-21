package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// DemoAccessStore persists the demo accesses of the public demo (#3462).
// Every method expects the administrative transaction in ctx.
type DemoAccessStore interface {
	InsertDemoAccess(ctx context.Context, access domain.DemoAccess) (int64, error)
	FindDemoAccessByTokenHash(ctx context.Context, tokenHash string) (domain.DemoAccess, bool, error)
	RecordDemoAccessUse(ctx context.Context, id, accountID int64, usedAt time.Time) error
	// FindSchoolAdministrator returns the oldest active administrator of the school.
	FindSchoolAdministrator(ctx context.Context, tenantID int64) (accountID int64, found bool, err error)
	// LockDemoAccessEmail serialises the requests of one address until the
	// transaction ends, so two of them cannot both find no active access.
	LockDemoAccessEmail(ctx context.Context, email string) error
	// UnexpiredDemoAccessSchools lists the schools of the address's accesses
	// that have not expired at now.
	UnexpiredDemoAccessSchools(ctx context.Context, email string, now time.Time) ([]string, error)
}

// DemoSchools reaches the demo schools through their owner, Organisation &
// Tenancy, inside the caller's administrative transaction.
type DemoSchools interface {
	// PrepareDemoSchool returns the slug of the school a new access enters:
	// a school of its own that is queued for seeding, or the standing school.
	PrepareDemoSchool(ctx context.Context, schoolName, personName string) (slug string, err error)
	// DemoSchoolEntry reports the school's progress. A school that is
	// unknown, inactive or deleted is still preparing.
	DemoSchoolEntry(ctx context.Context, slug string) (domain.DemoSchoolEntry, error)
}

// DemoAccessTokens mints an opaque token and derives its fingerprint.
type DemoAccessTokens interface {
	NewToken() (raw, fingerprint string, err error)
	Fingerprint(raw string) string
}

// DemoSessions mints the session of a redeemed demo access. The account
// authentication of this module satisfies it.
type DemoSessions interface {
	IssueTokensForAuthenticatedAccount(ctx context.Context, accountID, tenantID int64, ipAddress, userAgent string) (string, string, error)
}

// DemoAdminTx runs fn inside an administrative transaction.
type DemoAdminTx func(ctx context.Context, fn func(context.Context) error) error
