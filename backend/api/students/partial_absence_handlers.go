package students

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type PartialAbsenceRequest struct {
	Date     string `json:"date"`
	FromTime string `json:"from_time"`
	Reason   string `json:"reason,omitempty"`
}

func (req *PartialAbsenceRequest) Bind(_ *http.Request) error {
	req.Date = strings.TrimSpace(req.Date)
	req.FromTime = strings.TrimSpace(req.FromTime)
	req.Reason = strings.TrimSpace(req.Reason)
	if _, err := timezone.ParseDate(req.Date); err != nil {
		return errors.New("invalid date format, expected YYYY-MM-DD")
	}
	if _, err := time.Parse("15:04", req.FromTime); err != nil {
		return errors.New("invalid from_time format, expected HH:MM")
	}
	if len(req.Reason) > 255 {
		return errors.New("reason cannot exceed 255 characters")
	}
	return nil
}

type PartialAbsenceResponse struct {
	ID         int64   `json:"id"`
	StudentID  int64   `json:"student_id"`
	Date       string  `json:"date"`
	FromTime   string  `json:"from_time"`
	Reason     *string `json:"reason,omitempty"`
	PickupTime *string `json:"pickup_time,omitempty"`
	CreatedBy  int64   `json:"created_by"`
	CreatedAt  string  `json:"created_at"`
	UpdatedAt  string  `json:"updated_at"`
}

func mapPartialAbsenceToResponse(row *schedule.StudentPickupException) PartialAbsenceResponse {
	response := PartialAbsenceResponse{
		ID:        row.ID,
		StudentID: row.StudentID,
		Date:      row.ExceptionDate.String(),
		Reason:    row.ExcusedReason,
		CreatedAt: row.CreatedAt.Format(time.RFC3339),
		UpdatedAt: row.UpdatedAt.Format(time.RFC3339),
	}
	if row.ExcusedCreatedBy != nil {
		response.CreatedBy = *row.ExcusedCreatedBy
	}
	if row.ExcusedFrom != nil {
		response.FromTime = timezone.WallClock(*row.ExcusedFrom).Format("15:04")
	}
	if row.PickupTime != nil {
		pickup := timezone.WallClock(*row.PickupTime).Format("15:04")
		response.PickupTime = &pickup
	}
	return response
}

func (rs *Resource) getStudentPartialAbsences(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupReadAccess(w, r)
	if student == nil {
		return
	}
	if rs.PartialAbsenceService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("partial absence service not configured")))
		return
	}
	from, to, err := parseStatusDayRange(r)
	if err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	rows, err := rs.PartialAbsenceService.List(r.Context(), student.ID, from, to)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to fetch partial absences", err))
		return
	}
	response := make([]PartialAbsenceResponse, 0, len(rows))
	for _, row := range rows {
		response = append(response, mapPartialAbsenceToResponse(row))
	}
	common.Respond(w, r, http.StatusOK, response, "Partial absences retrieved successfully")
}

func (rs *Resource) createStudentPartialAbsence(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "create partial absences")
	if student == nil {
		return
	}
	input, ok := rs.bindPartialAbsenceInput(w, r, student.ID)
	if !ok {
		return
	}
	row, err := rs.PartialAbsenceService.Create(r.Context(), input)
	if err != nil {
		rs.renderPartialAbsenceError(w, r, err)
		return
	}
	rs.registerPartialAbsenceBroadcasts(r, student.ID)
	common.Respond(w, r, http.StatusCreated, mapPartialAbsenceToResponse(row), "Partial absence created successfully")
}

func (rs *Resource) updateStudentPartialAbsence(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "update partial absences")
	if student == nil {
		return
	}
	id, ok := parseEntityID(w, r, "partialAbsenceId", "partial absence")
	if !ok {
		return
	}
	input, ok := rs.bindPartialAbsenceInput(w, r, student.ID)
	if !ok {
		return
	}
	row, err := rs.PartialAbsenceService.Update(r.Context(), id, input)
	if err != nil {
		rs.renderPartialAbsenceError(w, r, err)
		return
	}
	rs.registerPartialAbsenceBroadcasts(r, student.ID)
	common.Respond(w, r, http.StatusOK, mapPartialAbsenceToResponse(row), "Partial absence updated successfully")
}

func (rs *Resource) deleteStudentPartialAbsence(w http.ResponseWriter, r *http.Request) {
	student := rs.requirePickupWriteAccess(w, r, "delete partial absences")
	if student == nil {
		return
	}
	id, ok := parseEntityID(w, r, "partialAbsenceId", "partial absence")
	if !ok {
		return
	}
	if rs.PartialAbsenceService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("partial absence service not configured")))
		return
	}
	if err := rs.PartialAbsenceService.Delete(r.Context(), id, student.ID); err != nil {
		rs.renderPartialAbsenceError(w, r, err)
		return
	}
	rs.registerPartialAbsenceBroadcasts(r, student.ID)
	common.Respond(w, r, http.StatusOK, map[string]bool{"deleted": true}, "Partial absence deleted successfully")
}

func (rs *Resource) bindPartialAbsenceInput(w http.ResponseWriter, r *http.Request, studentID int64) (scheduleService.PartialAbsenceInput, bool) {
	if rs.PartialAbsenceService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("partial absence service not configured")))
		return scheduleService.PartialAbsenceInput{}, false
	}
	req := &PartialAbsenceRequest{}
	if err := render.Bind(r, req); err != nil {
		renderError(w, r, common.ErrorInvalidRequest(err))
		return scheduleService.PartialAbsenceInput{}, false
	}
	staffID, err := rs.getStaffIDFromJWT(r)
	if err != nil {
		renderError(w, r, common.ErrorForbidden(err))
		return scheduleService.PartialAbsenceInput{}, false
	}
	date, _ := timezone.ParseDate(req.Date)
	fromTime, _ := parseTimeOnly(req.FromTime)
	return scheduleService.PartialAbsenceInput{
		StudentID: studentID,
		Date:      date,
		FromTime:  fromTime,
		Reason:    req.Reason,
		StaffID:   staffID,
	}, true
}

var partialAbsenceErrorRenderer = common.RulesRenderer([]common.ErrorRule{
	{Target: scheduleService.ErrPartialAbsenceAlreadyExists, Render: common.ErrorConflict},
	{Target: scheduleService.ErrPartialAbsenceFullDayConflict, Render: common.ErrorConflict},
	{Target: scheduleService.ErrPartialAbsencePendingRequestConflict, Render: common.ErrorConflict},
	{Target: scheduleService.ErrPartialAbsencePickupConflict, Render: common.ErrorConflict},
	{Target: scheduleService.ErrPartialAbsenceNotFound, Render: partialAbsenceNotFound},
	{Target: scheduleService.ErrPartialAbsenceWrongStudent, Render: partialAbsenceNotFound},
	{Target: sql.ErrNoRows, Render: partialAbsenceNotFound},
}, func(err error) render.Renderer {
	return common.ErrorInternalServerWrap("failed to write partial absence", err)
})

func partialAbsenceNotFound(error) render.Renderer {
	return common.ErrorNotFound(errors.New("partial absence not found"))
}

func (rs *Resource) renderPartialAbsenceError(w http.ResponseWriter, r *http.Request, err error) {
	renderError(w, r, partialAbsenceErrorRenderer(err))
}

func (rs *Resource) registerPartialAbsenceBroadcasts(r *http.Request, studentID int64) {
	tenantID := tenant.FromContext(r.Context())
	tenant.RegisterAfterCommit(r.Context(), func() {
		rs.broadcastStudentUpdated(tenantID, studentID)
		rs.broadcastPickupScheduleChanged(tenantID, studentID)
		rs.wakeChildGuardians(tenantID, studentID)
	})
}
