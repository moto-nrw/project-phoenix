package testutil

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// WithRefreshToken puts the raw refresh token on ctx the way the refresh
// authenticator does, so a refresh handler exercised directly reads it
// without the test naming the token package (#2736).
func WithRefreshToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, jwt.CtxRefreshToken, token)
}
