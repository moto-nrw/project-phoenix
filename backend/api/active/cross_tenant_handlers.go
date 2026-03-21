package active

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// getCrossTenantStudents returns students visiting from other tenants.
// GET /api/active/cross-tenant-students
func (rs *Resource) getCrossTenantStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hostingTenantID := tenant.FromContext(ctx)
	if hostingTenantID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Tenant context required")
		return
	}

	students, err := rs.ActiveService.GetCrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, students, "Cross-tenant students retrieved successfully")
}
