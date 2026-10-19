package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// RequestClaims are the verified claims of the authenticated request the
// root's compositions read.
type RequestClaims struct {
	AccountID   int64
	TenantID    int64
	Scope       string
	SchoolScope bool
	Admin       bool
	FirstName   string
	LastName    string
	Username    string
}

// TokenAdapter carries what the root's compositions read from Identity's
// session token adapter: the authenticated request's claims, permissions and
// identity memo, the session token codec and the opaque capability tokens.
type TokenAdapter struct {
	// Permissions returns the permissions of the authenticated request.
	Permissions func(context.Context) []string
	// Claims returns the verified claims of the request; false when the
	// request carries none.
	Claims func(context.Context) (RequestClaims, bool)
	// Memo resolves the request identity memo the request middleware
	// attached (#2099); nil when there is none.
	Memo func(context.Context) RequestMemo
	// SessionTokens signs and verifies the session tokens.
	SessionTokens identityaccessCompose.SignedIdentityTokens
	// NewCapabilityToken mints an opaque capability token and its
	// fingerprint; CapabilityFingerprint fingerprints a presented one.
	NewCapabilityToken    func() (raw, fingerprint string, err error)
	CapabilityFingerprint func(raw string) string
}

// RequestMemo is the request identity memo (#2099) the caller context
// consults, keyed by tenant and account.
type RequestMemo interface {
	Entry(tenantID, accountID int64, create func() any) any
	Evict(tenantID, accountID int64)
}

// memo resolves the request identity memo, nil when the request carries
// none.
func (a TokenAdapter) memo(ctx context.Context) identityaccessCompose.CallerMemo {
	if memo := a.Memo(ctx); memo != nil {
		return memo
	}
	return nil
}

// claims returns the request claims, or the zero claims without any.
func (a TokenAdapter) claims(ctx context.Context) RequestClaims {
	claims, _ := a.Claims(ctx)
	return claims
}

// Caller resolves the principal the authenticator verified.
func (a TokenAdapter) Caller(ctx context.Context) identityaccess.Caller {
	claims, ok := a.Claims(ctx)
	if !ok {
		return identityaccess.Caller{}
	}
	return identityaccess.Caller{
		Authenticated:  true,
		AccountID:      claims.AccountID,
		ClaimsTenantID: claims.TenantID,
		Scope:          claims.Scope,
		SchoolScope:    claims.SchoolScope,
		AdminRole:      claims.Admin,
		AdminWildcard:  securityruntime.HasAdminWildcard(a.Permissions(ctx)),
	}
}

// AuditActor returns the acting account of the request and the name the
// audit records it under.
func (a TokenAdapter) AuditActor(ctx context.Context) (int64, string) {
	claims := a.claims(ctx)
	return claims.AccountID, identityaccess.AuditActorName(claims.FirstName, claims.LastName, claims.Username)
}

// actorScope returns the acting account of the request and its token scope.
func (a TokenAdapter) actorScope(ctx context.Context) (int64, string) {
	claims := a.claims(ctx)
	return claims.AccountID, claims.Scope
}
