package test

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// Session claim types and context keys used by test routers. Keeping these
// aliases here lets route helpers use the existing test boundary instead of
// importing the Identity & Access adapter directly.
type AppClaims = jwt.AppClaims
type TokenAuth = jwt.TokenAuth
type MFAEnrollmentClaims = jwt.MFAEnrollmentClaims

const (
	CtxClaims                  = jwt.CtxClaims
	CtxPermissions             = jwt.CtxPermissions
	CtxRefreshToken            = jwt.CtxRefreshToken
	CtxEnrollmentClaims        = jwt.CtxEnrollmentClaims
	MFAEnrollmentScopeTenant   = jwt.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = jwt.MFAEnrollmentScopePlatform
)

func ClaimsFromCtx(ctx context.Context) AppClaims { return jwt.ClaimsFromCtx(ctx) }

func PermissionsFromCtx(ctx context.Context) []string { return jwt.PermissionsFromCtx(ctx) }

func OpaqueCapabilityFingerprint(token string) string { return jwt.OpaqueCapabilityFingerprint(token) }
