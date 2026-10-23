package presence

import (
	"context"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// operationalOverview propagates an unreadable visibility policy instead of
// interpreting it as the school's own-group choice.
func (rs *Resource) operationalOverview(ctx context.Context) (bool, error) {
	assignmentBound := common.IsAssignmentBoundPortal(ctx)
	admin := common.HasEffectiveAdminScope(ctx)
	return rs.authorization.OperationalOverview(ctx, rs.SettingsService, rs.UserContextService, assignmentBound, admin)
}
