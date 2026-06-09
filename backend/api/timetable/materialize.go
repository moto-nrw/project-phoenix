package timetable

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// materializeRequest is the optional body shape for the manual endpoint.
// Both dates are optional — if absent, the handler defaults to the next
// Monday–Sunday window (same rule the scheduler uses with weeks_ahead=1).
// If only one is present, reject with 400 rather than guessing.
type materializeRequest struct {
	FromDate *string `json:"from_date,omitempty"`
	ToDate   *string `json:"to_date,omitempty"`
}

// Bind is a no-op to satisfy chi/render's Bind contract — validation happens
// in the handler after date parsing because we need to compute defaults when
// fields are omitted. chi/render rejects an empty body when Bind returns an
// error, which would prevent the "no body = defaults" path.
func (req *materializeRequest) Bind(_ *http.Request) error {
	return nil
}

// materializeResponse mirrors scheduleSvc.MaterializationResult with the
// fields surfaced to API consumers. Admin UIs can show the counts directly;
// operators can use them to sanity-check a run.
type materializeResponse struct {
	From                        string               `json:"from"`
	To                          string               `json:"to"`
	InstancesCreated            int                  `json:"instances_created"`
	CandidatesSkippedExisting   int                  `json:"candidates_skipped_existing"`
	CandidatesSkippedException  int                  `json:"candidates_skipped_exception"`
	CandidatesSkippedABWeek     int                  `json:"candidates_skipped_ab_week"`
	CandidatesSkippedNoPeriod   int                  `json:"candidates_skipped_no_period"`
	CandidatesSkippedIncomplete int                  `json:"candidates_skipped_incomplete"`
	CandidatesRaced             int                  `json:"candidates_raced"`
	InstanceStudentsCreated     int                  `json:"instance_students_created"`
	InstanceStaffCreated        int                  `json:"instance_staff_created"`
	Warnings                    []materializeWarning `json:"warnings"`
	DurationMS                  int64                `json:"duration_ms"`
}

// materializeWarning is the API shape for soft preconditions surfaced by
// the materialization service. Mirrors scheduleSvc.MaterializationWarning.
type materializeWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// materialize is POST /api/timetable/materialize. Admin-only (gated by
// permissions.SchedulesManage on the route). Manually triggers the same
// service the scheduler uses; no setting-gate check — admin override.
func (rs *Resource) materialize(w http.ResponseWriter, r *http.Request) {
	if rs.materializationService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("materialization service is not wired")))
		return
	}

	req := &materializeRequest{}
	// chi/render.Bind returns an error on empty body — tolerate that path by
	// only decoding when a body is present.
	if r.ContentLength > 0 {
		if err := render.Bind(r, req); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}

	from, to, err := resolveMaterializationWindow(req, time.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	rs.getLogger().Info("manual materialization requested",
		slog.Int64("tenant_id", tenant.FromContext(r.Context())),
		slog.String("from", from.Format(dateLayout)),
		slog.String("to", to.Format(dateLayout)),
	)

	result, err := rs.materializationService.MaterializeForTenant(
		r.Context(), from, to, scheduleSvc.MaterializationSourceManual,
	)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("materialization failed", err))
		return
	}

	warnings := make([]materializeWarning, 0, len(result.Warnings))
	for _, w := range result.Warnings {
		warnings = append(warnings, materializeWarning{Code: w.Code, Message: w.Message})
	}

	resp := materializeResponse{
		From:                        result.From.Format(dateLayout),
		To:                          result.To.Format(dateLayout),
		InstancesCreated:            result.InstancesCreated,
		CandidatesSkippedExisting:   result.CandidatesSkippedExisting,
		CandidatesSkippedException:  result.CandidatesSkippedException,
		CandidatesSkippedABWeek:     result.CandidatesSkippedABWeek,
		CandidatesSkippedNoPeriod:   result.CandidatesSkippedNoPeriod,
		CandidatesSkippedIncomplete: result.CandidatesSkippedIncomplete,
		CandidatesRaced:             result.CandidatesRaced,
		InstanceStudentsCreated:     result.InstanceStudentsCreated,
		InstanceStaffCreated:        result.InstanceStaffCreated,
		Warnings:                    warnings,
		DurationMS:                  result.DurationMS,
	}
	common.Respond(w, r, http.StatusOK, resp, "Materialization completed")
}

// resolveMaterializationWindow validates the body-or-default rule, turning a
// materializeRequest + server clock into a concrete (from, to) pair.
//
// Rules:
//   - Neither field present → default next Monday–Sunday (weeks_ahead=1).
//   - Both present → parse as YYYY-MM-DD civil dates; reject if from > to
//     or the span exceeds 56 days (8 weeks).
//   - Only one present → 400 (ambiguous, don't guess).
func resolveMaterializationWindow(req *materializeRequest, now time.Time) (from, to time.Time, err error) {
	bothNil := req.FromDate == nil && req.ToDate == nil
	bothSet := req.FromDate != nil && req.ToDate != nil

	if bothNil {
		// Mirror the scheduler's default window exactly — next Mon to following Sun.
		// ResolveWindow is wired via the scheduleSvc package to keep the rule in
		// one place, but at the HTTP layer we don't want to instantiate a service
		// just to compute it; re-derive it inline with the same formula.
		d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		var delta int
		switch d.Weekday() {
		case time.Sunday:
			delta = 1
		case time.Monday:
			delta = 7
		default:
			delta = int(time.Saturday-d.Weekday()) + 2
		}
		from = d.AddDate(0, 0, delta)
		to = from.AddDate(0, 0, 6)
		return from, to, nil
	}

	if !bothSet {
		return time.Time{}, time.Time{}, errors.New("from_date and to_date must both be present or both omitted")
	}

	from, err = time.Parse(dateLayout, *req.FromDate)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid from_date: expected YYYY-MM-DD")
	}
	to, err = time.Parse(dateLayout, *req.ToDate)
	if err != nil {
		return time.Time{}, time.Time{}, errors.New("invalid to_date: expected YYYY-MM-DD")
	}

	if to.Before(from) {
		return time.Time{}, time.Time{}, errors.New("to_date must not be before from_date")
	}
	days := int(to.Sub(from)/(24*time.Hour)) + 1
	if days > scheduleSvc.MaxMaterializationWindowDays {
		return time.Time{}, time.Time{}, errors.New("window exceeds 56 days (8 weeks)")
	}
	return from, to, nil
}
