package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// EnrollmentProfileResponse mirrors the public form's autofill shape
// (api/enrollment/me_handlers.go MeProfileResponse) so the same
// EnrollmentForm component can consume it without branching. Children
// are scoped to the tenant resolved from the {tenantSlug} path.
type EnrollmentProfileResponse struct {
	Guardian profileGuardian `json:"guardian"`
	Children []profileChild  `json:"children"`
}

type profileGuardian struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     string  `json:"email"`
	Phone     *string `json:"phone,omitempty"`
}

type profileChild struct {
	ID          string `json:"id"`
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	SchoolClass string `json:"school_class"`
	GradeLevel  *int   `json:"grade_level,omitempty"`
}

// getEnrollmentProfile returns guardian + linked-children data for the
// calling parent within the requested tenant. Tenant comes from the
// path slug (admin-tx resolves it); account from claims.ID. The
// repository call runs under WithTenantTx via the loader so RLS
// narrows reads to that tenant — a parent who has no profile in this
// school just gets claims-derived guardian fields and an empty
// children list.
func (rs *Resource) getEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	if rs.GuardianProfileLoader == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent enrollment profile not wired")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	accountID := int64(claims.ID)

	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	schoolID, err := rs.resolveSchoolID(r.Context(), slug)
	if err != nil {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}

	loaded, err := rs.GuardianProfileLoader.LoadForTenant(r.Context(), accountID, schoolID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	resp := buildEnrollmentProfileResponse(claims, loaded)
	common.Respond(w, r, http.StatusOK, resp, "Profile retrieved")
}

// resolveSchoolID wraps SchoolRepo.FindBySlug in WithAdminTx so the
// cross-tenant lookup is allowed. Returns an error when the slug
// doesn't resolve or the school is soft-deleted.
func (rs *Resource) resolveSchoolID(ctx context.Context, slug string) (int64, error) {
	var out int64
	err := tenant.WithAdminTx(ctx, rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, findErr := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if findErr != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}
		out = school.ID
		return nil
	})
	if err != nil {
		return 0, err
	}
	return out, nil
}

// buildEnrollmentProfileResponse merges the claims-derived defaults
// with the (possibly nil) guardian profile loaded from the tenant.
// Nil loaded → defaults only + empty children, matching the legacy
// fall-through behavior.
func buildEnrollmentProfileResponse(claims jwt.AppClaims, loaded *usersModels.GuardianProfileWithChildren) EnrollmentProfileResponse {
	resp := EnrollmentProfileResponse{
		Guardian: profileGuardian{
			FirstName: claims.FirstName,
			LastName:  claims.LastName,
			Email:     claims.Sub,
		},
		Children: []profileChild{},
	}
	if loaded == nil || loaded.Profile == nil {
		return resp
	}
	if loaded.Profile.FirstName != "" {
		resp.Guardian.FirstName = loaded.Profile.FirstName
	}
	if loaded.Profile.LastName != "" {
		resp.Guardian.LastName = loaded.Profile.LastName
	}
	if loaded.Profile.Email != nil && *loaded.Profile.Email != "" {
		resp.Guardian.Email = *loaded.Profile.Email
	}
	if loaded.PrimaryPhone != "" {
		phone := loaded.PrimaryPhone
		resp.Guardian.Phone = &phone
	}
	for _, child := range loaded.Children {
		entry := profileChild{
			ID:          strconv.FormatInt(child.StudentID, 10),
			FirstName:   child.FirstName,
			LastName:    child.LastName,
			SchoolClass: child.SchoolClass,
		}
		if grade := parseLeadingGrade(child.SchoolClass); grade > 0 {
			entry.GradeLevel = &grade
		}
		resp.Children = append(resp.Children, entry)
	}
	return resp
}

// parseLeadingGrade extracts the leading integer from strings like "1a"
// or "10". Mirrors api/enrollment/me_handlers.go parseGradeLevel.
func parseLeadingGrade(schoolClass string) int {
	digits := ""
	for _, r := range schoolClass {
		if r >= '0' && r <= '9' {
			digits += string(r)
			continue
		}
		break
	}
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}

// submitParentChildRequest is the wire shape for one child within the
// parent submit body. Mirrors api/enrollment.SubmitChildRequest — kept
// duplicate (vs. importing the public type) so the parent route stays
// independent of the public API package.
type submitParentChildRequest struct {
	FirstName        string                          `json:"first_name"`
	LastName         string                          `json:"last_name"`
	DateOfBirth      string                          `json:"date_of_birth"`
	TargetGradeLevel *int16                          `json:"target_grade_level,omitempty"`
	CustomData       map[string]any                  `json:"custom_data,omitempty"`
	OfferingIDs      []int64                         `json:"offering_ids,omitempty"`
	OfferingDays     []submitParentOfferingDaysEntry `json:"offering_days,omitempty"`
}

// submitParentOfferingDaysEntry mirrors api/enrollment.SubmitOfferingDaysRow.
type submitParentOfferingDaysEntry struct {
	OfferingID   int64    `json:"offering_id"`
	SelectedDays []string `json:"selected_days"`
}

// submitParentGuardianEntry mirrors api/enrollment.SubmitGuardianRequest —
// one additional guardian (co-guardian) beyond the primary. Names
// required; email/phone optional.
type submitParentGuardianEntry struct {
	FirstName string  `json:"first_name"`
	LastName  string  `json:"last_name"`
	Email     *string `json:"email,omitempty"`
	Phone     *string `json:"phone,omitempty"`
}

// submitParentEnrollmentRequest is the parent-authenticated submit
// body. Captcha is intentionally absent — the parent JWT IS the
// anti-bot signal; we skip captcha verification end-to-end.
type submitParentEnrollmentRequest struct {
	PhaseID             int64                       `json:"phase_id"`
	GuardianFirstName   string                      `json:"guardian_first_name"`
	GuardianLastName    string                      `json:"guardian_last_name"`
	GuardianEmail       string                      `json:"guardian_email"`
	GuardianPhone       *string                     `json:"guardian_phone,omitempty"`
	AdditionalGuardians []submitParentGuardianEntry `json:"additional_guardians,omitempty"`
	ConsentFlags        map[string]any              `json:"consent_flags,omitempty"`
	CustomData          map[string]any              `json:"custom_data,omitempty"`
	Children            []submitParentChildRequest  `json:"children"`
}

// submitParentResponse is what the embedded form receives after a
// successful authenticated submit. Same shape as the public path —
// intentional, so the form can share its onSubmitted handler.
type submitParentResponse struct {
	RequestID string `json:"request_id"`
	StatusURL string `json:"status_url"`
}

// submitParentEnrollment handles a parent-authenticated submission.
// The handler resolves the slug to a tenant via admin-tx, verifies the
// calling account is mapped to that tenant via auth.account_tenants
// (defense against an authenticated parent stamping rows on schools
// they have no relationship with), then runs the existing
// RequestService.Submit with GuardianAccountID stamped from claims.ID
// and the originating IP captured for rate-limiting. Captcha is
// skipped — the JWT is the trust signal.
func (rs *Resource) submitParentEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit not configured")))
		return
	}
	if rs.AuthService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit: account tenant repo missing")))
		return
	}
	if rs.ParentService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit: parent service missing")))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	if claims.ID == 0 {
		common.RenderError(w, r, common.ErrorUnauthorized(jwt.ErrTokenUnauthorized))
		return
	}
	accountID := int64(claims.ID)

	slug := strings.TrimSpace(chi.URLParam(r, "tenantSlug"))
	if slug == "" {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("tenant slug is required")))
		return
	}

	wireReq := &submitParentEnrollmentRequest{}
	if err := json.NewDecoder(r.Body).Decode(wireReq); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if wireReq.ConsentFlags == nil {
		wireReq.ConsentFlags = map[string]any{}
	}
	if wireReq.CustomData == nil {
		wireReq.CustomData = map[string]any{}
	}
	if wireReq.Children == nil {
		wireReq.Children = []submitParentChildRequest{}
	}

	var (
		result    *enrollmentService.SubmitResult
		submitErr error
		forbidden bool
	)
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolService.GetSchoolBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Tenant-mapping check: the parent JWT identifies the actor but
		// does NOT prove they belong at this school. Without this
		// guard, any authenticated parent could stamp
		// guardian_account_id on requests for arbitrary tenants.
		mapped, mapErr := rs.AuthService.VerifyAccountTenantMembership(adminCtx, accountID, school.ID)
		if mapErr != nil {
			return fmt.Errorf("verify tenant membership: %w", mapErr)
		}
		if !mapped {
			forbidden = true
			return nil
		}
		canSubmit, permErr := rs.ParentService.CanSubmitEnrollmentForTenant(r.Context(), accountID, school.ID)
		if permErr != nil {
			return fmt.Errorf("verify enrollment submit permission: %w", permErr)
		}
		if !canSubmit {
			forbidden = true
			return nil
		}

		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)

		serviceReq, parseErr := buildParentServiceRequest(wireReq, school.ID, accountID, getClientIP(r))
		if parseErr != nil {
			submitErr = parseErr
			return nil
		}
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
	if forbidden {
		common.RenderError(w, r, common.ErrorForbidden(errors.New("account is not a member of this school")))
		return
	}
	if submitErr != nil {
		mapParentSubmitError(w, r, submitErr)
		return
	}

	resp := submitParentResponse{
		RequestID: strconv.FormatInt(result.Request.ID, 10),
		StatusURL: result.StatusURL,
	}
	common.Respond(w, r, http.StatusCreated, resp, "Enrollment submitted")
}

// buildParentServiceRequest converts the wire request to the
// enrollment.RequestService input. Mirrors api/enrollment.buildServiceRequest,
// stamps GuardianAccountID from the JWT, and now also forwards the
// caller's IP so per-IP rate limiting applies on top of per-email
// and per-account checks.
func buildParentServiceRequest(wireReq *submitParentEnrollmentRequest, tenantID, accountID int64, remoteIP string) (enrollmentService.SubmitRequest, error) {
	out := enrollmentService.SubmitRequest{
		TenantID:          tenantID,
		PhaseID:           wireReq.PhaseID,
		GuardianFirstName: wireReq.GuardianFirstName,
		GuardianLastName:  wireReq.GuardianLastName,
		GuardianEmail:     wireReq.GuardianEmail,
		GuardianPhone:     wireReq.GuardianPhone,
		ConsentFlags:      wireReq.ConsentFlags,
		CustomData:        wireReq.CustomData,
		GuardianAccountID: &accountID,
		RemoteIP:          remoteIP,
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
			FirstName:        c.FirstName,
			LastName:         c.LastName,
			DateOfBirth:      dob,
			TargetGradeLevel: c.TargetGradeLevel,
			CustomData:       c.CustomData,
			OfferingIDs:      c.OfferingIDs,
			OfferingDays:     offeringDays,
		})
	}
	return out, nil
}

// mapParentSubmitError mirrors mapSubmitError in api/enrollment, minus
// the captcha branch (parents never trigger captcha-shaped errors).
func mapParentSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	// Phone/email validation must precede the generic ErrInvalidSubmission
	// branch below: ErrInvalidGuardianEmail wraps ErrInvalidSubmission, and
	// ErrInvalidGuardianPhone does NOT wrap it at all — without these cases an
	// invalid optional co-guardian phone would fall through to 500. Codes
	// mirror enrollment.ErrCodeEnrollmentInvalidPhone / InvalidEmail so the
	// shared EnrollmentForm renders the same German message as the public form.
	case errors.Is(err, enrollmentService.ErrInvalidGuardianPhone):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "enrollment.invalid_phone"))
	case errors.Is(err, enrollmentService.ErrInvalidGuardianEmail):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, "enrollment.invalid_email"))
	case errors.Is(err, enrollmentService.ErrCareOfferingClosed),
		errors.Is(err, enrollmentService.ErrInvalidSubmission):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingFull):
		// Mirror the public submit handler: JSON envelope + stable code
		// so the parent portal form renders the friendly German message
		// instead of "(HTTP 409)". Code matches
		// enrollment.ErrCodeEnrollmentCareOfferingFull.
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "enrollment.care_offering_full"))
	case errors.Is(err, enrollmentService.ErrDuplicateEnrollment):
		common.RenderError(w, r, common.ErrorConflictMessage("Für dieses Kind liegt in dieser Phase bereits eine Anmeldung vor."))
	case errors.Is(err, enrollmentService.ErrRateLimited):
		w.Header().Set("Retry-After", "3600")
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
