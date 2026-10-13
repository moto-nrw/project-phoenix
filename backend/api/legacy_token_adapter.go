package api

import (
	"context"

	projectJWT "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/services"
)

// requestTokenReads binds the legacy composition's request reads to the
// retained token adapter the request middleware fills (#2747).
var requestTokenReads = services.TokenAdapter{
	Permissions:           projectJWT.PermissionsFromCtx,
	Claims:                requestClaims,
	Memo:                  requestIdentityMemo,
	NewCapabilityToken:    projectJWT.NewOpaqueCapabilityToken,
	CapabilityFingerprint: projectJWT.OpaqueCapabilityFingerprint,
}

// newLegacyTokenAdapter binds the request reads and the session signer the
// legacy composition consumes over the retained token adapter. The signing
// primitive stays in the adapter; Identity owns the claim schema and
// validation.
func newLegacyTokenAdapter() (services.TokenAdapter, error) {
	signer, err := newSessionTokenAuth()
	if err != nil {
		return services.TokenAdapter{}, err
	}
	codec, err := services.NewSessionTokens(signer.JwtAuth, signer.JwtExpiry, signer.JwtRefreshExpiry)
	if err != nil {
		return services.TokenAdapter{}, err
	}
	tokens := requestTokenReads
	tokens.SessionTokens = codec
	return tokens, nil
}

// CallerFromClaims resolves the principal the authenticator verified.
var CallerFromClaims = requestTokenReads.Caller

// requestClaims reads the verified claims the authenticator attached.
func requestClaims(ctx context.Context) (services.RequestClaims, bool) {
	claims, ok := ctx.Value(projectJWT.CtxClaims).(projectJWT.AppClaims)
	if !ok {
		return services.RequestClaims{}, false
	}
	return services.RequestClaims{
		AccountID: int64(claims.ID), TenantID: claims.TenantID, Scope: claims.Scope,
		SchoolScope: claims.IsSchoolScope(), Admin: claims.IsAdmin,
		FirstName: claims.FirstName, LastName: claims.LastName, Username: claims.Username,
	}, true
}

// requestIdentityMemo resolves the request identity memo the request
// middleware attached (#2099).
func requestIdentityMemo(ctx context.Context) services.RequestMemo {
	if cache := projectJWT.RequestIdentityCacheFrom(ctx); cache != nil {
		return cache
	}
	return nil
}

// actingAccountID reads the acting account of the request.
func actingAccountID(ctx context.Context) int64 {
	claims, _ := requestClaims(ctx)
	return claims.AccountID
}
