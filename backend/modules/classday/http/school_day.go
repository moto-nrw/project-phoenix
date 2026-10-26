package classdayhttp

import (
	"errors"
	"net/http"
	"strings"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
)

// School-wide day overview of moto schule (#4802): the Schulverwaltung sees
// the day's numbers of every class at once. The route needs class_day:read
// and class_day:all_classes; the projection reads the whole school in one
// pass and records the access.

// errSchoolDayNotConfigured answers when the projection was composed without
// the school-wide overview.
var errSchoolDayNotConfigured = errors.New("school day overview is not configured")

// getSchoolDay handles GET /school/class-day/school-day?date=YYYY-MM-DD
// (default today).
func (rs *Resource) getSchoolDay(w http.ResponseWriter, r *http.Request) {
	claims := sessiontoken.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(errors.New("invalid token")))
		return
	}
	if rs.SchoolDay == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errSchoolDayNotConfigured))
		return
	}
	date, ok := dateQueryOrToday(w, r)
	if !ok {
		return
	}
	actor := classday.Actor{AccountID: int64(claims.ID), Roles: strings.Join(claims.Roles, ",")}
	report, err := rs.SchoolDay.SchoolDayReport(r.Context(), classday.Date(date.String()), actor)
	if err != nil {
		rs.logger.Error("class day: school day overview failed",
			"date", date.String(),
			"error", err.Error(),
		)
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, report, "School day retrieved successfully")
}
