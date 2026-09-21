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
	// FindActiveDemoAccessByEmail returns the newest access of the address
	// that has not expired at now. It serialises requests of one address.
	FindActiveDemoAccessByEmail(ctx context.Context, email string, now time.Time) (domain.DemoAccess, bool, error)
	FindDemoAccessByTokenHash(ctx context.Context, tokenHash string) (domain.DemoAccess, bool, error)
	RecordDemoAccessUse(ctx context.Context, id, accountID int64, usedAt time.Time) error
	// FindSchoolAdministrator returns the oldest active administrator of the school.
	FindSchoolAdministrator(ctx context.Context, tenantID int64) (accountID int64, found bool, err error)
}

// DemoSchools resolves the demo school through its owner, Organisation &
// Tenancy. A school that does not exist yet, is inactive or deleted is not found.
type DemoSchools interface {
	FindDemoSchool(ctx context.Context, slug string) (tenantID int64, found bool, err error)
}

// DemoAccessMail sends the two mails of the public demo (#3465). Sending is
// asynchronous; a failed mail never fails the request.
type DemoAccessMail interface {
	// SendDemoAccessLink mails the prospect the way back into the demo.
	SendDemoAccessLink(ctx context.Context, access domain.DemoAccess, entryURL string)
	// SendDemoLead tells the team about a new demo access.
	SendDemoLead(ctx context.Context, access domain.DemoAccess)
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
