package staff

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// overviewErrorRules classifies the overview sentinels (#1417): caller
// mistakes → 400. ErrAccountStartTooOld stays a 500 on purpose — it means the
// tenant's account-start setting is misconfigured, the same way the per-staff
// Monatskarte fails.
var overviewErrorRules = []common.ErrorRule{
	{Target: activeSvc.ErrOverviewInvalid, Render: common.ErrorInvalidRequest},
}

func renderOverviewError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, overviewErrorRules, common.ErrorInternalServer))
}

// parseDashboardPeriod validates ?period=. Empty means month.
func parseDashboardPeriod(r *http.Request) (string, error) {
	period := r.URL.Query().Get("period")
	switch period {
	case "", activeSvc.OverviewPeriodMonth:
		return activeSvc.OverviewPeriodMonth, nil
	case activeSvc.OverviewPeriodWeek:
		return activeSvc.OverviewPeriodWeek, nil
	default:
		return "", errors.New("period must be week or month")
	}
}

// parseOverviewFilters reads the list filters. saldo_min/saldo_max are pointers
// so "0" stays distinguishable from "not set"; negative values are valid (a
// minus balance is a normal state). Unknown sort keys and employment types are
// rejected in the service rather than silently defaulted.
func parseOverviewFilters(r *http.Request) (activeSvc.OverviewFilters, error) {
	query := r.URL.Query()
	filters := activeSvc.OverviewFilters{
		EmploymentType: query.Get("employment_type"),
		SortBy:         query.Get("sort"),
	}
	switch order := query.Get("order"); order {
	case "", "asc":
	case "desc":
		filters.Descending = true
	default:
		return filters, errors.New("order must be asc or desc")
	}
	// year/month are optional and only meaningful together: a bare ?month=3
	// would silently mean March of whatever year today happens to be, which
	// reads as a working filter and is not one.
	rawYear, rawMonth := query.Get("year"), query.Get("month")
	if (rawYear == "") != (rawMonth == "") {
		return filters, errors.New("year and month must be given together")
	}
	if rawYear != "" {
		var err error
		if filters.Year, err = strconv.Atoi(rawYear); err != nil {
			return filters, errors.New("year must be an integer")
		}
		if filters.Month, err = strconv.Atoi(rawMonth); err != nil {
			return filters, errors.New("month must be an integer")
		}
		if filters.Year == 0 || filters.Month == 0 {
			return filters, errors.New("year and month must be greater than zero")
		}
	}
	for _, bound := range []struct {
		name   string
		target **int
	}{
		{"saldo_min", &filters.SaldoMin},
		{"saldo_max", &filters.SaldoMax},
	} {
		raw := query.Get(bound.name)
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil {
			return filters, errors.New(bound.name + " must be an integer number of minutes")
		}
		*bound.target = &value
	}
	return filters, nil
}

// getDashboardSummary handles GET /api/staff/dashboard-summary?period=week|month
func (rs *Resource) getDashboardSummary(w http.ResponseWriter, r *http.Request) {
	period, err := parseDashboardPeriod(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	summary, err := rs.StaffOverviewService.GetDashboardSummary(r.Context(), period)
	if err != nil {
		renderOverviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, summary, "Dashboard summary retrieved")
}

// getTimeTrackingOverview handles GET /api/staff/time-tracking/overview
func (rs *Resource) getTimeTrackingOverview(w http.ResponseWriter, r *http.Request) {
	filters, err := parseOverviewFilters(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	overview, err := rs.StaffOverviewService.GetTimeTrackingOverview(r.Context(), filters)
	if err != nil {
		renderOverviewError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, overview, "Time tracking overview retrieved")
}
