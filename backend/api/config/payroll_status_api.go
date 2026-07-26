package config

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
)

// getPayrollStatus reports the payroll configuration state (#1417): Lohnart
// mapping, LODAS header identifiers, and how many staff still lack a
// Personalnummer. The /payroll page renders this; the later DATEV writers
// run the same service check before producing a file.
func (rs *SettingsResource) getPayrollStatus(w http.ResponseWriter, r *http.Request) {
	if rs.payrollStatus == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("payroll status service is not wired")))
		return
	}
	status, err := rs.payrollStatus.GetPayrollStatus(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, status, "Payroll status retrieved successfully")
}
