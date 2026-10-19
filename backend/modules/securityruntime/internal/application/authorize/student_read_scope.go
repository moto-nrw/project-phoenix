package authorize

import (
	"context"

	securitycontract "github.com/moto-nrw/project-phoenix/modules/securityruntime/ports"
)

// ResolveStudentReadScope is the list-level counterpart to CanReadStudent: it
// reports whether the caller may read students across the tenant. True for
// admin permissions or a verified staff record; false for every other caller
// (guest, guardian), who may read nothing unredacted.
//
// Read-only — like CanReadStudent it must not gate writes.
func ResolveStudentReadScope(
	ctx context.Context,
	userPermissions []string,
	userCtx securitycontract.StudentAccessUserContext,
) bool {
	if HasAdminWildcard(userPermissions) {
		return true
	}
	return isVerifiedStaff(ctx, userCtx)
}
