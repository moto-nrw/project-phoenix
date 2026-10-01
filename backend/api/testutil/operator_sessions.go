package testutil

import (
	"context"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// WithRefreshToken puts the raw refresh token on ctx the way the refresh
// authenticator does, so a refresh handler exercised directly reads it
// without the test naming the token package (#2736).
func WithRefreshToken(ctx context.Context, token string) context.Context {
	return testpkg.WithRefreshToken(ctx, token)
}
