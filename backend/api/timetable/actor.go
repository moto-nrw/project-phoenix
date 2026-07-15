package timetable

import (
	"context"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// resolveActorAccountID reads the acting account id straight from the JWT
// claims — no DB lookup needed (unlike resolveStartedByStaffID, which resolves
// a staff row). Returns nil when claims are missing so the
// Änderungsprotokoll stores NULL instead of a fabricated id (#1886).
func resolveActorAccountID(ctx context.Context) *int64 {
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok || claims.ID == 0 {
		return nil
	}
	id := int64(claims.ID)
	return &id
}
