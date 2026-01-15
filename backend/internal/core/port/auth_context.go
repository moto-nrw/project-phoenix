package port

import "context"

// AuthContextKey is used to store auth-related values in context.
type AuthContextKey int

const (
	CtxClaims AuthContextKey = iota
	CtxRefreshToken
	CtxPermissions
)

// ClaimsFromCtx retrieves the parsed AppClaims from request context.
// Returns zero-value AppClaims if not present or wrong type.
func ClaimsFromCtx(ctx context.Context) AppClaims {
	claims, ok := ctx.Value(CtxClaims).(AppClaims)
	if !ok {
		return AppClaims{}
	}
	return claims
}

// PermissionsFromCtx retrieves the permissions array from request context.
func PermissionsFromCtx(ctx context.Context) []string {
	perms, ok := ctx.Value(CtxPermissions).([]string)
	if !ok {
		return []string{}
	}
	return perms
}

// RefreshTokenFromCtx retrieves the parsed refresh token string from context.
func RefreshTokenFromCtx(ctx context.Context) string {
	token, ok := ctx.Value(CtxRefreshToken).(string)
	if !ok {
		return ""
	}
	return token
}
