package presence

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// getCrossTenantStudents returns students visiting from other tenants.
// GET /api/active/cross-tenant-students
func (rs *Resource) getCrossTenantStudents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	hostingTenantID := rs.runtime.TenantID(ctx)
	if hostingTenantID == 0 {
		common.RespondWithError(w, r, http.StatusBadRequest, "Tenant context required")
		return
	}

	students, err := rs.Operations.CrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		common.RenderError(w, r, ErrorRenderer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, newCrossTenantStudentResponses(students), "Cross-tenant students retrieved successfully")
}
