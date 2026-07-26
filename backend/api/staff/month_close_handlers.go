package staff

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	timetracking "github.com/moto-nrw/project-phoenix/api/time-tracking"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// monthCloseErrorRules classifies the remaining Monatsabschluss sentinel
// (#1417): caller mistakes → 400. The sentinels the frontend has to
// distinguish carry machine-readable codes in renderMonthCloseError before
// this table is consulted.
var monthCloseErrorRules = []common.ErrorRule{
	{Target: activeSvc.ErrMonthCloseInvalid, Render: common.ErrorInvalidRequest},
}

func renderMonthCloseError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, activeSvc.ErrLaterMonthClosed):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "later_month_closed"))
	case errors.Is(err, activeSvc.ErrMonthNotClosable):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "month_not_closable"))
	case errors.Is(err, activeSvc.ErrMonthNotClosed):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, "month_not_closed"))
	default:
		common.RenderError(w, r, common.RenderWithRules(err, monthCloseErrorRules, common.ErrorInternalServer))
	}
}

// monthCloseRequest is the wire shape for POST
// /api/staff/time-tracking/month-close and the per-staff reopen. The reason is
// mandatory: freezing and unfreezing a Stundenkonto are both decisions that
// have to be attributable.
type monthCloseRequest struct {
	Year   int    `json:"year"`
	Month  int    `json:"month"`
	Reason string `json:"reason"`
}

// listMonthCloseStatus handles GET /api/staff/time-tracking/month-close?year=&month=
func (rs *Resource) listMonthCloseStatus(w http.ResponseWriter, r *http.Request) {
	year, month, err := timetracking.ParseYearMonthQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	snapshots, err := rs.MonthCloseService.ListMonthStatus(r.Context(), year, month)
	if err != nil {
		renderMonthCloseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, snapshots, "Month close status retrieved")
}

// closeMonth handles POST /api/staff/time-tracking/month-close — freezes the
// month for the whole school. The ambient tenant transaction wraps the entire
// loop, so one failing staff member rolls the close back completely.
func (rs *Resource) closeMonth(w http.ResponseWriter, r *http.Request) {
	closedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req monthCloseRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	result, err := rs.MonthCloseService.CloseMonth(r.Context(), closedBy, req.Year, req.Month, req.Reason)
	if err != nil {
		tenant.MarkRollback(r.Context())
		renderMonthCloseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, result, "Month closed")
}

// reopenMonth handles POST /api/staff/{id}/time-tracking/month-close/reopen.
// POST rather than DELETE because it carries a mandatory reason.
func (rs *Resource) reopenMonth(w http.ResponseWriter, r *http.Request) {
	staffID, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if !rs.requireBalanceAdjustmentStaff(w, r, staffID) {
		return
	}
	reopenedBy, err := rs.resolveEditorStaffID(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}
	var req monthCloseRequest
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err := rs.MonthCloseService.ReopenMonth(r.Context(), staffID, reopenedBy, req.Year, req.Month, req.Reason); err != nil {
		tenant.MarkRollback(r.Context())
		renderMonthCloseError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, nil, "Month reopened")
}
