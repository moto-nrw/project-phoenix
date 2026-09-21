package test

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
)

// RequestAuditActor binds the authenticated fixture context to the audit port.
func RequestAuditActor(ctx context.Context) (int64, string) {
	claims := jwt.ClaimsFromCtx(ctx)
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = strings.TrimSpace(claims.Username)
	}
	return int64(claims.ID), name
}
