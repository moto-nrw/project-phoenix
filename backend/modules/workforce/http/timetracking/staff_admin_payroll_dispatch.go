package timetracking

import (
	"context"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

// queuePayrollDispatch records the automatic send of #4805 in the close's
// request transaction: the dispatch commits with the frozen month or not at
// all. The person who closed the month is the actor of the file's access
// audit.
func (rs *StaffAdminResource) queuePayrollDispatch(ctx context.Context, result *workforce.MonthCloseResult) error {
	claims := rs.identity(ctx)
	return rs.PayrollDispatch.QueueAfterMonthClose(ctx, workforce.PayrollDispatchTrigger{
		Year: result.Year, Month: result.Month, ClosedStaff: result.ClosedStaff,
		ActorAccountID: claims.AccountID, ActorName: strings.TrimSpace(claims.DisplayName),
	})
}

// getPayrollDispatch handles GET /api/staff/time-tracking/payroll-dispatch:
// the newest automatic dispatch for the Lohnbüro block of /payroll.
func (rs *StaffAdminResource) getPayrollDispatch(w http.ResponseWriter, r *http.Request) {
	overview, err := rs.PayrollDispatch.Overview(r.Context())
	if err != nil {
		rs.logger.Error("failed to read payroll dispatch", "error", err.Error())
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Payroll dispatch retrieved")
}
