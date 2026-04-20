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
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
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
}

// startInstance handles POST /instances/{id}/start.
func (rs *Resource) startInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.instanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	// started_by is optional: resolve staff id from the JWT account, but do
	// not reject the request if the lookup fails — the transition itself is
	// permission-gated upstream, and a missing staff mapping is a data issue
	// the admin should not see as a 500.
	startedByStaffID := rs.resolveStartedByStaffID(r.Context())

	result, err := rs.instanceService.Start(r.Context(), id, startedByStaffID)
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
	if rs.instanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	instance, err := rs.instanceService.Complete(r.Context(), id)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	resp := InstanceStatusResponse{InstanceID: instance.ID, Status: instance.Status}
	if instance.CompletedAt != nil {
		resp.CompletedAt = instance.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance completed")
}

// cancelInstance handles POST /instances/{id}/cancel.
func (rs *Resource) cancelInstance(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid instance id")))
		return
	}
	if rs.instanceService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance service not wired")))
		return
	}

	instance, err := rs.instanceService.Cancel(r.Context(), id)
	if err != nil {
		renderInstanceLifecycleError(w, r, err)
		return
	}

	resp := InstanceStatusResponse{InstanceID: instance.ID, Status: instance.Status}
	if instance.CompletedAt != nil {
		resp.CompletedAt = instance.CompletedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	common.Respond(w, r, http.StatusOK, resp, "Instance cancelled")
}

// renderInstanceLifecycleError maps service-layer sentinel errors to HTTP
// status codes. Unknown errors fall through to 500 to avoid leaking a
// potentially wrong 4xx for a real database failure.
func renderInstanceLifecycleError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrInstanceNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrInvalidInstanceTransition):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "invalid_transition"))
	default:
		common.RenderError(w, r, common.ErrorInternalServerWrap("instance lifecycle failed", err))
	}
}

// resolveStartedByStaffID is best-effort: if the JWT carries an account that
// resolves to a staff row, return its id. Missing claims, missing person, or
// missing staff row all return 0 — StartedBy is nullable in the schema and
// the transition is already authorised via SchedulesManage.
func (rs *Resource) resolveStartedByStaffID(ctx context.Context) int64 {
	if rs.personService == nil {
		return 0
	}
	claims, ok := ctx.Value(jwt.CtxClaims).(jwt.AppClaims)
	if !ok {
		return 0
	}
	accountID := int64(claims.ID)
	person, err := rs.personService.FindByAccountID(ctx, accountID)
	if err != nil || person == nil {
		rs.getLogger().Debug("started_by lookup: person not found",
			slog.Int64("account_id", accountID),
		)
		return 0
	}
	staffRepo := rs.personService.StaffRepository()
	if staffRepo == nil {
		return 0
	}
	staff, err := staffRepo.FindByPersonID(ctx, person.ID)
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
	if rs.logger != nil {
		return rs.logger
	}
	return slog.Default()
}
