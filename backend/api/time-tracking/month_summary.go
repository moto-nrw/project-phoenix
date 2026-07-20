package timetracking

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
)

// ParseYearMonthQuery extracts and validates the year/month query parameters
// of a Monatskarte request. Exported for reuse by the staff-scoped admin
// endpoint.
func ParseYearMonthQuery(r *http.Request) (int, int, error) {
	year, err := strconv.Atoi(r.URL.Query().Get("year"))
	if err != nil {
		return 0, 0, errors.New("year query parameter is required")
	}
	month, err := strconv.Atoi(r.URL.Query().Get("month"))
	if err != nil {
		return 0, 0, errors.New("month query parameter is required")
	}
	if year < 2000 || year > 2100 || month < 1 || month > 12 {
		return 0, 0, errors.New("year/month out of range")
	}
	return year, month, nil
}

// getOwnMonthSummary handles GET /api/time-tracking/month-summary?year=&month=
// — the staff member's own Monatskarte (#1842).
func (rs *Resource) getOwnMonthSummary(w http.ResponseWriter, r *http.Request) {
	userClaims := jwt.ClaimsFromCtx(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	year, month, err := ParseYearMonthQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	summary, err := rs.WorkTimeMonthService.GetMonthSummary(r.Context(), staffID, year, month)
	if err != nil {
		if errors.Is(err, activeSvc.ErrMonthOutOfRange) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, summary, "Month summary retrieved successfully")
}
