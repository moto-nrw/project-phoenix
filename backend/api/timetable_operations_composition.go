package api

import (
	"context"

	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// TimetableRequestSession binds the operational day's session port to
// Identity's session token adapter.
type TimetableRequestSession struct{}

// Claims are the verified claims of the request, the zero claims without
// any.
func (TimetableRequestSession) Claims(ctx context.Context) timetablePorts.RequestClaims {
	claims, _ := requestTokenReads.Claims(ctx)
	return timetablePorts.RequestClaims{
		AccountID: claims.AccountID, TenantID: claims.TenantID, Scope: claims.Scope,
		SchoolScope: claims.SchoolScope, Admin: claims.Admin,
	}
}

// Permissions are the permissions of the request.
func (TimetableRequestSession) Permissions(ctx context.Context) []string {
	return requestTokenReads.Permissions(ctx)
}
