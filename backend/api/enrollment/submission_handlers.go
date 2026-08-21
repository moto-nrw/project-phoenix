package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/clientip"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SubmitChildRequest is the wire shape for a single child within the
// public submit body. Dates are ISO YYYY-MM-DD strings — the handler
// parses them.
//
// OfferingDays is an optional parallel array to OfferingIDs that
// carries the parent's per-day selections for offerings whose
// days_of_week_mode is "parent_choice". When omitted, the offering
// runs in its default (admin-fixed) day set. The service enforces
// that any entry here references an id from OfferingIDs and that
// selected_days is a non-empty subset of the offering's
// available_days.
type SubmitChildRequest struct {
	ID                *int64                  `json:"id,omitempty,string"`
	FirstName         string                  `json:"first_name"`
	LastName          string                  `json:"last_name"`
	DateOfBirth       string                  `json:"date_of_birth"`
	TargetGradeLevel  *int16                  `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                 `json:"target_school_class,omitempty"`
	CustomData        map[string]any          `json:"custom_data,omitempty"`
	OfferingIDs       []int64                 `json:"offering_ids,omitempty"`
	OfferingDays      []SubmitOfferingDaysRow `json:"offering_days,omitempty"`
}

// SubmitOfferingDaysRow is one row of SubmitChildRequest.OfferingDays.
// OfferingID must also appear in the sibling OfferingIDs list — the
// service uses OfferingIDs as the authoritative "this parent picked
// these offerings" set; OfferingDays only refines the day selection
// for parent_choice offerings.
type SubmitOfferingDaysRow struct {
	OfferingID   int64    `json:"offering_id"`
	SelectedDays []string `json:"selected_days"`
}

// SubmitGuardianRequest is the wire shape for one additional guardian
// (co-guardian) the parent added beyond the primary guardian. Only the
// names are required; email and phone are optional.
type SubmitGuardianRequest struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// SubmitEnrollmentRequest is the public submit body. PhaseID identifies
// the parent's chosen enrollment phase (school year, holiday window,
// etc.). CaptchaToken is the Turnstile widget output; verified before
// any DB write.
type SubmitEnrollmentRequest struct {
	PhaseID             int64                   `json:"phase_id"`
	GuardianFirstName   string                  `json:"guardian_first_name"`
	GuardianLastName    string                  `json:"guardian_last_name"`
	GuardianEmail       string                  `json:"guardian_email"`
	GuardianPhone       *string                 `json:"guardian_phone,omitempty"`
	AdditionalGuardians []SubmitGuardianRequest `json:"additional_guardians,omitempty"`
	ConsentFlags        map[string]any          `json:"consent_flags,omitempty"`
	CustomData          map[string]any          `json:"custom_data,omitempty"`
	Children            []SubmitChildRequest    `json:"children"`
	CaptchaToken        string                  `json:"captcha_token,omitempty"`
	LateInviteToken     string                  `json:"late_invite_token,omitempty"`
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
	if req.AdditionalGuardians == nil {
		req.AdditionalGuardians = []SubmitGuardianRequest{}
	}
	return nil
}

// SubmitEnrollmentResponse is what the public form receives after a
// successful submit. status_url is the link the parent receives by
// email; we return it inline too so the confirmation page can show it
// without waiting for the email.
type SubmitEnrollmentResponse struct {
	RequestID string                                `json:"request_id"`
	StatusURL string                                `json:"status_url"`
	Warnings  []enrollmentService.SubmissionWarning `json:"warnings,omitempty"`
}

// submitEnrollment is the public submission handler. Verifies the
// captcha, resolves the slug to a tenant, runs the submission service
// inside that tenant's tx, then returns a status URL.
func (rs *Resource) submitEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil || rs.CaptchaService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment submit not configured")))
		return
	}

	slug, wireReq, err := bindSubmitEnrollmentRequest(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	remoteIP := remoteIPFromRequest(r)

	// Resolve tenant and run submit inside the tenant's tx. RLS on
	// enrollment.* tables narrows writes to the resolved tenant.
	//
	// Every failure inside the closure MUST be returned from it: the
	// service's inner TxHandler.RunInTx reuses this outer transaction and
	// cannot roll back by itself, so swallowing the error here would
	// commit partial writes while the client receives an error response.
	// submitErr remembers which failures belong to the submit flow so the
	// post-tx mapping can distinguish them from tenant-resolve failures.
	var (
		result    *enrollmentService.SubmitResult
		submitErr error
	)
	schoolID, resolveErr := rs.resolvePublicTenantID(r.Context(), slug)
	if resolveErr == nil {
		resolveErr = tenant.WithTenantTx(r.Context(), rs.db, schoolID, func(tenantCtx context.Context, _ bun.Tx) error {
			// Captcha gate runs before the DB write.
			if err := rs.CaptchaService.Verify(tenantCtx, wireReq.CaptchaToken, remoteIP); err != nil {
				submitErr = fmt.Errorf("captcha: %w", err)
				return submitErr
			}

			serviceReq, parseErr := BuildServiceRequest(wireReq, schoolID, remoteIP)
			if parseErr != nil {
				submitErr = parseErr
				return submitErr
			}

			// Hand off to the service; it joins the tenant transaction we
			// opened, so returning its error rolls the whole submit back.
			res, err := rs.RequestService.Submit(tenantCtx, serviceReq)
			if err != nil {
				submitErr = err
				return submitErr
			}
			result = res
			return nil
		})
	}
	// submitErr wins over resolveErr: when the closure failed, resolveErr
	// carries the same error and the submit-specific mapping applies.
	if submitErr != nil {
		MapSubmitError(w, r, submitErr)
		return
	}
	if resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(resolveErr))
		return
	}

	resp := SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
		Warnings:  result.Warnings,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}

func bindSubmitEnrollmentRequest(r *http.Request) (string, *SubmitEnrollmentRequest, error) {
	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		return "", nil, errors.New("tenant slug is required")
	}
	request := &SubmitEnrollmentRequest{}
	if err := render.Bind(r, request); err != nil {
		return "", nil, err
	}
	if request.LateInviteToken == "" {
		request.LateInviteToken = lateInviteTokenFromRequest(r)
	}
	return slug, request, nil
}

// BuildServiceRequest converts the wire request into the service-layer
// shape. Parses date strings; surfaces a typed error on bad input.
func BuildServiceRequest(wireReq *SubmitEnrollmentRequest, tenantID int64, remoteIP string) (enrollmentService.SubmitRequest, error) {
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
		LateInviteToken:   wireReq.LateInviteToken,
	}
	for _, g := range wireReq.AdditionalGuardians {
		out.AdditionalGuardians = append(out.AdditionalGuardians, enrollmentService.SubmitGuardian{
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Email:     g.Email,
			Phone:     g.Phone,
		})
	}
	for i, c := range wireReq.Children {
		dob, err := timezone.ParseDate(c.DateOfBirth)
		if err != nil {
			return out, fmt.Errorf("child %d: invalid date_of_birth (expected YYYY-MM-DD)", i)
		}
		offeringDays := make([]enrollmentService.SubmitOfferingDays, 0, len(c.OfferingDays))
		for _, row := range c.OfferingDays {
			offeringDays = append(offeringDays, enrollmentService.SubmitOfferingDays{
				OfferingID:   row.OfferingID,
				SelectedDays: row.SelectedDays,
			})
		}
		out.Children = append(out.Children, enrollmentService.SubmitChild{
			ID:                int64PtrValue(c.ID),
			FirstName:         c.FirstName,
			LastName:          c.LastName,
			DateOfBirth:       dob,
			TargetGradeLevel:  c.TargetGradeLevel,
			TargetSchoolClass: c.TargetSchoolClass,
			CustomData:        c.CustomData,
			OfferingIDs:       c.OfferingIDs,
			OfferingDays:      offeringDays,
		})
	}
	return out, nil
}

func int64PtrValue(v *int64) int64 {
	if v == nil {
		return 0
	}
	return *v
}

// Stable error codes returned in the JSON envelope so the frontend can
// map to localized German messages without parsing free-form text. Keep
// in sync with ENROLLMENT_CODE_MESSAGES in
// frontend/src/lib/enrollment-error-messages.ts.
const (
	ErrCodeEnrollmentCareOfferingMissing         = "enrollment.care_offering_missing"
	ErrCodeEnrollmentApprovalCareOfferingMissing = "enrollment.approval_care_offering_missing"
	ErrCodeEnrollmentCareOfferingExactlyOne      = "enrollment.care_offering_exactly_one"
	ErrCodeEnrollmentRequiredCareOfferingMissing = "enrollment.required_care_offering_missing"
	ErrCodeEnrollmentCareOfferingFull            = "enrollment.care_offering_full"
	ErrCodeEnrollmentCareOfferingsDisabled       = "enrollment.care_offerings_disabled"
	ErrCodeEnrollmentCareOfferingUnavailable     = "enrollment.care_offering_unavailable"
	ErrCodeEnrollmentInvalidPhone                = "enrollment.invalid_phone"
	ErrCodeEnrollmentInvalidEmail                = "enrollment.invalid_email"
	ErrCodeEnrollmentPickupTimeNotAllowed        = "enrollment.pickup_time_not_allowed"
	ErrCodeEnrollmentDepartureModeLimit          = "enrollment.departure_mode_limit"
	ErrCodeEnrollmentLateInviteInvalid           = "enrollment.late_invite_invalid"
	ErrCodeEnrollmentSelectedDayNotAvailable     = "enrollment.selected_day_not_available"
	ErrCodeEnrollmentDaySelectionRequired        = "enrollment.day_selection_required"
	ErrCodeEnrollmentDaySelectionNotAllowed      = "enrollment.day_selection_not_allowed"
	// Phase eligibility codes (#1663).
	ErrCodeEnrollmentPhaseNotEligible     = "enrollment.phase_not_eligible"
	ErrCodeEnrollmentClassNotEligible     = "enrollment.class_not_eligible"
	ErrCodeEnrollmentGradeNotEligible     = "enrollment.grade_not_eligible"
	ErrCodeEnrollmentChildAlreadyEnrolled = "enrollment.child_already_enrolled"
	ErrCodeEnrollmentChildNotEnrolled     = "enrollment.child_not_enrolled"
	ErrCodeEnrollmentChildAmbiguous       = "enrollment.child_ambiguous"
	ErrCodeEnrollmentChildNotPermitted    = "enrollment.child_not_permitted"
)

// MapSubmitError translates service-layer sentinel errors into HTTP
// status codes. Unknown errors fall through to 500.
func MapSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrLateInviteInvalid):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, ErrCodeEnrollmentLateInviteInvalid))
	case errors.Is(err, enrollmentService.ErrPhaseNotEligible):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, ErrCodeEnrollmentPhaseNotEligible))
	// The two child-level eligibility errors wrap ErrInvalidSubmission,
	// so their specific matches must precede the generic case below.
	case errors.Is(err, enrollmentService.ErrChildClassNotEligible):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentClassNotEligible))
	case errors.Is(err, enrollmentService.ErrChildGradeNotEligible):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentGradeNotEligible))
	case errors.Is(err, enrollmentService.ErrChildAlreadyEnrolled):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentChildAlreadyEnrolled))
	case errors.Is(err, enrollmentService.ErrChildNotEnrolled):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentChildNotEnrolled))
	case errors.Is(err, enrollmentService.ErrChildEnrollmentAmbiguous):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentChildAmbiguous))
	// Per-child re-enrollment authorization failure (#1663): a guardian account
	// lacking parent_portal.enrollment.submit on the matched student. It does NOT
	// wrap ErrInvalidSubmission — it is a 403, not a 400.
	case errors.Is(err, enrollmentService.ErrChildEnrollmentNotPermitted):
		common.RenderError(w, r, common.ErrorForbiddenWithCode(err, ErrCodeEnrollmentChildNotPermitted))
	case errors.Is(err, enrollmentService.ErrCareOfferingUnavailable):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentCareOfferingUnavailable))
	case errors.Is(err, enrollmentService.ErrCareOfferingMissing):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentCareOfferingMissing))
	case errors.Is(err, enrollmentService.ErrCareOfferingExactlyOneRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentCareOfferingExactlyOne))
	case errors.Is(err, enrollmentService.ErrRequiredCareOfferingMissing):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentRequiredCareOfferingMissing))
	case errors.Is(err, enrollmentService.ErrCareOfferingsDisabled):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentCareOfferingsDisabled))
	case errors.Is(err, enrollmentService.ErrInvalidGuardianPhone):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentInvalidPhone))
	// Must precede the generic ErrInvalidSubmission case below: the email
	// error wraps ErrInvalidSubmission, so the specific match has to win.
	case errors.Is(err, enrollmentService.ErrInvalidGuardianEmail):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentInvalidEmail))
	// Must precede the generic ErrInvalidSubmission case below: the pickup
	// error wraps ErrInvalidSubmission, so the specific match has to win.
	case errors.Is(err, enrollmentService.ErrPickupTimeNotAllowed):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentPickupTimeNotAllowed))
	// Heimweg-Beschränkung (#2381) also wraps ErrInvalidSubmission.
	case errors.Is(err, enrollmentService.ErrDepartureModeLimitExceeded):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentDepartureModeLimit))
	// The three offering-day errors (#1885) also wrap ErrInvalidSubmission,
	// so their specific matches must precede the generic case below.
	case errors.Is(err, enrollmentService.ErrSelectedDayNotAvailable):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentSelectedDayNotAvailable))
	case errors.Is(err, enrollmentService.ErrDaySelectionRequired):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentDaySelectionRequired))
	case errors.Is(err, enrollmentService.ErrDaySelectionNotAllowed):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentDaySelectionNotAllowed))
	case errors.Is(err, enrollmentService.ErrCareOfferingClosed),
		errors.Is(err, enrollmentService.ErrInvalidSubmission):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingFull):
		// 409 Conflict: the request is well-formed but a selected
		// offering is at capacity and the tenant's overflow mode is
		// 'reject'. Return a JSON envelope with a stable code so the
		// frontend can render a friendly German message; the previous
		// http.Error() emitted plain text and the form fell back to
		// "(HTTP 409)".
		common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeEnrollmentCareOfferingFull))
	case errors.Is(err, enrollmentService.ErrDuplicateEnrollment):
		// 409 Conflict: the same guardian email already has an active
		// (non-rejected, non-withdrawn) enrollment for one of these
		// children in this phase. JSON envelope so the frontend's
		// readError helper surfaces the German message instead of
		// falling back to "(HTTP 409)".
		common.RenderError(w, r, common.ErrorConflictMessage("Für dieses Kind liegt in dieser Phase bereits eine Anmeldung vor."))
	case errors.Is(err, enrollmentService.ErrExistingStudentAlreadyRequested):
		// 409 Conflict: another active request in this phase already targets the
		// same already-enrolled student this child matched (a different guardian
		// email, so the email-scoped duplicate check missed it). Distinct German
		// message so parents understand the child is already being re-enrolled.
		common.RenderError(w, r, common.ErrorConflictMessage("Für dieses Kind liegt in dieser Phase bereits eine Anmeldung von einer anderen Person vor."))
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

func lateInviteTokenFromRequest(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("late_invite"))
}

// remoteIPFromRequest returns the router-selected client IP for captcha
// verification and submission rate limiting.
func remoteIPFromRequest(r *http.Request) string {
	return clientip.GetClientIPString(r)
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
	EditMode          string                `json:"edit_mode"`
	Children          []StatusChildResponse `json:"children"`
	// AdditionalGuardians are the co-guardians the parent added beyond the
	// primary guardian above. Empty when none were added.
	AdditionalGuardians []StatusGuardianResponse `json:"additional_guardians,omitempty"`
}

// StatusGuardianResponse is one additional guardian on the public status
// page. Email/phone are optional.
type StatusGuardianResponse struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// StatusChildResponse is one row in StatusResponse.Children.
type StatusChildResponse struct {
	ID           string  `json:"id"`
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Status       string  `json:"status"`
	StatusReason *string `json:"status_reason,omitempty"`
	// Locked marks a child that is already taken over into care. Its data
	// stays readable through the status link, but changes to it run through
	// the parent app only (ADR 0003).
	Locked bool `json:"locked"`
}

type EditBootstrapResponse struct {
	Phase                     PublicPhase               `json:"phase"`
	Schema                    *PublicFormSchemaResponse `json:"schema"`
	Offerings                 []CareOfferingResponse    `json:"offerings"`
	CareOfferingSelectionMode string                    `json:"care_offering_selection_mode"`
	CareRequired              bool                      `json:"care_required"`
	SchoolClass               PublicSchoolClassConfig   `json:"school_class"`
	CollectGradeLevel         bool                      `json:"collect_grade_level"`
	CareOfferingsEnabled      bool                      `json:"care_offerings_enabled"`
	GradeLevelMax             int                       `json:"grade_level_max"`
	LegalTexts                PublicLegalTextsResponse  `json:"legal_texts"`
	Draft                     EditDraftResponse         `json:"draft"`
	EditMode                  string                    `json:"edit_mode"`
}

type EditDraftResponse struct {
	RequestID           string                      `json:"request_id"`
	StatusToken         string                      `json:"status_token"`
	TenantID            string                      `json:"tenant_id"`
	TenantSubdomain     string                      `json:"tenant_subdomain"`
	PhaseID             string                      `json:"phase_id"`
	GuardianFirstName   string                      `json:"guardian_first_name"`
	GuardianLastName    string                      `json:"guardian_last_name"`
	GuardianEmail       string                      `json:"guardian_email"`
	GuardianPhone       *string                     `json:"guardian_phone,omitempty"`
	ConsentFlags        map[string]any              `json:"consent_flags"`
	CustomData          map[string]any              `json:"custom_data"`
	AdditionalGuardians []EditDraftGuardianResponse `json:"additional_guardians,omitempty"`
	Children            []EditDraftChildResponse    `json:"children"`
}

type EditDraftGuardianResponse struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

type EditDraftChildResponse struct {
	ID                string                         `json:"id"`
	FirstName         string                         `json:"first_name"`
	LastName          string                         `json:"last_name"`
	DateOfBirth       string                         `json:"date_of_birth"`
	TargetGradeLevel  *int16                         `json:"target_grade_level,omitempty"`
	TargetSchoolClass *string                        `json:"target_school_class,omitempty"`
	CustomData        map[string]any                 `json:"custom_data"`
	OfferingIDs       []string                       `json:"offering_ids"`
	OfferingDays      []EditDraftOfferingDayResponse `json:"offering_days,omitempty"`
	// Locked marks a child that is already taken over into care: the change
	// form shows it read-only and points to the parent app (ADR 0003).
	Locked bool `json:"locked"`
}

type EditDraftOfferingDayResponse struct {
	OfferingID            string   `json:"offering_id"`
	SelectedDays          []string `json:"selected_days"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
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
		req       *enrollmentModels.Request
		children  []*enrollmentModels.RequestChild
		guardians []*enrollmentModels.RequestGuardian
		editMode  string
		statusErr error
	)
	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		serviceReq, serviceChildren, err := rs.RequestService.GetByStatusToken(adminCtx, token)
		if err != nil {
			return err
		}
		req = serviceReq
		children = serviceChildren
		mode, modeErr := rs.RequestService.EditModeForStatus(adminCtx, req, children)
		if modeErr != nil {
			statusErr = fmt.Errorf("status: compute edit mode: %w", modeErr)
			return statusErr
		}
		editMode = mode
		// Best-effort: a failure here must not hide the request status, but a
		// silent drop would mask a permission/RLS/DB problem behind a 200 with
		// missing co-guardians. Log it so the failure is visible.
		if g, gerr := rs.RequestService.GuardiansByStatusToken(adminCtx, token); gerr == nil {
			guardians = g
		} else {
			rs.logger().Warn("enrollment status: load co-guardians failed",
				slog.String("error", gerr.Error()))
		}
		return nil
	})
	if err != nil {
		if statusErr != nil {
			common.RenderError(w, r, common.ErrorInternalServer(statusErr))
			return
		}
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
		EditMode:          editMode,
	}
	for _, c := range children {
		resp.Children = append(resp.Children, StatusChildResponse{
			ID:           strconv.FormatInt(c.ID, 10),
			FirstName:    c.FirstName,
			LastName:     c.LastName,
			Status:       c.Status,
			StatusReason: c.StatusReason,
			Locked:       enrollmentService.ChildTakenOver(c),
		})
	}
	for _, g := range guardians {
		resp.AdditionalGuardians = append(resp.AdditionalGuardians, StatusGuardianResponse{
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Email:     g.Email,
			Phone:     g.Phone,
		})
	}
	common.Respond(w, r, http.StatusOK, resp, "Status retrieved")
}

func (rs *Resource) getEditBootstrap(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}

	draft, err := rs.RequestService.GetEditDraft(r.Context(), token)
	if err != nil {
		mapEditError(w, r, err)
		return
	}

	offerings := make([]CareOfferingResponse, 0, len(draft.OpenOfferings))
	for _, o := range draft.OpenOfferings {
		offerings = append(offerings, toCareOfferingResponse(o))
	}
	common.Respond(w, r, http.StatusOK, EditBootstrapResponse{
		Phase:                     toPublicPhase(draft.Phase),
		Schema:                    toPublicFormSchemaResponse(draft.Schema),
		Offerings:                 offerings,
		CareOfferingSelectionMode: effectiveCareOfferingSelectionMode(draft.Phase.CareOfferingSelectionMode, draft.CareOfferingsEnabled),
		CareRequired:              draft.CareOfferingsEnabled && draft.Phase.CareOfferingSelectionMode != enrollmentModels.PhaseCareOfferingSelectionOptional,
		SchoolClass:               toPublicSchoolClassConfig(draft.Phase, draft.CollectSchoolClass),
		CollectGradeLevel:         draft.CollectGradeLevel,
		CareOfferingsEnabled:      draft.CareOfferingsEnabled,
		GradeLevelMax:             draft.GradeLevelMax,
		LegalTexts: PublicLegalTextsResponse{
			AGB:                 draft.LegalTexts.AGB,
			DSGVO:               draft.LegalTexts.DSGVO,
			EmailContact:        draft.LegalTexts.EmailContact,
			Photo:               draft.LegalTexts.Photo,
			TermsEnabled:        draft.LegalTexts.TermsEnabled,
			DSGVOEnabled:        draft.LegalTexts.DSGVOEnabled,
			EmailContactEnabled: draft.LegalTexts.EmailContactEnabled,
			PhotoEnabled:        draft.LegalTexts.PhotoEnabled,
			Blocks:              draft.LegalTexts.Blocks,
		},
		Draft:    toEditDraftResponse(draft),
		EditMode: draft.EditMode,
	}, "Enrollment edit bootstrap retrieved")
}

func toEditDraftResponse(draft *enrollmentService.EditDraft) EditDraftResponse {
	resp := EditDraftResponse{
		RequestID:           strconv.FormatInt(draft.Request.ID, 10),
		StatusToken:         draft.Request.StatusToken,
		TenantID:            strconv.FormatInt(draft.Request.GetTenantID(), 10),
		PhaseID:             strconv.FormatInt(draft.Request.PhaseID, 10),
		GuardianFirstName:   draft.Request.GuardianFirstName,
		GuardianLastName:    draft.Request.GuardianLastName,
		GuardianEmail:       draft.Request.GuardianEmail,
		GuardianPhone:       draft.Request.GuardianPhone,
		ConsentFlags:        draft.Request.ConsentFlags,
		CustomData:          draft.Request.CustomData,
		AdditionalGuardians: toEditDraftGuardianResponses(draft.Guardians),
		Children:            toEditDraftChildResponses(draft),
	}
	if draft.School != nil {
		// Tenant resolution (TenantProvider → /auth/tenant/resolve) works by
		// subdomain, not slug (#1977).
		resp.TenantSubdomain = draft.School.Subdomain
	}
	return resp
}

func toEditDraftGuardianResponses(guardians []*enrollmentModels.RequestGuardian) []EditDraftGuardianResponse {
	var responses []EditDraftGuardianResponse
	for _, guardian := range guardians {
		responses = append(responses, EditDraftGuardianResponse{
			FirstName: guardian.FirstName,
			LastName:  guardian.LastName,
			Email:     guardian.Email,
			Phone:     guardian.Phone,
		})
	}
	return responses
}

func toEditDraftChildResponses(draft *enrollmentService.EditDraft) []EditDraftChildResponse {
	responses := make([]EditDraftChildResponse, 0, len(draft.Children))
	for _, child := range draft.Children {
		offeringLinks := draft.OfferingsByChild[child.ID]
		if !draft.CareOfferingsEnabled && !enrollmentService.ChildTakenOver(child) {
			offeringLinks = nil
		}
		responses = append(responses, toEditDraftChildResponse(child, offeringLinks))
	}
	return responses
}

func toEditDraftChildResponse(child *enrollmentModels.RequestChild, offeringLinks []*enrollmentModels.RequestChildOffering) EditDraftChildResponse {
	response := EditDraftChildResponse{
		ID:                strconv.FormatInt(child.ID, 10),
		FirstName:         child.FirstName,
		LastName:          child.LastName,
		DateOfBirth:       child.DateOfBirth.String(),
		TargetGradeLevel:  child.TargetGradeLevel,
		TargetSchoolClass: child.TargetSchoolClass,
		CustomData:        child.CustomData,
		OfferingIDs:       []string{},
		Locked:            enrollmentService.ChildTakenOver(child),
	}
	for _, link := range offeringLinks {
		response.OfferingIDs = append(response.OfferingIDs, strconv.FormatInt(link.CareOfferingID, 10))
		if offeringDay := toEditDraftOfferingDayResponse(link); offeringDay != nil {
			response.OfferingDays = append(response.OfferingDays, *offeringDay)
		}
	}
	return response
}

func toEditDraftOfferingDayResponse(link *enrollmentModels.RequestChildOffering) *EditDraftOfferingDayResponse {
	if len(link.SelectedDays) == 0 {
		return nil
	}
	manualDays := link.ManualSelectedDays
	if len(manualDays) == 0 && len(link.AutomaticSelectedDays) == 0 {
		manualDays = link.SelectedDays
	}
	return &EditDraftOfferingDayResponse{
		OfferingID:            strconv.FormatInt(link.CareOfferingID, 10),
		SelectedDays:          link.SelectedDays,
		ManualSelectedDays:    manualDays,
		AutomaticSelectedDays: link.AutomaticSelectedDays,
	}
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
		case errors.Is(err, enrollmentService.ErrInvalidGuardianPhone):
			common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeEnrollmentInvalidPhone))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"message": "updated"}, "Request updated")
}

func (rs *Resource) replaceStatus(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}
	wireReq := &SubmitEnrollmentRequest{}
	if err := render.Bind(r, wireReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	serviceReq, err := BuildServiceRequest(wireReq, 0, "")
	if err != nil {
		MapSubmitError(w, r, err)
		return
	}
	result, err := rs.RequestService.ReplaceEditable(r.Context(), token, serviceReq)
	if err != nil {
		mapEditError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
		Warnings:  result.Warnings,
	}, "Request updated")
}

func mapEditError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrRequestNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, enrollmentService.ErrEditNotAllowed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	default:
		MapSubmitError(w, r, err)
	}
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

// ConfirmRenewalResponse reports how many child rows were transitioned
// from pending_renewal to submitted.
type ConfirmRenewalResponse struct {
	Confirmed int `json:"confirmed"`
}

// confirmRenewal handles POST /requests/{statusToken}/confirm-renewal.
// Public — gated only by status token possession, the same way the
// other parent-facing /requests endpoints are.
func (rs *Resource) confirmRenewal(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("enrollment request service not configured")))
		return
	}
	token := strings.TrimSpace(chi.URLParam(r, "statusToken"))
	if token == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("status token is required")))
		return
	}

	var confirmed int
	err := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		c, runErr := rs.RequestService.ConfirmRenewal(adminCtx, token)
		confirmed = c
		return runErr
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrRequestNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}
	common.Respond(w, r, http.StatusOK, ConfirmRenewalResponse{Confirmed: confirmed}, "Renewal confirmed")
}
