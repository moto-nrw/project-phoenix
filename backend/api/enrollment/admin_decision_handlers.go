package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// AdminRequestSummary is the wire shape for the admin list page.
// Carries the request + per-child overview + the phase name so the
// list can render without a second fetch.
type AdminRequestSummary struct {
	ID                string              `json:"id"`
	PhaseID           string              `json:"phase_id"`
	PhaseName         string              `json:"phase_name"`
	GuardianFirstName string              `json:"guardian_first_name"`
	GuardianLastName  string              `json:"guardian_last_name"`
	GuardianEmail     string              `json:"guardian_email"`
	GuardianPhone     *string             `json:"guardian_phone,omitempty"`
	SubmittedAt       time.Time           `json:"submitted_at"`
	WithdrawnAt       *time.Time          `json:"withdrawn_at,omitempty"`
	StatusToken       string              `json:"status_token"`
	Children          []AdminRequestChild `json:"children"`
}

// AdminRequestChild is one child within an admin summary/detail
// payload.
type AdminRequestChild struct {
	ID               string     `json:"id"`
	FirstName        string     `json:"first_name"`
	LastName         string     `json:"last_name"`
	DateOfBirth      string     `json:"date_of_birth"`
	TargetGradeLevel *int16     `json:"target_grade_level,omitempty"`
	Status           string     `json:"status"`
	StatusReason     *string    `json:"status_reason,omitempty"`
	ReviewedAt       *time.Time `json:"reviewed_at,omitempty"`
	ReviewedBy       *int64     `json:"reviewed_by,omitempty"`
	ActivationMode   string     `json:"activation_mode"`
}

func toAdminRequestSummary(s *enrollmentService.RequestSummary) AdminRequestSummary {
	out := AdminRequestSummary{
		ID:                strconv.FormatInt(s.Request.ID, 10),
		PhaseID:           strconv.FormatInt(s.Request.PhaseID, 10),
		GuardianFirstName: s.Request.GuardianFirstName,
		GuardianLastName:  s.Request.GuardianLastName,
		GuardianEmail:     s.Request.GuardianEmail,
		GuardianPhone:     s.Request.GuardianPhone,
		SubmittedAt:       s.Request.SubmittedAt,
		WithdrawnAt:       s.Request.WithdrawnAt,
		StatusToken:       s.Request.StatusToken,
	}
	if s.Phase != nil {
		out.PhaseName = s.Phase.Name
	}
	out.Children = make([]AdminRequestChild, 0, len(s.Children))
	for _, c := range s.Children {
		out.Children = append(out.Children, AdminRequestChild{
			ID:               strconv.FormatInt(c.ID, 10),
			FirstName:        c.FirstName,
			LastName:         c.LastName,
			DateOfBirth:      c.DateOfBirth.Format("2006-01-02"),
			TargetGradeLevel: c.TargetGradeLevel,
			Status:           c.Status,
			StatusReason:     c.StatusReason,
			ReviewedAt:       c.ReviewedAt,
			ReviewedBy:       c.ReviewedBy,
			ActivationMode:   c.ActivationMode,
		})
	}
	return out
}

// listAdminRequests returns the queue of submissions for the tenant in
// session. Filters: phase_id, child_status. Both optional.
func (rs *Resource) listAdminRequests(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}

	filters := enrollmentService.RequestFilters{}
	if v := r.URL.Query().Get("phase_id"); v != "" {
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil || id <= 0 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid phase_id")))
			return
		}
		filters.PhaseID = id
	}
	if v := r.URL.Query().Get("child_status"); v != "" {
		filters.ChildStatus = v
	}

	var summaries []*enrollmentService.RequestSummary
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		list, listErr := rs.DecisionService.List(ctx, filters)
		summaries = list
		return listErr
	})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}

	out := make([]AdminRequestSummary, 0, len(summaries))
	for _, s := range summaries {
		out = append(out, toAdminRequestSummary(s))
	}
	common.Respond(w, r, http.StatusOK, out, "Admin requests retrieved")
}

func (rs *Resource) getAdminRequest(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}

	var summary *enrollmentService.RequestSummary
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		s, e := rs.DecisionService.Get(ctx, id)
		summary = s
		return e
	})
	if err != nil {
		if errors.Is(err, enrollmentService.ErrDecisionRequestNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(err))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	common.Respond(w, r, http.StatusOK, toAdminRequestSummary(summary), "Admin request retrieved")
}

// AdminDecideRequest is the body of POST .../children/{childId}/decide.
type AdminDecideRequest struct {
	Status string `json:"status"` // approved | waitlisted | rejected | under_review
	Reason string `json:"reason,omitempty"`
}

func (req *AdminDecideRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) decideAdminChild(w http.ResponseWriter, r *http.Request) {
	if rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("decision service not configured")))
		return
	}
	requestID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || requestID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id")))
		return
	}
	childID, err := strconv.ParseInt(chi.URLParam(r, "childId"), 10, 64)
	if err != nil || childID <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid childId")))
		return
	}
	body := &AdminDecideRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	reviewedBy := int64(claims.ID)

	var updated *enrollmentModels.RequestChild
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		out, e := rs.DecisionService.Decide(ctx, enrollmentService.DecideInput{
			RequestID:  requestID,
			ChildID:    childID,
			Status:     enrollmentService.DecisionStatus(body.Status),
			Reason:     body.Reason,
			ReviewedBy: reviewedBy,
		})
		updated = out
		return e
	})
	if err != nil {
		switch {
		case errors.Is(err, enrollmentService.ErrDecisionChildNotFound):
			common.RenderError(w, r, common.ErrorNotFound(err))
		case errors.Is(err, enrollmentService.ErrDecisionInvalidStatus),
			errors.Is(err, enrollmentService.ErrDecisionAlreadyTerminal):
			common.RenderError(w, r, common.ErrorInvalidRequest(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServer(err))
		}
		return
	}

	common.Respond(w, r, http.StatusOK, AdminRequestChild{
		ID:               strconv.FormatInt(updated.ID, 10),
		FirstName:        updated.FirstName,
		LastName:         updated.LastName,
		DateOfBirth:      updated.DateOfBirth.Format("2006-01-02"),
		TargetGradeLevel: updated.TargetGradeLevel,
		Status:           updated.Status,
		StatusReason:     updated.StatusReason,
		ReviewedAt:       updated.ReviewedAt,
		ReviewedBy:       updated.ReviewedBy,
		ActivationMode:   updated.ActivationMode,
	}, "Decision applied")
}
