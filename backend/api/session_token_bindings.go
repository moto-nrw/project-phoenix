package api

import (
	"context"
	"fmt"
	"time"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/spf13/viper"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// requestTokenReads binds the root's request reads to Identity's session
// token adapter the request middleware fills (#2747, #4006).
var requestTokenReads = owners.TokenAdapter{
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
func newLegacyTokenAdapter() (owners.TokenAdapter, error) {
	return tokenAdapterFor(viper.GetString("auth_jwt_secret"), viper.GetDuration("auth_jwt_expiry"), viper.GetDuration("auth_jwt_refresh_expiry"))
}

// tokenAdapterFor binds the request reads to the session tokens of the given
// signing secret and token lifetimes.
func tokenAdapterFor(secret string, accessExpiry, refreshExpiry time.Duration) (owners.TokenAdapter, error) {
	codec, err := NewConfiguredSessionTokens(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return owners.TokenAdapter{}, err
	}
	tokens := requestTokenReads
	tokens.SessionTokens = codec
	return tokens, nil
}

// NewConfiguredSessionAuth builds the HTTP session runtime the server root
// mounts (verifier, authenticator, rate-limit key) from configuration values:
// the signing secret and the lifetimes of the access and refresh tokens. The
// signing primitive stays in Identity's session token adapter, so the root
// hands over values instead of building the signer (ADR 0053 point 6,
// ADR 0056).
func NewConfiguredSessionAuth(secret string, accessExpiry, refreshExpiry time.Duration) (*sessiontoken.TokenAuth, error) {
	signer, err := sessiontoken.NewTokenAuthWithDurations(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, fmt.Errorf("invalid auth JWT configuration: %w", err)
	}
	return signer, nil
}

// NewConfiguredSessionTokens builds the session token codec Identity's flows
// sign with from the same configuration values, over the same signer.
func NewConfiguredSessionTokens(secret string, accessExpiry, refreshExpiry time.Duration) (identityaccessCompose.SignedIdentityTokens, error) {
	signer, err := NewConfiguredSessionAuth(secret, accessExpiry, refreshExpiry)
	if err != nil {
		return nil, err
	}
	return identityaccessCompose.NewSessionTokenCodec(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
}

// CallerFromClaims resolves the principal the authenticator verified.
var CallerFromClaims = requestTokenReads.Caller

// requestClaims reads the verified claims the authenticator attached.
func requestClaims(ctx context.Context) (owners.RequestClaims, bool) {
	claims, ok := ctx.Value(sessiontoken.CtxClaims).(sessiontoken.AppClaims)
	if !ok {
		return owners.RequestClaims{}, false
	}
	return owners.RequestClaims{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Scope: claims.Scope,
		SchoolScope: claims.IsSchoolScope(), Admin: claims.IsAdmin,
		FirstName: claims.FirstName, LastName: claims.LastName, Username: claims.Username,
	}, true
}

// actingAccountID reads the acting account of the request.
func actingAccountID(ctx context.Context) int64 {
	claims, _ := requestClaims(ctx)
	return claims.AccountID
}
