package testutil

import "github.com/moto-nrw/project-phoenix/api/testutil/routetest"

// The request, response, router and claims helpers live in routetest, which
// does not link the service graph this package composes. They are re-exported
// here so suites that also need a module builder keep one import.
type (
	RequestOption       = routetest.RequestOption
	Response            = routetest.Response
	Claims              = routetest.Claims
	MFAEnrollmentClaims = routetest.MFAEnrollmentClaims
)

const (
	MFAEnrollmentScopeTenant   = routetest.MFAEnrollmentScopeTenant
	MFAEnrollmentScopePlatform = routetest.MFAEnrollmentScopePlatform
)

var (
	WithTestTenant                = routetest.WithTestTenant
	ProtectedTestTenantGroup      = routetest.ProtectedTestTenantGroup
	ProtectedTestTenantGroupFunc  = routetest.ProtectedTestTenantGroupFunc
	UnprotectedGroupFunc          = routetest.UnprotectedGroupFunc
	RecordingUnprotectedGroupFunc = routetest.RecordingUnprotectedGroupFunc
	IdentityMiddleware            = routetest.IdentityMiddleware
	RespondSuccess                = routetest.RespondSuccess
	RespondNoContent              = routetest.RespondNoContent
	RespondError                  = routetest.RespondError
	RespondInvalidRequest         = routetest.RespondInvalidRequest
	RespondCoded                  = routetest.RespondCoded
	ErrorResponder                = routetest.ErrorResponder
	WithJWTBearer                 = routetest.WithJWTBearer
	SeedTestJWTConfig             = routetest.SeedTestJWTConfig
	NewRequest                    = routetest.NewRequest
	NewAuthenticatedRequest       = routetest.NewAuthenticatedRequest
	NewJSONRequest                = routetest.NewJSONRequest
	NewMultipartRequest           = routetest.NewMultipartRequest
	NewTenantRouter               = routetest.NewTenantRouter
	NewJSONRouter                 = routetest.NewJSONRouter
	ExecuteRequest                = routetest.ExecuteRequest
	ExecuteRequestForTest         = routetest.ExecuteRequestForTest
	ParseResponse                 = routetest.ParseResponse
	ParseJSONResponse             = routetest.ParseJSONResponse
	AssertSuccessResponse         = routetest.AssertSuccessResponse
	AssertErrorResponse           = routetest.AssertErrorResponse
	AssertUnauthorized            = routetest.AssertUnauthorized
	AssertForbidden               = routetest.AssertForbidden
	AssertNotFound                = routetest.AssertNotFound
	AssertBadRequest              = routetest.AssertBadRequest
	WithSessionVerifier           = routetest.WithSessionVerifier
	WithAuthenticatedContext      = routetest.WithAuthenticatedContext
	WithPermissions               = routetest.WithPermissions
	WithClaims                    = routetest.WithClaims
	MintTestJWT                   = routetest.MintTestJWT
	AuthenticationContext         = routetest.AuthenticationContext
	ExecuteWithAuth               = routetest.ExecuteWithAuth
	ExecuteWithAuthPermissions    = routetest.ExecuteWithAuthPermissions
	DefaultTestClaims             = routetest.DefaultTestClaims
	TeacherTestClaims             = routetest.TeacherTestClaims
	AdminTestClaims               = routetest.AdminTestClaims
	AdminTestClaimsForTenant      = routetest.AdminTestClaimsForTenant
	TenantUserTestClaims          = routetest.TenantUserTestClaims
	ParentTestClaims              = routetest.ParentTestClaims
	TestTokenAuth                 = routetest.TestTokenAuth
	ClaimsFromContext             = routetest.ClaimsFromContext
	WithEnrollmentClaims          = routetest.WithEnrollmentClaims
	OpaqueCapabilityFingerprint   = routetest.OpaqueCapabilityFingerprint
	WithRefreshToken              = routetest.WithRefreshToken
)
