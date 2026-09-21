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
	// RecordDemoAccessUse notes one redemption, the account it signed in and
	// the school's parent of the role parent (#3468); a zero parent keeps
	// the one noted before.
	RecordDemoAccessUse(ctx context.Context, id, accountID, parentAccountID int64, usedAt time.Time) error
	// ReplaceDemoAccountRole makes the system role the only role of the
	// visitor's account in its demo school (#3467).
	ReplaceDemoAccountRole(ctx context.Context, accountID, tenantID int64, role string) error
	// FindSchoolAdministrator returns the oldest active administrator of the school.
	FindSchoolAdministrator(ctx context.Context, tenantID int64) (accountID int64, found bool, err error)
}

// DemoSchools reaches the demo schools through their owner, Organisation &
// Tenancy, inside the caller's administrative transaction.
type DemoSchools interface {
	// PrepareDemoSchool returns the slug of the school a new access enters:
	// a school of its own that is queued for seeding, or the standing school.
	// It reports domain.ErrDemoCapacityReached when no further school may
	// be queued (#3466).
	PrepareDemoSchool(ctx context.Context, schoolName, personName string) (slug string, err error)
	// DemoSchoolEntry reports the school's progress. A school that is
	// unknown, inactive or deleted is still preparing.
	DemoSchoolEntry(ctx context.Context, slug string) (domain.DemoSchoolEntry, error)
	// MarkDemoSchoolUsed notes an entry into the school; the simulation
	// serves only schools entered in the last minutes (#3464).
	MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error
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
	// IssueParentTokensForAuthenticatedAccount mints the parent-scope session
	// of the demo role parent (#3468).
	IssueParentTokensForAuthenticatedAccount(ctx context.Context, accountID int64, ipAddress, userAgent string) (string, string, error)
}

// DemoAdminTx runs fn inside an administrative transaction.
type DemoAdminTx func(ctx context.Context, fn func(context.Context) error) error
