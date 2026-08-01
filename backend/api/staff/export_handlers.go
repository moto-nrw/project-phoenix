package staff

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// timeExportErrorRules classifies the export sentinels: caller mistakes →
// 400, an incomplete payroll configuration → 409 (fixable on /payroll, not by
// changing the request).
var timeExportErrorRules = []common.ErrorRule{
	{Target: activeSvc.ErrTimeExportInvalid, Render: common.ErrorInvalidRequest},
	{Target: activeSvc.ErrPayrollConfigIncomplete, Render: func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, "payroll_config_incomplete")
	}},
}

// parseTimeExportRequest reads the export parameters. Year is required; month
// is optional (absent = whole year). Granularity/format/time_format default in
// the service so their canonical values live in one place.
func parseTimeExportRequest(r *http.Request) (activeSvc.TimeExportRequest, error) {
	query := r.URL.Query()
	req := activeSvc.TimeExportRequest{
		Granularity: query.Get("granularity"),
		Format:      query.Get("format"),
		TimeFormat:  query.Get("time_format"),
	}
	rawYear := query.Get("year")
	if rawYear == "" {
		return req, errors.New("year query parameter is required")
	}
	var err error
	if req.Year, err = strconv.Atoi(rawYear); err != nil {
		return req, errors.New("year must be an integer")
	}
	if rawMonth := query.Get("month"); rawMonth != "" {
		if req.Month, err = strconv.Atoi(rawMonth); err != nil {
			return req, errors.New("month must be an integer")
		}
	}
	return req, nil
}

// exportTimeTracking handles GET /api/staff/time-tracking/export — the
// cross-staff payroll/evidence export (#1417 2b). time_tracking:manage is
// enforced at the route; the service writes a GDPR access-audit row and
// produces no file when that write fails.
func (rs *Resource) exportTimeTracking(w http.ResponseWriter, r *http.Request) {
	req, err := parseTimeExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	file, err := rs.TimeExportService.Export(r.Context(), req, int64(claims.ID), strings.Join(claims.Roles, ","))
	if err != nil {
		rs.getLogger().Error("failed to export time tracking",
			"error", err.Error(),
		)
		common.RenderError(w, r, common.RenderWithRules(err, timeExportErrorRules, common.ErrorInternalServer))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+file.Filename+"\"")
	w.Header().Set("Content-Length", strconv.Itoa(len(file.Data)))
	if _, err := w.Write(file.Data); err != nil {
		rs.getLogger().Error("failed to write export response", "error", err.Error())
	}
}

// datevExportReport handles GET /api/staff/time-tracking/export/datev-report —
// the preflight the export dialog shows before a DATEV download. Same
// permission as the export; no audit row because nothing leaves the system.
func (rs *Resource) datevExportReport(w http.ResponseWriter, r *http.Request) {
	req, err := parseTimeExportRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	report, err := rs.TimeExportService.DatevReport(r.Context(), req)
	if err != nil {
		common.RenderError(w, r, common.RenderWithRules(err, timeExportErrorRules, common.ErrorInternalServer))
		return
	}
	common.Respond(w, r, http.StatusOK, report, "DATEV export report")
}
