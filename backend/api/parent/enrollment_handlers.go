package parent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
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
// guardian_profile lookup runs under WithTenantTx so RLS narrows
// reads to that tenant — a parent who has no profile in this school
// just gets claims-derived guardian fields and an empty children list.
func (rs *Resource) getEnrollmentProfile(w http.ResponseWriter, r *http.Request) {
	if rs.db == nil {
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

	resp := EnrollmentProfileResponse{
		Guardian: profileGuardian{
			FirstName: claims.FirstName,
			LastName:  claims.LastName,
			Email:     claims.Sub,
		},
		Children: []profileChild{},
	}

	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		// Stamp the resolved school onto the context BEFORE the nested
		// WithTenantTx so its mismatch guard sees the same id (instead
		// of the parent JWT's tenant_id=0, which would error out with
		// "mismatched tenant_id"). Mirrors the submitParentEnrollment
		// path below.
		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)

		return tenant.WithTenantTx(tenantCtx, rs.db, school.ID, func(txCtx context.Context, tx bun.Tx) error {
			var profile usersModels.GuardianProfile
			schoolErr := tx.NewSelect().
				Model(&profile).
				ModelTableExpr(`users.guardian_profiles AS "guardian_profile"`).
				Where(`"guardian_profile".account_id = ?`, accountID).
				Limit(1).
				Scan(txCtx)
			if schoolErr != nil {
				return nil //nolint:nilerr // no profile in this tenant — fall through to claims-derived response
			}

			if profile.FirstName != "" {
				resp.Guardian.FirstName = profile.FirstName
			}
			if profile.LastName != "" {
				resp.Guardian.LastName = profile.LastName
			}
			if profile.Email != nil && *profile.Email != "" {
				resp.Guardian.Email = *profile.Email
			}

			var primaryPhone usersModels.GuardianPhoneNumber
			if err := tx.NewSelect().
				Model(&primaryPhone).
				ModelTableExpr(`users.guardian_phone_numbers AS "guardian_phone_number"`).
				Where(`"guardian_phone_number".guardian_profile_id = ?`, profile.ID).
				OrderExpr(`"guardian_phone_number".is_primary DESC, "guardian_phone_number".id ASC`).
				Limit(1).
				Scan(txCtx); err == nil && primaryPhone.PhoneNumber != "" {
				phone := primaryPhone.PhoneNumber
				resp.Guardian.Phone = &phone
			}

			type studentRow struct {
				StudentID   int64  `bun:"student_id"`
				FirstName   string `bun:"first_name"`
				LastName    string `bun:"last_name"`
				SchoolClass string `bun:"school_class"`
			}
			var rows []studentRow
			if err := tx.NewSelect().
				TableExpr(`users.students_guardians AS "sg"`).
				ColumnExpr(`"s".id AS student_id`).
				ColumnExpr(`"p".first_name`).
				ColumnExpr(`"p".last_name`).
				ColumnExpr(`"s".school_class`).
				Join(`INNER JOIN users.students AS "s" ON "s".id = "sg".student_id`).
				Join(`INNER JOIN users.persons AS "p" ON "p".id = "s".person_id`).
				Where(`"sg".guardian_profile_id = ?`, profile.ID).
				Where(`"s".status <> ?`, "alumnus").
				OrderExpr(`"p".last_name ASC, "p".first_name ASC`).
				Scan(txCtx, &rows); err == nil {
				for _, row := range rows {
					child := profileChild{
						ID:          strconv.FormatInt(row.StudentID, 10),
						FirstName:   row.FirstName,
						LastName:    row.LastName,
						SchoolClass: row.SchoolClass,
					}
					if grade := parseLeadingGrade(row.SchoolClass); grade > 0 {
						child.GradeLevel = &grade
					}
					resp.Children = append(resp.Children, child)
				}
			}
			return nil
		})
	})
	if resolveErr != nil {
		common.RenderError(w, r, common.ErrorNotFound(resolveErr))
		return
	}

	common.Respond(w, r, http.StatusOK, resp, "Profile retrieved")
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
	FirstName        string         `json:"first_name"`
	LastName         string         `json:"last_name"`
	DateOfBirth      string         `json:"date_of_birth"`
	TargetGradeLevel *int16         `json:"target_grade_level,omitempty"`
	CustomData       map[string]any `json:"custom_data,omitempty"`
	OfferingIDs      []int64        `json:"offering_ids,omitempty"`
}

// submitParentEnrollmentRequest is the parent-authenticated submit
// body. Captcha is intentionally absent — the parent JWT IS the
// anti-bot signal; we skip captcha verification end-to-end.
type submitParentEnrollmentRequest struct {
	PhaseID           int64                      `json:"phase_id"`
	GuardianFirstName string                     `json:"guardian_first_name"`
	GuardianLastName  string                     `json:"guardian_last_name"`
	GuardianEmail     string                     `json:"guardian_email"`
	GuardianPhone     *string                    `json:"guardian_phone,omitempty"`
	ConsentFlags      map[string]any             `json:"consent_flags,omitempty"`
	CustomData        map[string]any             `json:"custom_data,omitempty"`
	Children          []submitParentChildRequest `json:"children"`
}

// submitParentResponse is what the embedded form receives after a
// successful authenticated submit. Same shape as the public path —
// intentional, so the form can share its onSubmitted handler.
type submitParentResponse struct {
	RequestID string `json:"request_id"`
	StatusURL string `json:"status_url"`
}

// submitParentEnrollment handles a parent-authenticated submission.
// The handler resolves the slug to a tenant via admin-tx, then runs
// the existing RequestService.Submit with GuardianAccountID stamped
// from claims.ID. Captcha is skipped — the JWT is the trust signal.
func (rs *Resource) submitParentEnrollment(w http.ResponseWriter, r *http.Request) {
	if rs.RequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("parent submit not configured")))
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
	)
	resolveErr := tenant.WithAdminTx(r.Context(), rs.db, func(adminCtx context.Context, _ bun.Tx) error {
		school, err := rs.SchoolRepo.FindBySlug(adminCtx, slug)
		if err != nil || school == nil || school.IsDeleted() {
			return errors.New("tenant not found")
		}

		tenantCtx := tenant.WithTenantID(adminCtx, school.ID)

		// No captcha. The parent JWT (issued only for accounts with
		// a guardian role on at least one school) is sufficient
		// anti-abuse signal; rate limiting still runs inside Submit.

		serviceReq, parseErr := buildParentServiceRequest(wireReq, school.ID, accountID)
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
// enrollment.RequestService input. Mirrors api/enrollment.buildServiceRequest
// but stamps GuardianAccountID and intentionally leaves RemoteIP empty
// (parent submissions skip IP-based rate-limiting since the JWT already
// identifies the actor).
func buildParentServiceRequest(wireReq *submitParentEnrollmentRequest, tenantID, accountID int64) (enrollmentService.SubmitRequest, error) {
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

// mapParentSubmitError mirrors mapSubmitError in api/enrollment, minus
// the captcha branch (parents never trigger captcha-shaped errors).
func mapParentSubmitError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrEnrollmentDisabled),
		errors.Is(err, enrollmentService.ErrEnrollmentWindowClosed):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingClosed),
		errors.Is(err, enrollmentService.ErrInvalidSubmission):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, enrollmentService.ErrCareOfferingFull):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, enrollmentService.ErrDuplicateEnrollment):
		common.RenderError(w, r, common.ErrorConflictMessage("Für dieses Kind liegt in dieser Phase bereits eine Anmeldung vor."))
	case errors.Is(err, enrollmentService.ErrRateLimited):
		w.Header().Set("Retry-After", "3600")
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
