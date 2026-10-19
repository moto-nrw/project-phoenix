package api

import (
	"context"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// requestIdentityMemo resolves the request identity memo the request
// middleware attached (#2099).
func requestIdentityMemo(ctx context.Context) owners.RequestMemo {
	if cache := securityruntime.RequestIdentityCacheFrom(ctx); cache != nil {
		return cache
	}
	return nil
}
