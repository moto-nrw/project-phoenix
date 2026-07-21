package enrollment

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

type AdminChildDataCorrectionRequest struct {
	FirstName         string  `json:"first_name"`
	LastName          string  `json:"last_name"`
	DateOfBirth       string  `json:"date_of_birth"`
	TargetGradeLevel  *int16  `json:"target_grade_level"`
	TargetSchoolClass *string `json:"target_school_class"`
	Reason            string  `json:"reason"`
}

func (req *AdminChildDataCorrectionRequest) Bind(_ *http.Request) error { return nil }

func (rs *Resource) correctAdminChildData(w http.ResponseWriter, r *http.Request) {
	if rs.ChangeRequestService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("change request service not configured")))
		return
	}
	requestID, ok := common.ParsePositiveInt64IDWithError(w, r, "id", "invalid id")
	if !ok {
		return
	}
	childID, ok := common.ParsePositiveInt64IDWithError(w, r, "childId", "invalid childId")
	if !ok {
		return
	}
	body := &AdminChildDataCorrectionRequest{}
	if err := render.Bind(r, body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	dob, err := timezone.ParseDate(body.DateOfBirth)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	var corrected *enrollmentService.ChangeRequestAggregate
	err = rs.runInTenantTx(r, func(ctx context.Context) error {
		var correctionErr error
		corrected, correctionErr = rs.ChangeRequestService.CorrectApprovedChildData(ctx, enrollmentService.CorrectApprovedChildDataInput{
			RequestID:         requestID,
			ChildID:           childID,
			FirstName:         body.FirstName,
			LastName:          body.LastName,
			DateOfBirth:       dob,
			TargetGradeLevel:  body.TargetGradeLevel,
			TargetSchoolClass: body.TargetSchoolClass,
			Reason:            body.Reason,
			ActorAccountID:    int64(claims.ID),
		})
		return correctionErr
	})
	if err != nil {
		mapChangeRequestWriteError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, toChangeRequestResponse(corrected, true, true), "Enrollment child data corrected")
}
