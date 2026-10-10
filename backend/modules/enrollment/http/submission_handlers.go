package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
	RequestID string              `json:"request_id"`
	StatusURL string              `json:"status_url"`
	Warnings  []SubmissionWarning `json:"warnings,omitempty"`
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

	result, submitErr, resolveErr := rs.submitPublicEnrollment(r.Context(), slug, wireReq, remoteIPFromRequest(r))
	// submitErr wins over resolveErr: when the closure failed, resolveErr
	// carries the same error and the submit-specific mapping applies.
	if submitErr != nil {
		mapSubmitError(w, r, submitErr)
		return
	}
	if resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFoundWithCode(resolveErr, common.CodeEnrollmentFormNotFound))
		return
	}

	resp := SubmitEnrollmentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
		Warnings:  result.Warnings,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}

// submitPublicEnrollment resolves the tenant and runs the submit inside the
// tenant's tx. RLS on enrollment.* tables narrows writes to the resolved
// tenant.
//
// Every failure inside the closure MUST be returned from it: the
// service's inner TxHandler.RunInTx reuses this outer transaction and
// cannot roll back by itself, so swallowing the error here would
// commit partial writes while the client receives an error response.
// submitErr remembers which failures belong to the submit flow so the
// post-tx mapping can distinguish them from tenant-resolve failures.
func (rs *Resource) submitPublicEnrollment(ctx context.Context, slug string, wireReq *SubmitEnrollmentRequest, remoteIP string) (result *SubmitResult, submitErr, resolveErr error) {
	schoolID, resolveErr := rs.resolvePublicTenantID(ctx, slug)
	if resolveErr != nil {
		return nil, nil, resolveErr
	}
	resolveErr = withinTenant(ctx, schoolID, func(tenantCtx context.Context) error {
		// Captcha gate runs before the DB write.
		if err := rs.CaptchaService.Verify(tenantCtx, wireReq.CaptchaToken, remoteIP); err != nil {
			submitErr = fmt.Errorf("captcha: %w", err)
			return submitErr
		}

		serviceReq, parseErr := buildServiceRequest(wireReq, schoolID, remoteIP)
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
	return result, submitErr, resolveErr
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

// buildServiceRequest converts the wire request into the service-layer
// shape. Parses date strings; surfaces a typed error on bad input.
func buildServiceRequest(wireReq *SubmitEnrollmentRequest, tenantID int64, remoteIP string) (SubmitRequest, error) {
	out := SubmitRequest{
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
		out.AdditionalGuardians = append(out.AdditionalGuardians, SubmitGuardian{
			FirstName: g.FirstName,
			LastName:  g.LastName,
			Email:     g.Email,
			Phone:     g.Phone,
		})
	}
	for i, c := range wireReq.Children {
		dob, err := calendar.ParseDate(c.DateOfBirth)
		if err != nil {
			return out, capability.InvalidInput(common.CodeEnrollmentChildBirthDateInvalid, fmt.Sprintf("children.%d.date_of_birth", i),
				fmt.Errorf("child %d: invalid date_of_birth (expected YYYY-MM-DD)", i))
		}
		offeringDays := make([]SubmitOfferingDays, 0, len(c.OfferingDays))
		for _, row := range c.OfferingDays {
			offeringDays = append(offeringDays, SubmitOfferingDays{
				OfferingID:   row.OfferingID,
				SelectedDays: row.SelectedDays,
			})
		}
		out.Children = append(out.Children, SubmitChild{
			ID:                deref(c.ID),
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

func lateInviteTokenFromRequest(r *http.Request) string {
	return strings.TrimSpace(r.URL.Query().Get("late_invite"))
}

// remoteIPFromRequest returns the router-selected client IP for captcha
// verification and submission rate limiting.
func remoteIPFromRequest(r *http.Request) string {
	return common.GetClientIPString(r)
}
