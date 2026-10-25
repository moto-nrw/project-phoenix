package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
)

// SchoolDirectory reads the Organisation & Tenancy school facts login
// resolves. Missing schools report found=false.
type SchoolDirectory interface {
	FindSchool(ctx context.Context, id int64) (identityaccess.School, bool, error)
	FindSchoolBySubdomain(ctx context.Context, subdomain string) (identityaccess.School, bool, error)
	LockSchoolShared(ctx context.Context, id int64) (identityaccess.School, bool, error)
	ListActiveSchoolsByID(ctx context.Context, schoolIDs []int64) ([]identityaccess.School, error)
}

// TraegerMemberships reads the Organisation & Tenancy Träger memberships of an
// account: the active, undeleted Träger where its membership is active, in a
// stable order. It joins the caller's transaction.
type TraegerMemberships interface {
	ListAccountTraegerOrganizations(ctx context.Context, accountID int64) ([]identityaccess.TraegerOrganization, error)
}

// PortalModules answers whether a school booked the module of a portal
// (#4895): the parents portal belongs to Eltern-App, the school portal to
// Unterricht am Vormittag. Each call reads in a transaction of its own, so
// login and refresh check it before they open their session transaction. A
// school that has not booked the module answers
// *identityaccess.PortalNotBookedError; a failed read is any other error and
// never counts as booked.
type PortalModules interface {
	// BookedParentPortalSchools returns those of the schools that booked
	// Eltern-App, in the given order.
	BookedParentPortalSchools(ctx context.Context, tenantIDs []int64) ([]int64, error)
	// RequireParentPortal and RequireSchoolPortal refuse one school.
	RequireParentPortal(ctx context.Context, tenantID int64) error
	RequireSchoolPortal(ctx context.Context, tenantID int64) error
}

// TokenCodec signs and parses the session JWTs.
type TokenCodec interface {
	IssueTokenPair(access identityaccess.SessionClaims, refresh identityaccess.RefreshClaims) (accessToken, refreshToken string, err error)
	IssueMFAEnrollmentToken(accountID, tenantID int64, scope string, ttl time.Duration) (string, error)
	ParseAccessToken(token string) (identityaccess.SessionClaims, error)
	ParseRefreshToken(token string) (identityaccess.RefreshClaims, error)
	RefreshExpiry() time.Duration
	// IssueAccessToken signs an access-only JWT (the staff preview mints no
	// refresh token); ParseAccessTokenAllowExpired verifies its signature but
	// not its expiry; AccessExpiry is the access token lifetime.
	IssueAccessToken(claims identityaccess.SessionClaims) (string, error)
	ParseAccessTokenAllowExpired(token string) (identityaccess.SessionClaims, error)
	AccessExpiry() time.Duration
}

// AuthAudit appends authentication events and resolves pending wipes.
type AuthAudit interface {
	RecordAuthEvent(ctx context.Context, event identityaccess.AuthEvent) error
	ListPendingAccountWideWipes(ctx context.Context) ([]identityaccess.PendingAccountWideWipe, error)
	ClaimPendingAccountWideWipes(ctx context.Context, accountID int64) ([]identityaccess.PendingAccountWideWipe, error)
}
