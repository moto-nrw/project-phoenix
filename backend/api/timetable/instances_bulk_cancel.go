package timetable

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// bulkCancelRequest is the body of POST /instances/bulk-cancel (#3594).
type bulkCancelRequest struct {
	From   string `json:"from"`
	To     string `json:"to"`
	DryRun bool   `json:"dry_run"`
	// IncludeClosingDaySeries also cancels series planned on closing days on
	// purpose (holiday care). Off by default.
	IncludeClosingDaySeries bool `json:"include_closing_day_series"`
}

func (req *bulkCancelRequest) Bind(_ *http.Request) error {
	if req.From == "" || req.To == "" {
		return errors.New("from and to are required")
	}
	return nil
}

// bulkCancelInstances cancels and removes the planned occurrences of a date
// range, for example a closure the school entered after the plan was made.
// dry_run only returns the count per day and the series that stay, for the
// confirmation dialog.
func (rs *Resource) bulkCancelInstances(w http.ResponseWriter, r *http.Request) {
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}
	req := &bulkCancelRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	from, fromErr := berlinDate(req.From)
	to, toErr := berlinDate(req.To)
	if fromErr != nil || toErr != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(fmt.Errorf("%w: from and to must be dates in YYYY-MM-DD format", timetable.ErrInvalidBulkCancelRange)))
		return
	}

	opts := timetable.BulkCancelOptions{DryRun: req.DryRun, IncludeClosingDaySeries: req.IncludeClosingDaySeries}
	result, err := rs.InstanceService.BulkCancelPlanned(r.Context(), from, to, opts, jwt.ActorAccountIDFromCtx(r.Context()))
	if errors.Is(err, timetable.ErrInvalidBulkCancelRange) {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	message := "Instances cancelled"
	if req.DryRun {
		message = "Instances counted"
	}
	common.Respond(w, r, http.StatusOK, result, message)
}
