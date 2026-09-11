package config

import (
	"net/http"
)

// getPayrollStatus reports the payroll configuration state (#1417): Lohnart
// mapping, LODAS header identifiers, and how many staff still lack a
// Personalnummer. The /payroll page renders this; the later DATEV writers
// run the same service check before producing a file.
func (rs *SettingsResource) getPayrollStatus(w http.ResponseWriter, r *http.Request) {
	status, err := rs.operations.PayrollStatus(r.Context())
	if err != nil {
		rs.runtime.RenderError(w, r, http.StatusInternalServerError, err)
		return
	}
	rs.runtime.Respond(w, r, http.StatusOK, status, "Payroll status retrieved successfully")
}
