package services

import (
	"context"
	"strings"

	authjwt "github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

func requestAuditActor(ctx context.Context) (int64, string) {
	claims := authjwt.ClaimsFromCtx(ctx)
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = strings.TrimSpace(claims.Username)
	}
	return int64(claims.ID), name
}

func requestActorScope(ctx context.Context) (int64, string) {
	claims := authjwt.ClaimsFromCtx(ctx)
	return int64(claims.ID), claims.Scope
}
