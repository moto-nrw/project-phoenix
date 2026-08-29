// Package timetable — instance lifecycle handlers (WP-B9).
//
// Three POST endpoints per instance:
//
//	POST /instances/{id}/start     planned → active   (creates active.group bridge)
//	POST /instances/{id}/complete  active  → completed (ends active.group)
//	POST /instances/{id}/cancel    planned|active → cancelled (ends active.group if active)
//
// All three gated on permissions.SchedulesManage. All three run inside the
// shared TenantTxMiddleware tx — no per-handler transaction management.
package timetable

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/base"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

// StartInstanceResponse is the 200 body for POST /instances/{id}/start. Warnings
// is always present (empty array when clean) so clients can iterate without a
// nil check.
type StartInstanceResponse struct {
	InstanceID    int64                                 `json:"instance_id"`
	Status        string                                `json:"status"`
	ActiveGroupID int64                                 `json:"active_group_id"`
	StartedAt     string                                `json:"started_at"`
	Warnings      []scheduleSvc.InstanceConflictWarning `json:"warnings"`
}

// InstanceStatusResponse is the 200 body for complete and cancel — minimal
// because the frontend already has the full instance in its cache.
type InstanceStatusResponse struct {
	InstanceID  int64  `json:"instance_id"`
	Status      string `json:"status"`
	CompletedAt string `json:"completed_at,omitempty"`
	ReopenUntil string `json:"reopen_until,omitempty"`
	// GuardianNotice is set when a cancellation informed the families (#2601).
	GuardianNotice *GuardianNoticeResponse `json:"guardian_notice,omitempty"`
}

// GuardianNoticeRequest is the optional "Eltern informieren" part of a cancel
// body: the text written for the families. Both fields are required when the
// object is present; the internal reason travels separately.
type GuardianNoticeRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
}

// GuardianNoticeResponse reports what a sent notice reached.
type GuardianNoticeResponse struct {
	AnnouncementID int64 `json:"announcement_id"`
	ChildCount     int   `json:"child_count"`
	FamilyCount    int   `json:"family_count"`
}

// GuardianNoticeReachResponse is the preview for the cancel dialog.
type GuardianNoticeReachResponse struct {
	Enabled     bool `json:"enabled"`
	DefaultOn   bool `json:"default_on"`
	ChildCount  int  `json:"child_count"`
	FamilyCount int  `json:"family_count"`
}

func (req *GuardianNoticeRequest) toServiceInput() *scheduleSvc.GuardianNoticeInput {
	if req == nil {
		return nil
	}
	return &scheduleSvc.GuardianNoticeInput{
		Title:   strings.TrimSpace(req.Title),
		Message: strings.TrimSpace(req.Message),
	}
}

func guardianNoticeResponseOf(result *scheduleSvc.GuardianNoticeResult) *GuardianNoticeResponse {
	if result == nil {
		return nil
	}
	return &GuardianNoticeResponse{
		AnnouncementID: result.AnnouncementID,
		ChildCount:     result.ChildCount,
		FamilyCount:    result.FamilyCount,
	}
}

// guardianNoticeReach handles GET /instances/{id}/guardian-notice: the cancel
// dialog asks before sending whether the school allows the notice, whether
// the checkbox starts ticked, and how many families it would reach.
func (rs *Resource) guardianNoticeReach(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}
	reach, err := rs.InstanceService.GuardianNoticeReachFor(r.Context(), id)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, GuardianNoticeReachResponse{
		Enabled:     reach.Enabled,
		DefaultOn:   reach.DefaultOn,
		ChildCount:  reach.ChildCount,
		FamilyCount: reach.FamilyCount,
	}, "Guardian notice reach retrieved")
}

// startInstance handles POST /instances/{id}/start.
func (rs *Resource) startInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	// started_by is optional: resolve staff id from the JWT account, but do
	// not reject the request if the lookup fails — the transition itself is
	// permission-gated upstream, and a missing staff mapping is a data issue
	// the admin should not see as a 500.
	startedByStaffID := rs.resolveStartedByStaffID(r.Context())

	result, err := rs.InstanceService.Start(r.Context(), id, startedByStaffID)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	resp := StartInstanceResponse{
		InstanceID:    result.Instance.ID,
		Status:        result.Instance.Status,
		ActiveGroupID: result.ActiveGroupID,
		Warnings:      result.Warnings,
	}
	if result.Warnings == nil {
		resp.Warnings = []scheduleSvc.InstanceConflictWarning{}
	}
	if result.Instance.StartedAt != nil {
		resp.StartedAt = result.Instance.StartedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance started")
}

// completeInstance handles POST /instances/{id}/complete.
func (rs *Resource) completeInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	ctx := scheduleSvc.WithLifecycleActor(r.Context(), int64(claims.ID))
	// Planner complete has no live visit roster. An empty body (the historic
	// e2e contract) is accepted; confirmation stays on the operations path.
	instance, err := rs.InstanceService.Complete(ctx, id)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	resp := InstanceStatusResponse{InstanceID: instance.ID, Status: instance.Status}
	if instance.CompletedAt != nil {
		resp.CompletedAt = instance.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	if instance.ReopenUntil != nil {
		resp.ReopenUntil = instance.ReopenUntil.UTC().Format("2006-01-02T15:04:05Z")
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance completed")
}

func (rs *Resource) reopenInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.InstanceService.Reopen(r.Context(), id, int64(claims.ID), common.HasEffectiveAdminScope(r.Context()))
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, StartInstanceResponse{InstanceID: result.Instance.ID, Status: result.Instance.Status, ActiveGroupID: result.ActiveGroupID, Warnings: result.Warnings}, "Instance reopened")
}

// cancelInstance handles POST /instances/{id}/cancel.
func (rs *Resource) cancelInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	// Optional {reason, guardian_notice} body (#1840, #2601). No body / empty
	// body → nil reason and no notice, so the shared cancel keeps working for
	// callers that send nothing.
	var reason *string
	var notice *GuardianNoticeRequest
	if r.Body != nil {
		var body struct {
			Reason         *string                `json:"reason,omitempty"`
			GuardianNotice *GuardianNoticeRequest `json:"guardian_notice,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid JSON body")))
			return
		}
		reason = trimReason(body.Reason)
		notice = body.GuardianNotice
	}

	result, err := rs.InstanceService.CancelWithNotice(r.Context(), scheduleSvc.CancelInstanceInput{
		InstanceID:     id,
		Reason:         reason,
		ActorAccountID: jwt.ActorAccountIDFromCtx(r.Context()),
		GuardianNotice: notice.toServiceInput(),
	})
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}
	instance := result.Instance

	resp := InstanceStatusResponse{InstanceID: instance.ID, Status: instance.Status, GuardianNotice: guardianNoticeResponseOf(result.GuardianNotice)}
	if instance.CompletedAt != nil {
		resp.CompletedAt = instance.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance cancelled")
}

// deleteInstance handles DELETE /instances/{id}. Planned and cancelled
// instances can be deleted; active/completed history stays protected and
// returns the same 409 contract as lifecycle transitions.
func (rs *Resource) deleteInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.InstanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	if err := rs.InstanceService.DeleteCancelled(r.Context(), id); err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	common.RespondNoContent(w, r)
}

// conflictCode renders the error itself as a 409 carrying the given code.
func conflictCode(code string) func(error) render.Renderer {
	return func(err error) render.Renderer {
		return common.ErrorConflictWithCode(err, code)
	}
}

// staticConflict renders a fixed message as a 409 carrying the given code,
// hiding the raw error from the response.
func staticConflict(message, code string) func(error) render.Renderer {
	return func(error) render.Renderer {
		return common.ErrorConflictWithCode(errors.New(message), code)
	}
}

// instanceLifecycleErrorRules maps service-layer sentinel errors to HTTP
// status codes. Unknown errors fall through to 500 to avoid leaking a
// potentially wrong 4xx for a real database failure.
var instanceLifecycleErrorRules = []common.ErrorRule{
	{Target: scheduleSvc.ErrInstanceNotFound, Render: common.ErrorNotFound},
	{
		Match: func(err error) bool {
			return errors.Is(err, scheduleSvc.ErrInvalidInstanceReference) ||
				errors.Is(err, scheduleSvc.ErrInstanceWeekend) ||
				errors.Is(err, scheduleSvc.ErrInstanceOutsideActiveCalendarPeriod) ||
				errors.Is(err, scheduleSvc.ErrGuardianNoticeInvalid)
		},
		Render: common.ErrorInvalidRequest,
	},
	{Target: scheduleSvc.ErrGuardianNoticeDisabled, Render: conflictCode("guardian_notice_disabled")},
	{
		Target: scheduleSvc.ErrInstanceMoved,
		Render: staticConflict("block was changed concurrently; reopen it and try again", "instance_moved"),
	},
	{Target: scheduleSvc.ErrInvalidInstanceTransition, Render: conflictCode("invalid_transition")},
	{Target: scheduleSvc.ErrInstanceStartTooEarly, Render: conflictCode("start_too_early")},
	{Target: scheduleSvc.ErrInstanceStartExpired, Render: conflictCode("start_window_expired")},
	{Target: scheduleSvc.ErrInstanceCompleteEarly, Render: conflictCode("complete_too_early")},
	{Target: scheduleSvc.ErrCompletionConfirmationStale, Render: conflictCode("completion_confirmation_stale")},
	{Target: scheduleSvc.ErrTimetableOperationForbidden, Render: common.ErrorForbidden},
	{
		Match: func(err error) bool {
			return errors.Is(err, scheduleSvc.ErrTimetableOperationConflict) ||
				errors.Is(err, activeSvc.ErrStudentAlreadyActive) ||
				errors.Is(err, activeSvc.ErrRoomConflict) ||
				errors.Is(err, activeSvc.ErrRoomCapacityExceeded)
		},
		Render: common.ErrorConflict,
	},
	{
		Target: scheduleSvc.ErrUnderstaffedAckStillStaffed,
		Render: staticConflict(
			"dieser Block kann nicht als bewusst unbesetzt markiert werden, solange noch Personal eingeteilt ist",
			"understaffed_still_staffed",
		),
	},
	{
		Target: scheduleSvc.ErrAmbiguousTemplateInstanceDelete,
		Render: staticConflict(
			"dieser Termin kann nicht einzeln gelöscht werden, weil die Vorlage an diesem Tag mehrere Termine hat",
			"ambiguous_template_instance_delete",
		),
	},
	{
		Match: func(err error) bool {
			return base.IsUniqueViolationOn(err, "idx_activity_instances_template_unique")
		},
		Render: staticConflict("instance already exists for this template/date/start_time", "duplicate_instance"),
	},
}

func renderInstanceLifecycleError(w http.ResponseWriter, r *http.Request, err error) {
	common.RenderError(w, r, common.RenderWithRules(err, instanceLifecycleErrorRules, func(err error) render.Renderer {
		return common.ErrorInternalServerWrap("instance lifecycle failed", err)
	}))
}

// resolveStartedByStaffID is best-effort: if the JWT carries an account that
// resolves to a staff row, return its id. Missing claims, missing person, or
// missing staff row all return 0 — StartedBy is nullable in the schema and
// the transition is already authorised via SchedulesManage.
func (rs *Resource) resolveStartedByStaffID(ctx context.Context) int64 {
	if rs.PersonService == nil {
		return 0
	}
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0
	}
	accountID := int64(claims.ID)
	person, err := rs.PersonService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		rs.getLogger().Debug("started_by lookup: person not found",
			slog.Int64("account_id", accountID),
		)
		return 0
	}
	staff, err := rs.PersonService.GetStaffByPersonID(ctx, person.ID)
	if err != nil || staff == nil {
		rs.getLogger().Debug("started_by lookup: staff not found",
			slog.Int64("person_id", person.ID),
		)
		return 0
	}
	return staff.ID
}

// getLogger is a nil-safe accessor used by helpers that run outside the
// chi handler's standard error-rendering path.
func (rs *Resource) getLogger() *slog.Logger {
	return cmp.Or(rs.Logger, slog.Default())
}
