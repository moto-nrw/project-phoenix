package api

import (
	"context"
	"time"

	"github.com/spf13/viper"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// requestTokenReads binds the root's request reads to Identity's session
// token adapter the request middleware fills (#2747, #4006).
var requestTokenReads = TokenAdapter{
	Permissions:           sessiontoken.PermissionsFromCtx,
	Claims:                requestClaims,
	Memo:                  requestIdentityMemo,
	NewCapabilityToken:    sessiontoken.NewOpaqueCapabilityToken,
	CapabilityFingerprint: sessiontoken.OpaqueCapabilityFingerprint,
}

// newLegacyTokenAdapter binds the request reads and the session tokens the
// root's Identity & Access composition consumes. Identity's composition
// builds the session
// codec from the configured secret and token lifetimes; the signing primitive
// stays in its session token adapter (ADR 0053 point 6).
func newLegacyTokenAdapter() (TokenAdapter, error) {
	return tokenAdapterFor(viper.GetString("auth_jwt_secret"), viper.GetDuration("auth_jwt_expiry"), viper.GetDuration("auth_jwt_refresh_expiry"))
}

// tokenAdapterFor binds the request reads to the session tokens of the given
// signing secret and token lifetimes.
func tokenAdapterFor(secret string, accessExpiry, refreshExpiry time.Duration) (TokenAdapter, error) {
	codec, err := identityaccessCompose.NewConfiguredSessionTokens(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return TokenAdapter{}, err
	}
	tokens := requestTokenReads
	tokens.SessionTokens = codec
	return tokens, nil
}

// CallerFromClaims resolves the principal the authenticator verified.
var CallerFromClaims = requestTokenReads.Caller

// requestClaims reads the verified claims the authenticator attached.
func requestClaims(ctx context.Context) (RequestClaims, bool) {
	claims, ok := ctx.Value(sessiontoken.CtxClaims).(sessiontoken.AppClaims)
	if !ok {
		return RequestClaims{}, false
	}
	return RequestClaims{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Scope: claims.Scope,
		SchoolScope: claims.IsSchoolScope(), Admin: claims.IsAdmin,
		FirstName: claims.FirstName, LastName: claims.LastName, Username: claims.Username,
	}, true
}

// requestIdentityMemo resolves the request identity memo the request
// middleware attached (#2099).
func requestIdentityMemo(ctx context.Context) RequestMemo {
	if cache := sessiontoken.RequestIdentityCacheFrom(ctx); cache != nil {
		return cache
	}
	return nil
}

// actingAccountID reads the acting account of the request.
func actingAccountID(ctx context.Context) int64 {
	claims, _ := requestClaims(ctx)
	return claims.AccountID
}
