package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// --- request/response payloads ---

// RolloverCreateRequest is the body of POST /admin/phases/{id}/rollover.
// Dates come in as ISO-8601 strings; the handler parses them.
type RolloverCreateRequest struct {
	Name              string  `json:"name"`
	Kind              string  `json:"kind"`
	ServiceStartDate  string  `json:"service_start_date"`
	ServiceEndDate    string  `json:"service_end_date"`
	EnrollmentOpenAt  *string `json:"enrollment_open_at,omitempty"`
	EnrollmentCloseAt *string `json:"enrollment_close_at,omitempty"`
	FormSchemaID      *int64  `json:"form_schema_id,omitempty"`

	RolloverMode        string `json:"rollover_mode"`
	RolloverAutoApprove bool   `json:"rollover_auto_approve"`
	RolloverDeadline    string `json:"rollover_deadline"`
	RolloverBumpsGrade  *bool  `json:"rollover_bumps_grade,omitempty"`
}

// Bind satisfies the chi/render.Binder contract — no extra validation
// here; the service does the full check.
func (req *RolloverCreateRequest) Bind(*http.Request) error { return nil }

// RolloverCreateResponse is what we hand back: the new phase + the
// summary so the admin UI can show "27 carried, 2 need review".
type RolloverCreateResponse struct {
	Phase   PhaseResponse   `json:"phase"`
	Summary RolloverSummary `json:"summary"`
}

// RolloverSummary mirrors the service-level result.
type RolloverSummary struct {
	SourceChildCount  int            `json:"source_child_count"`
	RolledCount       int            `json:"rolled_count"`
	ReviewCount       int            `json:"review_count"`
	ReviewByReason    map[string]int `json:"review_by_reason"`
	RequestCount      int            `json:"request_count"`
	EnqueuedEmails    int            `json:"enqueued_emails"`
	SkippedEmptyEmail int            `json:"skipped_empty_email"`
}

// ReviewItemResponse is one row in the admin review queue UI.
type ReviewItemResponse struct {
	ChildID           int64   `json:"child_id"`
	RequestID         int64   `json:"request_id"`
	FirstName         string  `json:"first_name"`
	LastName          string  `json:"last_name"`
	TargetGradeLevel  *int16  `json:"target_grade_level,omitempty"`
	ReviewReason      *string `json:"review_reason,omitempty"`
	SourceGradeLevel  *int16  `json:"source_grade_level,omitempty"`
	SourceFirstName   string  `json:"source_first_name,omitempty"`
	SourceLastName    string  `json:"source_last_name,omitempty"`
	GuardianFirstName string  `json:"guardian_first_name"`
	GuardianLastName  string  `json:"guardian_last_name"`
	GuardianEmail     string  `json:"guardian_email"`
	StatusToken       string  `json:"status_token"`
}

// RolloverReviewDecisionRequest is the body of POST
// /admin/request-children/{id}/rollover-review. Decision is one of
// keep|drop|defer. NewGradeLevel is only honoured for keep.
type RolloverReviewDecisionRequest struct {
	Decision      string `json:"decision"`
	NewGradeLevel *int16 `json:"new_grade_level,omitempty"`
}

func (req *RolloverReviewDecisionRequest) Bind(*http.Request) error { return nil }

// --- handlers ---

// createRollover handles POST /admin/phases/{id}/rollover.
func (rs *Resource) createRollover(w http.ResponseWriter, r *http.Request) {
	if rs.RolloverService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover service not configured")))
		return
	}

	sourceID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid source phase id")
	if !ok {
		return
	}

	body := &RolloverCreateRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	serviceReq, err := parseRolloverCreateRequest(sourceID, body, rs.adminAccountID(r))
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	var result *enrollmentService.RolloverResult
	txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		res, runErr := rs.RolloverService.CreatePhaseFromSource(ctx, serviceReq)
		result = res
		return runErr
	})
	if txErr != nil {
		rs.mapRolloverError(w, r, txErr)
		return
	}
	if result == nil || result.Phase == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover produced no result")))
		return
	}

	resp := RolloverCreateResponse{
		Phase: toPhaseResponse(result.Phase),
		Summary: RolloverSummary{
			SourceChildCount:  result.SourceChildCount,
			RolledCount:       result.RolledCount,
			ReviewCount:       result.ReviewCount,
			ReviewByReason:    result.ReviewByReason,
			RequestCount:      result.RequestCount,
			EnqueuedEmails:    result.EnqueuedEmails,
			SkippedEmptyEmail: result.SkippedEmptyEmail,
		},
	}
	common.Respond(w, r, http.StatusCreated, resp, "Rollover phase created")
}

// RolloverPreviewResponse mirrors the service-level dry-run summary
// shown before the admin executes the rollover (#2251).
type RolloverPreviewResponse struct {
	CarryCandidateCount int            `json:"carry_candidate_count"`
	CarriedCount        int            `json:"carried_count"`
	ReviewCount         int            `json:"review_count"`
	ReviewByReason      map[string]int `json:"review_by_reason"`
	ExcludedCount       int            `json:"excluded_count"`
	ExcludedByStatus    map[string]int `json:"excluded_by_status"`
	RequestCount        int            `json:"request_count"`
}

// previewRollover handles GET /admin/phases/{id}/rollover-preview.
// `bumps_grade` mirrors the checkbox on the rollover form so the
// review classification matches what the create would do.
func (rs *Resource) previewRollover(w http.ResponseWriter, r *http.Request) {
	if rs.RolloverService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover service not configured")))
		return
	}
	sourceID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid source phase id")
	if !ok {
		return
	}
	bumpsGrade := r.URL.Query().Get("bumps_grade") != "false"

	var preview *enrollmentService.RolloverPreview
	if txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		got, previewErr := rs.RolloverService.PreviewPhaseFromSource(ctx, sourceID, bumpsGrade)
		preview = got
		return previewErr
	}); txErr != nil {
		rs.mapRolloverError(w, r, txErr)
		return
	}
	if preview == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover preview not returned")))
		return
	}

	common.Respond(w, r, http.StatusOK, RolloverPreviewResponse{
		CarryCandidateCount: preview.CarryCandidateCount,
		CarriedCount:        preview.CarriedCount,
		ReviewCount:         preview.ReviewCount,
		ReviewByReason:      preview.ReviewByReason,
		ExcludedCount:       preview.ExcludedCount,
		ExcludedByStatus:    preview.ExcludedByStatus,
		RequestCount:        preview.RequestCount,
	}, "Rollover preview computed")
}

// listRolloverReview handles GET /admin/phases/{id}/review.
func (rs *Resource) listRolloverReview(w http.ResponseWriter, r *http.Request) {
	if rs.RolloverService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover service not configured")))
		return
	}
	phaseID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid phase id")
	if !ok {
		return
	}

	var items []*enrollmentService.ReviewQueueItem
	if txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		got, listErr := rs.RolloverService.ListReviewQueue(ctx, phaseID)
		items = got
		return listErr
	}); txErr != nil {
		rs.mapRolloverError(w, r, txErr)
		return
	}

	out := make([]ReviewItemResponse, 0, len(items))
	for _, it := range items {
		row := ReviewItemResponse{
			ChildID:           it.Child.ID,
			RequestID:         it.Request.ID,
			FirstName:         it.Child.FirstName,
			LastName:          it.Child.LastName,
			TargetGradeLevel:  it.Child.TargetGradeLevel,
			ReviewReason:      it.Child.ReviewReason,
			GuardianFirstName: it.Request.GuardianFirstName,
			GuardianLastName:  it.Request.GuardianLastName,
			GuardianEmail:     it.Request.GuardianEmail,
			StatusToken:       it.Request.StatusToken,
		}
		if it.SourceChild != nil {
			row.SourceGradeLevel = it.SourceChild.TargetGradeLevel
			row.SourceFirstName = it.SourceChild.FirstName
			row.SourceLastName = it.SourceChild.LastName
		}
		out = append(out, row)
	}
	common.Respond(w, r, http.StatusOK, out, "Review queue retrieved")
}

// decideRolloverReview handles POST
// /admin/request-children/{id}/rollover-review.
func (rs *Resource) decideRolloverReview(w http.ResponseWriter, r *http.Request) {
	if rs.RolloverService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("rollover service not configured")))
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid request_child id")
	if !ok {
		return
	}
	body := &RolloverReviewDecisionRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	req := enrollmentService.DecideReviewRequest{
		RequestChildID: childID,
		Decision:       body.Decision,
		NewGradeLevel:  body.NewGradeLevel,
		AdminAccountID: rs.adminAccountID(r),
	}

	if txErr := rs.runInTenantTx(r, func(ctx context.Context) error {
		return rs.RolloverService.DecideReview(ctx, req)
	}); txErr != nil {
		rs.mapRolloverError(w, r, txErr)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]string{"status": "ok"}, "Decision applied")
}

// --- helpers ---

func parseRolloverCreateRequest(sourceID int64, body *RolloverCreateRequest, adminAccountID int64) (enrollmentService.CreatePhaseFromSourceRequest, error) {
	out := enrollmentService.CreatePhaseFromSourceRequest{
		SourcePhaseID:       sourceID,
		Name:                body.Name,
		Kind:                body.Kind,
		FormSchemaID:        body.FormSchemaID,
		RolloverMode:        body.RolloverMode,
		RolloverAutoApprove: body.RolloverAutoApprove,
		AdminAccountID:      adminAccountID,
	}
	// rollover_bumps_grade defaults to true (yearly cadence). Only an
	// explicit `false` from the form turns it off.
	out.RolloverBumpsGrade = true
	if body.RolloverBumpsGrade != nil {
		out.RolloverBumpsGrade = *body.RolloverBumpsGrade
	}

	if out.Kind == "" {
		out.Kind = enrollmentModels.PhaseKindSchoolYear
	}

	start, err := parseDateField("service_start_date", body.ServiceStartDate)
	if err != nil {
		return out, err
	}
	end, err := parseDateField("service_end_date", body.ServiceEndDate)
	if err != nil {
		return out, err
	}
	out.ServiceStartDate = start
	out.ServiceEndDate = end

	if body.EnrollmentOpenAt != nil && *body.EnrollmentOpenAt != "" {
		t, err := parseDateTimeField("enrollment_open_at", *body.EnrollmentOpenAt)
		if err != nil {
			return out, err
		}
		out.EnrollmentOpenAt = &t
	}
	if body.EnrollmentCloseAt != nil && *body.EnrollmentCloseAt != "" {
		t, err := parseDateTimeField("enrollment_close_at", *body.EnrollmentCloseAt)
		if err != nil {
			return out, err
		}
		out.EnrollmentCloseAt = &t
	}

	deadline, err := parseDateTimeField("rollover_deadline", body.RolloverDeadline)
	if err != nil {
		return out, err
	}
	out.RolloverDeadline = deadline

	return out, nil
}

// parseDateField accepts YYYY-MM-DD; parseDateTimeField accepts
// RFC3339. Returned errors include the field name so the admin sees a
// useful 400.
func parseDateField(name, raw string) (timezone.Date, error) {
	if raw == "" {
		return timezone.Date(""), fmt.Errorf("%s is required", name)
	}
	d, err := timezone.ParseDate(raw)
	if err != nil {
		return timezone.Date(""), fmt.Errorf("%s must be YYYY-MM-DD: %w", name, err)
	}
	return d, nil
}

func parseDateTimeField(name, raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, fmt.Errorf("%s is required", name)
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", name, err)
	}
	return t, nil
}

// adminAccountID extracts the calling admin's account id from JWT
// claims. Falls back to 0 when not present; UpdateRolloverReview
// tolerates 0 for tests that bypass auth.
func (rs *Resource) adminAccountID(r *http.Request) int64 {
	claims := jwt.ClaimsFromCtx(r.Context())
	return int64(claims.ID)
}

// Stable error codes returned in the response envelope so the frontend
// can map to localized German messages without parsing the free-form
// `error` string. Keep in sync with the matching map in
// `frontend/src/lib/enrollment-phase-api.ts`.
const (
	ErrCodeRolloverSourceNotFound      = "rollover.source_not_found"
	ErrCodeRolloverInvalidRequest      = "rollover.invalid_request"
	ErrCodeRolloverReviewInvalid       = "rollover.review_invalid"
	ErrCodeRolloverReviewNotFound      = "rollover.review_not_found"
	ErrCodeRolloverDuplicateName       = "rollover.duplicate_name"
	ErrCodeRolloverSourceAlreadyRolled = "rollover.source_already_rolled"
)

func (rs *Resource) mapRolloverError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, enrollmentService.ErrRolloverSourceNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, ErrCodeRolloverSourceNotFound))
	case errors.Is(err, enrollmentService.ErrRolloverInvalidRequest):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeRolloverInvalidRequest))
	case errors.Is(err, enrollmentService.ErrRolloverReviewInvalid):
		common.RenderError(w, r, common.ErrorInvalidRequestWithCode(err, ErrCodeRolloverReviewInvalid))
	case errors.Is(err, enrollmentService.ErrRolloverReviewNotFound):
		common.RenderError(w, r, common.ErrorNotFoundWithCode(err, ErrCodeRolloverReviewNotFound))
	case errors.Is(err, enrollmentService.ErrRolloverDuplicateName):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeRolloverDuplicateName))
	case errors.Is(err, enrollmentService.ErrRolloverSourceAlreadyRolled):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, ErrCodeRolloverSourceAlreadyRolled))
	default:
		rs.logger().Error("rollover handler failed", slog.String("error", err.Error()))
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}

// logger returns slog.Default — the Resource doesn't carry a scoped
// logger today. Kept as a helper so swapping later is one edit.
func (rs *Resource) logger() *slog.Logger {
	return slog.Default()
}
