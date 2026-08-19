package operator

import (
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// GetProvisioningStats returns platform-wide aggregate counts for the operator
// overview KPI strip. GET /api/operator/stats.
func (rs *ProvisioningResource) GetProvisioningStats(w http.ResponseWriter, r *http.Request) {
	stats, err := rs.service.GetProvisioningStats(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, stats, "Provisioning stats retrieved successfully")
}

// GetSchoolPWAUsage returns one school's PWA standalone-usage counts
// (staff/parent, 30-day window). GET /api/operator/schools/{id}/pwa-usage.
func (rs *ProvisioningResource) GetSchoolPWAUsage(w http.ResponseWriter, r *http.Request) {
	idList(w, r, "id", "invalid school ID", rs.service.GetSchoolPWAUsage, ProvisioningErrorRenderer, "School PWA usage retrieved successfully")
}

// ListOrganizationSummaries returns all organizations with per-row counts for
// the Träger overview table. GET /api/operator/organizations/summaries.
func (rs *ProvisioningResource) ListOrganizationSummaries(w http.ResponseWriter, r *http.Request) {
	summaries, err := rs.service.ListOrganizationSummaries(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summaries, "Organization summaries retrieved successfully")
}

// ListSchoolSummaries returns all schools (global scope) with per-row counts
// for the global Schulen table. GET /api/operator/schools/summaries.
func (rs *ProvisioningResource) ListSchoolSummaries(w http.ResponseWriter, r *http.Request) {
	summaries, err := rs.service.ListSchoolSummaries(r.Context())
	if err != nil {
		common.RenderError(w, r, ProvisioningErrorRenderer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, summaries, "School summaries retrieved successfully")
}

// ListOrganizationSchoolSummaries returns schools under a specific organization
// with per-row counts. GET /api/operator/organizations/{id}/schools.
func (rs *ProvisioningResource) ListOrganizationSchoolSummaries(w http.ResponseWriter, r *http.Request) {
	idList(w, r, "id", "invalid organization ID", rs.service.ListOrganizationSchoolSummaries, ProvisioningErrorRenderer, "Organization school summaries retrieved successfully")
}

// ListOrganizationPersons returns all persons across every school belonging to
// the given organization. GET /api/operator/organizations/{id}/persons.
func (rs *ProvisioningResource) ListOrganizationPersons(w http.ResponseWriter, r *http.Request) {
	idList(w, r, "id", "invalid organization ID", rs.service.ListOrganizationPersons, ProvisioningErrorRenderer, "Organization persons retrieved successfully")
}
