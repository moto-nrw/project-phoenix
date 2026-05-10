package timetable

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
)

func (rs *Resource) operationsPlannedNow(w http.ResponseWriter, r *http.Request) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	date := timezone.TodayUTC()
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := time.Parse(dateLayout, raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date")))
			return
		}
		date = parsed
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.PlannedNow(r.Context(), int64(claims.ID), claims.IsAdmin, date, timezone.Now())
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"instances": result}, "Planned timetable instances retrieved")
}

func (rs *Resource) operationsRoster(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		return rs.operationsService.Roster(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
	}, "Timetable roster retrieved")
}

func (rs *Resource) operationsRosterByActiveGroup(w http.ResponseWriter, r *http.Request) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	activeGroupID, ok := parseOperationID(w, r, "id")
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.RosterByActiveGroup(r.Context(), int64(claims.ID), claims.IsAdmin, activeGroupID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable roster retrieved")
}

func (rs *Resource) operationsStart(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		result, err := rs.operationsService.Start(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
		if err != nil {
			return nil, err
		}
		return startOperationResponse{
			InstanceID:    result.Instance.ID,
			Status:        result.Instance.Status,
			ActiveGroupID: result.ActiveGroupID,
			Warnings:      result.Warnings,
		}, nil
	}, "Timetable instance started")
}

func (rs *Resource) operationsComplete(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		return rs.operationsService.Complete(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
	}, "Timetable instance completed")
}

func (rs *Resource) operationsCheckInStudent(w http.ResponseWriter, r *http.Request) {
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.CheckInStudent(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Student checked in to timetable instance")
}

func (rs *Resource) operationsCheckOutStudent(w http.ResponseWriter, r *http.Request) {
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.CheckOutStudent(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Student checked out from timetable instance")
}

func (rs *Resource) operationsPatchAttendance(w http.ResponseWriter, r *http.Request) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	req, ok := decodePatchBody(w, r)
	if !ok {
		return
	}
	patch, parseErrs := parseAttendancePatchRequest(req)
	if len(parseErrs) > 0 {
		renderValidationErrors(w, r, parseErrs)
		return
	}
	if !patch.HasChanges() {
		renderValidationErrors(w, r, []fieldError{{Field: "body", Reason: "at least one of status, substatus, note must be set"}})
		return
	}
	if rs.instanceStudentRepo == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("instance student repository not wired")))
		return
	}
	current, err := rs.instanceStudentRepo.FindByInstanceAndStudent(r.Context(), instanceID, studentID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServer(err))
		return
	}
	if current == nil {
		rs.renderOperationsError(w, r, scheduleSvc.ErrTimetableOperationNotFound)
		return
	}
	if verrs := validateAttendancePatch(patch, current); len(verrs) > 0 {
		renderValidationErrors(w, r, verrs)
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.PatchAttendance(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID, patch)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable attendance updated")
}

func (rs *Resource) withOperationInstance(w http.ResponseWriter, r *http.Request, fn func(int64) (any, error), message string) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	instanceID, ok := parseOperationID(w, r, "id")
	if !ok {
		return
	}
	result, err := fn(instanceID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, message)
}

func parseOperationInstanceStudentIDs(w http.ResponseWriter, r *http.Request) (int64, int64, bool) {
	instanceID, ok := parseOperationID(w, r, "id")
	if !ok {
		return 0, 0, false
	}
	studentID, ok := parseOperationID(w, r, "student_id")
	if !ok {
		return 0, 0, false
	}
	return instanceID, studentID, true
}

func parseOperationID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	raw := chi.URLParam(r, key)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid id parameter")))
		return 0, false
	}
	return id, true
}

type startOperationResponse struct {
	InstanceID    int64                                 `json:"instance_id"`
	Status        string                                `json:"status"`
	ActiveGroupID int64                                 `json:"active_group_id"`
	Warnings      []scheduleSvc.InstanceConflictWarning `json:"warnings"`
}

func (rs *Resource) renderOperationsError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, scheduleSvc.ErrTimetableOperationForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, scheduleSvc.ErrTimetableOperationNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, scheduleSvc.ErrTimetableOperationConflict), errors.Is(err, scheduleSvc.ErrInvalidInstanceTransition):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, scheduleSvc.ErrInstanceNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, activeSvc.ErrStudentAlreadyActive), errors.Is(err, activeSvc.ErrRoomConflict):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, activeSvc.ErrStudentNotFound), errors.Is(err, activeSvc.ErrVisitNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, activeSvc.ErrInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
