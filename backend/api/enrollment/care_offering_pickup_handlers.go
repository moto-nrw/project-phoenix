package enrollment

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// Angebots-Gehzeit rollout endpoints (#2290): the admin editor previews the
// impact of an offering's pickup_times and executes the rollout with the
// dialog's per-child opt-outs.

// OfferingPickupConflictResponse is one child whose manually maintained
// Gehzeit differs from the Angebots-Gehzeit; IDs stringified per convention.
type OfferingPickupConflictResponse struct {
	StudentID   string `json:"student_id"`
	StudentName string `json:"student_name"`
	Weekday     int    `json:"weekday"`
	CurrentTime string `json:"current_time"`
	NewTime     string `json:"new_time"`
}

type OfferingPickupRolloutPreviewResponse struct {
	AffectedStudents int                              `json:"affected_students"`
	NewRows          int                              `json:"new_rows"`
	UpdatedRows      int                              `json:"updated_rows"`
	RemovedRows      int                              `json:"removed_rows"`
	Conflicts        []OfferingPickupConflictResponse `json:"conflicts"`
}

type OfferingPickupRolloutRequest struct {
	SkipStudentIDs []string `json:"skip_student_ids"`
}

func (req *OfferingPickupRolloutRequest) Bind(_ *http.Request) error {
	if req.SkipStudentIDs == nil {
		req.SkipStudentIDs = []string{}
	}
	return nil
}

type OfferingPickupRolloutResultResponse struct {
	CreatedRows     int `json:"created_rows"`
	UpdatedRows     int `json:"updated_rows"`
	DeletedRows     int `json:"deleted_rows"`
	SkippedStudents int `json:"skipped_students"`
}

func (rs *Resource) offeringPickupService(w http.ResponseWriter, r *http.Request) (enrollmentService.OfferingPickupTimeService, bool) {
	svc, ok := rs.DecisionService.(enrollmentService.OfferingPickupTimeService)
	if !ok || rs.DecisionService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("offering pickup service not configured")))
		return nil, false
	}
	return svc, true
}

func (rs *Resource) previewCareOfferingPickupRollout(w http.ResponseWriter, r *http.Request) {
	svc, ok := rs.offeringPickupService(w, r)
	if !ok {
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	var preview *enrollmentService.OfferingPickupRolloutPreview
	err := rs.runInTenantTx(r, func(ctx context.Context) error {
		p, e := svc.PreviewOfferingPickupRollout(ctx, id)
		preview = p
		return e
	})
	if err != nil {
		renderOfferingPickupError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toOfferingPickupPreviewResponse(preview), "Rollout preview computed")
}

func (rs *Resource) rolloutCareOfferingPickupTimes(w http.ResponseWriter, r *http.Request) {
	svc, ok := rs.offeringPickupService(w, r)
	if !ok {
		return
	}
	id, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	req := &OfferingPickupRolloutRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	skipIDs, err := parseCareOfferingIDStrings(req.SkipStudentIDs, "skip_student_ids")
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	var result *enrollmentService.OfferingPickupRolloutResult
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		res, e := svc.RolloutOfferingPickupTimes(ctx, id, skipIDs, int64(claims.ID))
		result = res
		return e
	})
	if err != nil {
		renderOfferingPickupError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, OfferingPickupRolloutResultResponse{
		CreatedRows:     result.CreatedRows,
		UpdatedRows:     result.UpdatedRows,
		DeletedRows:     result.DeletedRows,
		SkippedStudents: result.SkippedStudents,
	}, "Rollout executed")
}

func renderOfferingPickupError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, enrollmentService.ErrCareOfferingNotFound) {
		common.RenderError(w, r, common.ErrorNotFound(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("offering pickup rollout failed", err))
}

func toOfferingPickupPreviewResponse(preview *enrollmentService.OfferingPickupRolloutPreview) OfferingPickupRolloutPreviewResponse {
	out := OfferingPickupRolloutPreviewResponse{
		AffectedStudents: preview.AffectedStudents,
		NewRows:          preview.NewRows,
		UpdatedRows:      preview.UpdatedRows,
		RemovedRows:      preview.RemovedRows,
		Conflicts:        make([]OfferingPickupConflictResponse, 0, len(preview.Conflicts)),
	}
	for _, conflict := range preview.Conflicts {
		out.Conflicts = append(out.Conflicts, OfferingPickupConflictResponse{
			StudentID:   strconv.FormatInt(conflict.StudentID, 10),
			StudentName: conflict.StudentName,
			Weekday:     conflict.Weekday,
			CurrentTime: conflict.CurrentTime,
			NewTime:     conflict.NewTime,
		})
	}
	return out
}
