package timetracking

import (
	"errors"
	"net/http"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

// ParseDateRangeQuery extracts and validates the from/to query parameters of a
// schedule-targets request. Exported for reuse by the staff-scoped admin
// endpoint.
func ParseDateRangeQuery(r *http.Request) (timezone.Date, timezone.Date, error) {
	from, err := timezone.ParseDate(r.URL.Query().Get("from"))
	if err != nil {
		return timezone.Date(""), timezone.Date(""), errors.New("from query parameter must be YYYY-MM-DD")
	}
	to, err := timezone.ParseDate(r.URL.Query().Get("to"))
	if err != nil {
		return timezone.Date(""), timezone.Date(""), errors.New("to query parameter must be YYYY-MM-DD")
	}
	if to.Before(from) {
		return timezone.Date(""), timezone.Date(""), errors.New("to must be on or after from")
	}
	return from, to, nil
}

// getOwnScheduleTargets handles
// GET /api/time-tracking/schedule-targets?from=&to= — the per-day projection
// (Soll, Gutschrift, Ist, Saldo) for the staff member's own daily table (#1842,
// #2443). The table used to apply the CURRENT schedule to every rendered date
// and to derive its Saldo as "Ist minus Soll", which contradicted the
// Monatskarte above it as soon as someone's contracted hours changed or a day
// carried an absence instead of a work session.
func (rs *Resource) getOwnScheduleTargets(w http.ResponseWriter, r *http.Request) {
	userClaims := rs.identity(r.Context())
	staffID, err := rs.getStaffIDFromClaims(r.Context(), userClaims)
	if err != nil {
		common.RenderError(w, r, common.ErrorUnauthorized(err))
		return
	}

	from, to, err := ParseDateRangeQuery(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	projection, err := rs.WorkTimeMonthService.DailyProjection(r.Context(), staffID, from.String(), to.String())
	if err != nil {
		if errors.Is(err, workforce.ErrInvalidTargetRange) {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	common.Respond(w, r, http.StatusOK, projection, "Daily projection retrieved successfully")
}
