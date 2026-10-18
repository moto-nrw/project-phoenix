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
	// ListManageableSchoolIDs returns the live, active schools of the
	// organisation: the set an organisation-scoped administrator may
	// administer accounts at.
	ListManageableSchoolIDs(ctx context.Context, organizationID int64) ([]int64, error)
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
