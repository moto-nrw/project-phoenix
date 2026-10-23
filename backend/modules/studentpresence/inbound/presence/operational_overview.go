package presence

import (
	"context"
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// operationalOverview propagates an unreadable visibility policy instead of
// interpreting it as the school's own-group choice.
func (rs *Resource) operationalOverview(ctx context.Context) (bool, error) {
	assignmentBound := common.IsAssignmentBoundPortal(ctx)
	admin := common.HasEffectiveAdminScope(ctx)
	return rs.authorization.OperationalOverview(ctx, rs.SettingsService, rs.UserContextService, assignmentBound, admin)
}

// requireOperationalOverview renders policy faults separately from a genuine
// access denial, so both overview endpoints preserve the same failure contract.
func (rs *Resource) requireOperationalOverview(w http.ResponseWriter, r *http.Request, denial string) bool {
	allowed, err := rs.operationalOverview(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("failed to resolve operational overview", err))
		return false
	}
	if !allowed {
		common.RenderError(w, r, common.ErrorForbidden(errors.New(denial)))
		return false
	}
	return true
}
