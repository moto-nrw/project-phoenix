package active

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

// operationalOverview reports whether the caller may see and operate every
// running module of this school (#2380). It fails closed: a settings fault is
// logged as an operational error and never read as a tenant choice.
//
// This is the single rule behind list, detail, SSE and write paths — do not
// re-derive operational scope from the organisational group mode.
func (rs *Resource) operationalOverview(ctx context.Context) bool {
	principal, principalErr := common.CurrentPrincipal(ctx)
	assignmentBound := principalErr == nil && principal.Scope() == permissions.ScopeSchool
	admin := principalErr == nil && principal.HasAdminScope()
	allowed, err := authorize.HasOperationalOverview(ctx, rs.SettingsService, rs.UserContextService, assignmentBound, admin)
	if err != nil {
		rs.getLogger().ErrorContext(ctx, "failed to resolve operational overview scope", slog.String("error", err.Error()))
		return false
	}
	return allowed
}
