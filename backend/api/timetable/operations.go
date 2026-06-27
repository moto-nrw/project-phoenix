package timetable

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/constants"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activityModel "github.com/moto-nrw/project-phoenix/models/activities"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	activeSvc "github.com/moto-nrw/project-phoenix/services/active"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type spontaneousStartRequest struct {
	Title           string  `json:"title"`
	Description     *string `json:"description,omitempty"`
	Notes           *string `json:"notes,omitempty"`
	RoomID          int64   `json:"room_id"`
	ActivityGroupID *int64  `json:"activity_group_id,omitempty"`
	StaffIDs        []int64 `json:"staff_ids,omitempty"`
	StudentIDs      []int64 `json:"student_ids,omitempty"`
}

func (req *spontaneousStartRequest) Bind(_ *http.Request) error {
	if req.Title == "" {
		return errors.New("title is required")
	}
	if len(req.Title) > 255 {
		return errors.New("title cannot exceed 255 characters")
	}
	if req.RoomID <= 0 {
		return errors.New("room_id is required")
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		return errors.New("title is required")
	}
	return nil
}

func (rs *Resource) operationsPlannedNow(w http.ResponseWriter, r *http.Request) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	date := timezone.TodayDate()
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := timezone.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date")))
			return
		}
		date = parsed
	}
	opts, ok := parsePlannedNowOptions(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.PlannedNow(r.Context(), int64(claims.ID), claims.IsAdmin, date, timezone.Now(), opts)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"instances": result}, "Planned timetable instances retrieved")
}

func parsePlannedNowOptions(w http.ResponseWriter, r *http.Request) (scheduleSvc.PlannedNowOptions, bool) {
	query := r.URL.Query()
	var opts scheduleSvc.PlannedNowOptions
	if raw := query.Get("horizon_minutes"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 24*60 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid horizon_minutes")))
			return opts, false
		}
		opts.HorizonMinutes = value
	}
	if raw := query.Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 50 {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid limit")))
			return opts, false
		}
		opts.Limit = value
	}
	if raw := query.Get("include_roster"); raw != "" {
		value, err := strconv.ParseBool(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid include_roster")))
			return opts, false
		}
		opts.IncludeRoster = value
	}
	return opts, true
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

func (rs *Resource) operationsCreateAndStartSpontaneous(w http.ResponseWriter, r *http.Request) {
	if rs.instanceService == nil || rs.operationsService == nil || rs.timetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations resource not fully wired")))
		return
	}
	if !rs.webSpontaneousActivitiesEnabled(r) {
		common.RenderError(w, r, common.ErrorForbidden(scheduleSvc.ErrTimetableOperationForbidden))
		return
	}
	req, ok := bindSpontaneousStartRequest(w, r)
	if !ok {
		return
	}
	if len(req.StudentIDs) > 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids are not accepted for spontaneous operational starts")))
		return
	}
	room, err := rs.timetableData.GetRoom(r.Context(), req.RoomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("room not found")))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load spontaneous room failed", err))
		return
	}
	if room == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("room not found")))
		return
	}
	if room.Name == constants.SchulhofRoomName {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("für den Schulhof bitte die Schulhof-Aufsicht verwenden")))
		return
	}
	if err := rs.lockSpontaneousStartRoom(r.Context(), req.RoomID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("lock spontaneous start room failed", err))
		return
	}
	hasRoomConflict, _, err := rs.timetableData.CheckRoomConflict(r.Context(), req.RoomID, 0)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("check room conflict failed", err))
		return
	}
	if hasRoomConflict {
		common.RenderError(w, r, common.ErrorConflict(activeSvc.ErrRoomConflict))
		return
	}

	currentStaffID := rs.resolveStartedByStaffID(r.Context())
	if currentStaffID <= 0 {
		common.RenderError(w, r, common.ErrorForbidden(scheduleSvc.ErrTimetableOperationForbidden))
		return
	}

	req.StaffIDs = appendUniquePositive(req.StaffIDs, currentStaffID)
	createdBy := currentStaffID
	activityGroupID, err := rs.resolveSpontaneousActivityGroupID(r.Context(), req.Title, req.ActivityGroupID, createdBy)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("resolve spontaneous activity group failed", err))
		return
	}
	isSpontaneous := true
	window := serverSpontaneousActivityWindow(timezone.Now())
	inst, err := rs.instanceService.Create(r.Context(), scheduleSvc.CreateInstanceInput{
		Date:             window.date,
		StartTime:        window.startTime,
		EndTime:          window.endTime,
		Title:            req.Title,
		Description:      req.Description,
		Notes:            req.Notes,
		RoomID:           req.RoomID,
		ActivityGroupID:  activityGroupID,
		IsSpontaneous:    &isSpontaneous,
		StaffIDs:         req.StaffIDs,
		StudentIDs:       nil,
		CreatedByStaffID: &createdBy,
	})
	if err != nil {
		renderCreateInstanceError(w, r, err)
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.operationsService.Start(r.Context(), int64(claims.ID), claims.IsAdmin, inst.ID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, startOperationResponse{
		InstanceID:    result.Instance.ID,
		Status:        result.Instance.Status,
		ActiveGroupID: result.ActiveGroupID,
		Warnings:      result.Warnings,
	}, "Spontaneous timetable instance created and started")
}

type spontaneousActivityWindow struct {
	date      timezone.Date
	startTime time.Time
	endTime   time.Time
}

func bindSpontaneousStartRequest(w http.ResponseWriter, r *http.Request) (*spontaneousStartRequest, bool) {
	req := &spontaneousStartRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, false
	}
	return req, true
}

func (rs *Resource) resolveSpontaneousActivityGroupID(ctx context.Context, title string, requestedID *int64, createdBy int64) (*int64, error) {
	if requestedID != nil {
		return requestedID, nil
	}
	if rs.timetableData == nil {
		return nil, errors.New("activity repositories are not wired")
	}
	if err := rs.lockSpontaneousActivityName(ctx, title); err != nil {
		return nil, err
	}
	if existing, err := rs.timetableData.GetActivityGroupByName(ctx, title); err == nil && existing != nil {
		return &existing.ID, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	category, err := rs.ensureSpontaneousActivityCategory(ctx)
	if err != nil {
		return nil, err
	}
	group := &activityModel.Group{
		Name:            title,
		CategoryID:      category.ID,
		MaxParticipants: 999,
		IsOpen:          true,
		CreatedBy:       &createdBy,
		Type:            activityModel.GroupTypeActivity,
		IsTemplate:      false,
	}
	group.SetTenantID(tenant.FromContext(ctx))
	if err := rs.timetableData.CreateActivityGroup(ctx, group); err != nil {
		return nil, err
	}
	return &group.ID, nil
}

func (rs *Resource) ensureSpontaneousActivityCategory(ctx context.Context) (*activityModel.Category, error) {
	const spontaneousCategoryName = "Spontan"
	if err := rs.lockSpontaneousActivityCategory(ctx); err != nil {
		return nil, err
	}
	if existing, err := rs.timetableData.GetActivityCategoryByName(ctx, spontaneousCategoryName); err == nil && existing != nil {
		return existing, nil
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}

	category := &activityModel.Category{
		Name:        spontaneousCategoryName,
		Description: "Automatisch angelegte Aktivitäten aus spontanen Web-Starts",
		Color:       "#83CD2D",
	}
	category.SetTenantID(tenant.FromContext(ctx))
	if err := rs.timetableData.CreateActivityCategory(ctx, category); err != nil {
		return nil, err
	}
	return category, nil
}

func serverSpontaneousActivityWindow(now time.Time) spontaneousActivityWindow {
	now = now.In(timezone.Berlin)
	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := min(currentMinutes, 23*60+30)
	endMinutes := min(startMinutes+60, 23*60+59)
	return spontaneousActivityWindow{
		date:      timezone.DateFromTime(now),
		startTime: clockTimeFromMinutes(startMinutes),
		endTime:   clockTimeFromMinutes(endMinutes),
	}
}

func clockTimeFromMinutes(minutes int) time.Time {
	return time.Date(2000, 1, 1, minutes/60, minutes%60, 0, 0, time.UTC)
}

func (rs *Resource) lockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	return rs.timetableData.LockSpontaneousStartRoom(ctx, roomID)
}

func (rs *Resource) lockSpontaneousActivityName(ctx context.Context, name string) error {
	return rs.timetableData.LockSpontaneousActivityName(ctx, name)
}

func (rs *Resource) lockSpontaneousActivityCategory(ctx context.Context) error {
	return rs.timetableData.LockSpontaneousActivityCategory(ctx)
}

func (rs *Resource) operationsCapabilities(w http.ResponseWriter, r *http.Request) {
	common.Respond(w, r, http.StatusOK, map[string]any{
		"web_spontaneous_activities_enabled": rs.webSpontaneousActivitiesEnabled(r),
	}, "Timetable operation capabilities retrieved")
}

func (rs *Resource) webSpontaneousActivitiesEnabled(r *http.Request) bool {
	logger := rs.logger
	if logger == nil {
		logger = slog.Default()
	}
	careConcept := configSvc.ResolveStringOrDefault(
		r.Context(),
		rs.settingsService,
		configModel.KeyCareConcept,
		configModel.CareConceptOpenRooms,
		logger,
	)
	if careConcept != configModel.CareConceptOpenRooms {
		return false
	}
	return configSvc.ResolveBoolOrDefault(
		r.Context(),
		rs.settingsService,
		configModel.KeyWebSpontaneousActivities,
		true,
		logger,
	)
}

func (rs *Resource) operationsComplete(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		return rs.operationsService.Complete(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
	}, "Timetable instance completed")
}

func (rs *Resource) operationsCheckInStudent(w http.ResponseWriter, r *http.Request) {
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
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
	if rs.operationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
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

func appendUniquePositive(ids []int64, id int64) []int64 {
	if id <= 0 {
		return ids
	}
	for _, existing := range ids {
		if existing == id {
			return ids
		}
	}
	return append(ids, id)
}

func (rs *Resource) renderOperationsError(w http.ResponseWriter, r *http.Request, err error) {
	var validationErr *scheduleSvc.TimetableAttendanceValidationError
	switch {
	case errors.As(err, &validationErr):
		renderValidationErrors(w, r, attendancePatchFieldErrors(validationErr.Fields))
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
