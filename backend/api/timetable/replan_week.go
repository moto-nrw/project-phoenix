// Package timetable — re-plan-week handler (WP-B9).
//
// POST /instances/re-plan-week deletes protected-status='planned' non-spontaneous
// instances in a date window and re-materializes. active/completed/cancelled
// rows and spontaneous planned rows survive. Admin-only (SchedulesManage).
package timetable

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
)

// replanWeekRequest mirrors materializeRequest: both date fields optional,
// both-or-neither rule, default = next Mon–Sun when omitted. Shared body
// shape keeps admin UIs from having to special-case two near-identical
// endpoints.
type replanWeekRequest struct {
	FromDate *string `json:"from_date,omitempty"`
	ToDate   *string `json:"to_date,omitempty"`
	// ActivityGroupID optionally narrows the re-plan to one template's
	// planned instances (WP-B3 template split). Omitted/null = whole grid.
	ActivityGroupID *int64 `json:"activity_group_id,omitempty"`
}

// Bind is a no-op. Same rationale as materializeRequest.Bind: date parsing
// and both-or-neither validation happen inside the handler via the shared
// resolveMaterializationWindow helper, because chi/render rejects an empty
// body when Bind returns a non-nil error, breaking the "no body = defaults"
// path.
func (req *replanWeekRequest) Bind(_ *http.Request) error {
	return nil
}

// replanWeekResponse surfaces both the delete count and the materialization
// counts so admins can sanity-check the operation.
type replanWeekResponse struct {
	From                      string `json:"from"`
	To                        string `json:"to"`
	DeletedInstances          int    `json:"deleted_instances"`
	InstancesCreated          int    `json:"instances_created"`
	CandidatesSkippedExisting int    `json:"candidates_skipped_existing"`
	InstanceStaffCreated      int    `json:"instance_staff_created"`
	InstanceStudentsCreated   int    `json:"instance_students_created"`
	DurationMS                int64  `json:"duration_ms"`
}

// replanWeek handles POST /instances/re-plan-week.
func (rs *Resource) replanWeek(w http.ResponseWriter, r *http.Request) {
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	req := &replanWeekRequest{}
	if r.ContentLength > 0 {
		if err := render.Bind(r, req); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}

	// The shared window helper enforces the same validation as /materialize:
	// ≤ 56 days, from ≤ to, both-or-neither. Keep the rule in one place.
	bodyForWindow := &materializeRequest{FromDate: req.FromDate, ToDate: req.ToDate}
	from, to, err := resolveMaterializationWindow(bodyForWindow, time.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	result, err := rs.InstanceService.ReplanWeek(r.Context(), from, to, req.ActivityGroupID, jwt.ActorAccountIDFromCtx(r.Context()))
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("replan week failed", err))
		return
	}

	resp := replanWeekResponse{
		From:             result.From.Format(dateLayout),
		To:               result.To.Format(dateLayout),
		DeletedInstances: result.DeletedInstances,
	}
	if result.Materialization != nil {
		resp.InstancesCreated = result.Materialization.InstancesCreated
		resp.CandidatesSkippedExisting = result.Materialization.CandidatesSkippedExisting
		resp.InstanceStaffCreated = result.Materialization.InstanceStaffCreated
		resp.InstanceStudentsCreated = result.Materialization.InstanceStudentsCreated
		resp.DurationMS = result.Materialization.DurationMS
	}
	common.Respond(w, r, http.StatusOK, resp, "Week re-planned")
}
