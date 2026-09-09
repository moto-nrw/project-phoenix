package timetracking

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
)

var auditLogErrorRules = []common.ErrorRule{
	{Target: workforce.ErrAuditLogInvalid, Render: common.ErrorInvalidRequest},
}

// getTimeTrackingAuditLog handles GET /api/staff/time-tracking/audit-log
// (#1417): the cross-staff audit feed, filtered by affected staff, acting
// staff, date range, and source, keyset-paginated via an opaque cursor.
func (rs *StaffAdminResource) getTimeTrackingAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := workforce.AuditLogListRequest{Cursor: q.Get("cursor")}

	var err error
	if req.From, err = parseOptionalDate(q.Get("from")); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid from date format, expected YYYY-MM-DD")))
		return
	}
	if req.To, err = parseOptionalDate(q.Get("to")); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid to date format, expected YYYY-MM-DD")))
		return
	}
	if req.StaffID, err = parseOptionalInt64(q.Get("staff_id")); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid staff_id")))
		return
	}
	if req.ActorStaffID, err = parseOptionalInt64(q.Get("actor_id")); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid actor_id")))
		return
	}
	if limit := q.Get("limit"); limit != "" {
		parsed, err := strconv.Atoi(limit)
		if err != nil || parsed <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid limit")))
			return
		}
		req.Limit = parsed
	}
	if sources := strings.TrimSpace(q.Get("sources")); sources != "" {
		req.Sources = strings.Split(sources, ",")
	}

	page, err := rs.AuditLogService.ListAuditLog(r.Context(), req)
	if err != nil {
		common.RenderError(w, r, common.RenderWithRules(err, auditLogErrorRules, common.ErrorInternalServer))
		return
	}
	common.Respond(w, r, http.StatusOK, page, "Audit log retrieved")
}

// parseOptionalDate validates an optional YYYY-MM-DD query value; the
// capability takes the empty string as "no bound".
func parseOptionalDate(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	date, err := timezone.ParseDate(raw)
	if err != nil {
		return "", err
	}
	return date.String(), nil
}

func parseOptionalInt64(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, errors.New("invalid id")
	}
	return parsed, nil
}
