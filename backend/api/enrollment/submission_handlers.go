package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SubmitChildRequest is the wire shape for a single child within the
// public submit body. Dates are ISO YYYY-MM-DD strings — the handler
// parses them.
type SubmitChildRequest struct {
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	DateOfBirth      string         `json:"date_of_birth"`
	TargetGradeLevel *int16         `json:"target_grade_level,omitempty"`
	CustomData       map[string]any `json:"custom_data,omitempty"`
	OfferingIDs      []int64        `json:"offering_ids,omitempty"`
}

// SubmitEnrollmentRequest is the public submit body. PhaseID identifies
// the parent's chosen enrollment phase (school year, holiday window,
// etc.). CaptchaToken is the Turnstile widget output; verified before
// any DB write.
type SubmitEnrollmentRequest struct {
	PhaseID           int64                `json:"phase_id"`
	GuardianFirstName string               `json:"guardian_first_name"`
	GuardianLastName  string               `json:"guardian_last_name"`
	GuardianEmail     string               `json:"guardian_email"`
	GuardianPhone     *string              `json:"guardian_phone,omitempty"`
	ConsentFlags      map[string]any       `json:"consent_flags,omitempty"`
	CustomData        map[string]any       `json:"custom_data,omitempty"`
	Children          []SubmitChildRequest `json:"children"`
	CaptchaToken      string               `json:"captcha_token,omitempty"`
}

// Bind defaults nil maps + slices to empty so downstream code doesn't
// have to nil-check.
func (req *SubmitEnrollmentRequest) Bind(_ *http.Request) error {
	if req.ConsentFlags == nil {
		req.ConsentFlags = map[string]any{}
	}
	if req.CustomData == nil {
		req.CustomData = map[string]any{}
	}
	if req.Children == nil {
		req.Children = []SubmitChildRequest{}
	}
	return nil
}

// SubmitEnrollmentResponse is what the public form receives after a
// successful submit. status_url is the link the parent receives by
// email; we return it inline too so the confirmation page can show it
// without waiting for the email.
type SubmitEnrollmentResponse struct {
	RequestID string `json:"request_id"`
	StatusURL string `json:"status_url"`
}

// submitEnrollment is the public submission handler. Verifies the
// captcha, resolves the slug to a tenant, runs the submission service
// inside that tenant's tx, then returns a status URL.
func (rs *Resource) submitEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.CaptchaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment submit not configured")))
		return
	}

	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	wireReq := &SubmitEnrollmentRequest{}
	if err := render.Bind(r, wireReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	remoteIP := remoteIPFromRequest(r)

	// Resolve tenant and run submit inside the tenant's tx. RLS on
	// enrollment.* tables narrows writes to the resolved tenant.
	var (
		result    *enrollmentService.SubmitResult
		submitErr error
	)
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)

		// Captcha gate runs before the DB write.
		if err := rs.CaptchaService.Verify(tenantCtx, wireReq.CaptchaToken, remoteIP); err != nil {
			submitErr = fmt.Errorf("captcha: %w", err)
			return nil
		}

		serviceReq, parseErr := buildServiceRequest(wireReq, school.ID, remoteIP)
		if parseErr != nil {
			submitErr = parseErr
			return nil
		}

		// Hand off to the service; it manages its own inner tx via
		// TxHandler.RunInTx, picking up the tenant context we set.
		res, err := rs.RequestService.Submit(tenantCtx, serviceReq)
		if err != nil {
			submitErr = err
			return nil
		}
		result = res
		return nil
	})
	if resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(resolveErr))
		return
	}
	if submitErr != nil {
		mapSubmitError(w, r, submitErr)
		return
	}

	resp := SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}

// buildServiceRequest converts the wire request into the service-layer
// shape. Parses date strings; surfaces a typed error on bad input.
func buildServiceRequest(wireReq *SubmitEnrollmentRequest, tenantID int64, remoteIP string) (enrollmentService.SubmitRequest, error) {
	out := enrollmentService.SubmitRequest{
		TenantID:          tenantID,
		PhaseID:           wireReq.PhaseID,
		RemoteIP:          remoteIP,
		GuardianFirstName: wireReq.GuardianFirstName,
		GuardianLastName:  wireReq.GuardianLastName,
		GuardianEmail:     wireReq.GuardianEmail,
		GuardianPhone:     wireReq.GuardianPhone,
		ConsentFlags:      wireReq.ConsentFlags,
		CustomData:        wireReq.CustomData,
	}
	for i, c := range wireReq.Children {
		dob, err := time.Parse("2006-01-02", c.DateOfBirth)
		if err != nil {
			return out, fmt.Errorf("child %d: invalid date_of_birth (expected YYYY-MM-DD)", i)
		}
		out.Children = append(out.Children, enrollmentService.SubmitChild{
			FirstName:        c.FirstName,
			LastName:         c.LastName,
			DateOfBirth:      dob,
			TargetGradeLevel: c.TargetGradeLevel,
			CustomData:       c.CustomData,
			OfferingIDs:      c.OfferingIDs,
		})
	}
	return out, nil
}

// mapSubmitError translates service-layer sentinel errors into HTTP
// status codes. Unknown errors fall through to 500.
func mapSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingClosed),
		errors.Is(err, enrollmentService.ErrInvalidSubmission):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingFull):
		// 409 Conflict: the request is well-formed but a selected
		// offering is at capacity and the tenant's overflow mode is
		// 'reject'. Parents should re-pick or wait.
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, enrollmentService.ErrDuplicateEnrollment):
		// 409 Conflict: the same guardian email already has an active
		// (non-rejected, non-withdrawn) enrollment for one of these
		// children in this phase.
		http.Error(w, "Für dieses Kind liegt in dieser Phase bereits eine Anmeldung vor.", http.StatusConflict)
	case errors.Is(err, enrollmentService.ErrRateLimited):
		// 429 Too Many Requests. Hard-coded retry hint avoids leaking
		// the exact remaining seconds.
		w.Header().Set("Retry-After", "3600")
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		// Capture captcha-shaped errors built with fmt.Errorf above.
		if strings.Contains(err.Error(), "captcha") {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// remoteIPFromRequest extracts the client IP for captcha verification +
// future rate limiting. Honors X-Forwarded-For (first hop) when set.
func remoteIPFromRequest(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.IndexByte(xff, ','); comma >= 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// --- status / edit / withdraw handlers (token-gated, public) ---

// StatusResponse is the public status-page payload. ChildID is
// stringified so the frontend keeps its int64-as-string convention.
type StatusResponse struct {
	RequestID         string                `json:"request_id"`
	GuardianFirstName string                `json:"guardian_first_name"`
	GuardianLastName  string                `json:"guardian_last_name"`
	GuardianEmail     string                `json:"guardian_email"`
	GuardianPhone     *string               `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time             `json:"submitted_at"`
	WithdrawnAt       *time.Time            `json:"withdrawn_at,omitempty"`
	Children          []StatusChildResponse `json:"children"`
}

// StatusChildResponse is one row in StatusResponse.Children.
type StatusChildResponse struct {
	ID           string  `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
}

// getStatus returns the per-child status for a token-bearing parent.
// Public route — caller must wrap in admin-tx because there's no JWT.
func (rs *Resource) getStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}

	var (
		req      *enrollmentModels.Request
		children []*enrollmentModels.RequestChild
	)
	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		serviceReq, serviceChildren, err := rs.RequestService.GetByStatusToken(adminCtx, token)
		if err != nil {
			return err
		}
		req = serviceReq
		children = serviceChildren
		return nil
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	resp := StatusResponse{
		RequestID:         strconv.FormatInt(req.ID, 10),
		GuardianFirstName: req.GuardianFirstName,
		GuardianLastName:  req.GuardianLastName,
		GuardianEmail:     req.GuardianEmail,
		GuardianPhone:     req.GuardianPhone,
		SubmittedAt:       req.SubmittedAt,
		WithdrawnAt:       req.WithdrawnAt,
	}
	for _, c := range children {
		resp.Children = append(resp.Children, StatusChildResponse{
			ID:           strconv.FormatInt(c.ID, 10),
			FirstName:    c.FirstName,
			LastName:     c.LastName,
			Status:       c.Status,
			StatusReason: c.StatusReason,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Status retrieved")
}

// EditPatchRequest is the wire shape for PATCH /requests/{token}.
type EditPatchRequest struct {
	GuardianFirstName *string        `json:"guardian_first_name,omitempty"`
	GuardianLastName  *string        `json:"guardian_last_name,omitempty"`
	GuardianPhone     *string        `json:"guardian_phone,omitempty"`
	ConsentFlags      map[string]any `json:"consent_flags,omitempty"`
	CustomData        map[string]any `json:"custom_data,omitempty"`
}

// Bind makes EditPatchRequest a render.Binder so the chi binder helper
// can decode request bodies into it without ad-hoc json.Decoder code.
func (req *EditPatchRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) patchStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}
	patchReq := &EditPatchRequest{}
	if err := render.Bind(r, patchReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	patch := enrollmentService.EditPatch{
		GuardianFirstName: patchReq.GuardianFirstName,
		GuardianLastName:  patchReq.GuardianLastName,
		GuardianPhone:     patchReq.GuardianPhone,
		ConsentFlags:      patchReq.ConsentFlags,
		CustomData:        patchReq.CustomData,
	}
	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		return rs.RequestService.Edit(adminCtx, token, patch)
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, enrollmentService.ErrEditNotAllowed):
			common.RenderError(w, r, common.ErrorForbidden(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"message": "updated"}, "Request updated")
}

// WithdrawRequest is the wire shape for POST /requests/{token}/withdraw.
// Optional child_id; omit to withdraw every non-terminal child.
type WithdrawRequest struct {
	ChildID *string `json:"child_id,omitempty"`
}

func (req *WithdrawRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) withdrawStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}

	body := &WithdrawRequest{}
	// The body is optional; render.Bind requires a body but we tolerate
	// empty payloads (omit child_id = withdraw all). Decode manually.
	if r.Body != nil && r.ContentLength != 0 {
		if err := json.NewDecoder(r.Body).Decode(body); err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
			return
		}
	}
	var childID int64
	if body.ChildID != nil && *body.ChildID != "" {
		v, err := strconv.ParseInt(*body.ChildID, 10, 64)
		if err != nil || v <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid child_id")))
			return
		}
		childID = v
	}

	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		return rs.RequestService.Withdraw(adminCtx, token, childID)
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, enrollmentService.ErrWithdrawNotAllowed):
			common.RenderError(w, r, common.ErrorForbidden(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.RespondNoContent(w, r)
}
