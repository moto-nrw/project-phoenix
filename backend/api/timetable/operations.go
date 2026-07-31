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
	if rs.OperationsService == nil {
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
	result, err := rs.OperationsService.PlannedNow(r.Context(), int64(claims.ID), claims.IsAdmin, date, timezone.Now(), opts)
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
		return rs.OperationsService.Roster(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
	}, "Timetable roster retrieved")
}

func (rs *Resource) operationsRosterByActiveGroup(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	activeGroupID, ok := parseOperationID(w, r, "id")
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.OperationsService.RosterByActiveGroup(r.Context(), int64(claims.ID), claims.IsAdmin, activeGroupID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable roster retrieved")
}

func (rs *Resource) operationsStart(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		result, err := rs.OperationsService.Start(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
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
	if rs.OperationsService == nil || rs.TimetableData == nil {
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
	window, err := spontaneousStartWorkdayWindow(rs.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	if len(req.StudentIDs) > 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids are not accepted for spontaneous operational starts")))
		return
	}
	if !rs.validateSpontaneousRoom(w, r, req.RoomID) {
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
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.OperationsService.CreateAndStartSpontaneous(r.Context(), int64(claims.ID), claims.IsAdmin, scheduleSvc.CreateInstanceInput{
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
		// The create+start composition owns its own rollback (both phases share
		// the request tx). A Create-phase failure is wrapped so it keeps the
		// create-specific error mapping; a Start-phase failure uses the
		// operations mapping.
		var createErr *scheduleSvc.SpontaneousCreateError
		if errors.As(err, &createErr) {
			renderCreateInstanceError(w, r, createErr.Err)
		} else {
			rs.renderOperationsError(w, r, err)
		}
		return
	}
	common.Respond(w, r, http.StatusCreated, startOperationResponse{
		InstanceID:    result.Instance.ID,
		Status:        result.Instance.Status,
		ActiveGroupID: result.ActiveGroupID,
		Warnings:      result.Warnings,
	}, "Spontaneous timetable instance created and started")
}

// validateSpontaneousRoom checks the target room exists, is not the permanent
// Schulhof room (which has its own supervision flow), and is currently
// unoccupied — taking the spontaneous-start room lock in between so the
// existence-vs-conflict check is serialized. Renders the appropriate error and
// returns false on any failure.
func (rs *Resource) validateSpontaneousRoom(w http.ResponseWriter, r *http.Request, roomID int64) bool {
	room, err := rs.TimetableData.GetRoom(r.Context(), roomID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("room not found")))
			return false
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("load spontaneous room failed", err))
		return false
	}
	if room == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("room not found")))
		return false
	}
	if room.Name == constants.SchulhofRoomName {
		common.RenderError(w, r, common.ErrorConflictWithCode(
			scheduleSvc.ErrSchulhofSupervisionRequired,
			schulhofSupervisionRequiredCode,
		))
		return false
	}
	if err := rs.lockSpontaneousStartRoom(r.Context(), roomID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("lock spontaneous start room failed", err))
		return false
	}
	hasRoomConflict, _, err := rs.TimetableData.CheckRoomConflict(r.Context(), roomID, 0)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("check room conflict failed", err))
		return false
	}
	if hasRoomConflict {
		common.RenderError(w, r, common.ErrorConflict(activeSvc.ErrRoomConflict))
		return false
	}
	return true
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
	if rs.TimetableData == nil {
		return nil, errors.New("activity repositories are not wired")
	}
	if err := rs.lockSpontaneousActivityName(ctx, title); err != nil {
		return nil, err
	}
	if existing, err := rs.TimetableData.GetActivityGroupByName(ctx, title); err == nil && existing != nil {
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
	if err := rs.TimetableData.CreateActivityGroup(ctx, group); err != nil {
		return nil, err
	}
	return &group.ID, nil
}

func (rs *Resource) ensureSpontaneousActivityCategory(ctx context.Context) (*activityModel.Category, error) {
	const spontaneousCategoryName = "Spontan"
	if err := rs.lockSpontaneousActivityCategory(ctx); err != nil {
		return nil, err
	}
	if existing, err := rs.TimetableData.GetActivityCategoryByName(ctx, spontaneousCategoryName); err == nil && existing != nil {
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
	if err := rs.TimetableData.CreateActivityCategory(ctx, category); err != nil {
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

func spontaneousStartWorkdayWindow(now time.Time) (spontaneousActivityWindow, error) {
	window := serverSpontaneousActivityWindow(now)
	if err := validateTimetableWorkday(window.date); err != nil {
		return spontaneousActivityWindow{}, err
	}
	return window, nil
}

func clockTimeFromMinutes(minutes int) time.Time {
	return time.Date(2000, 1, 1, minutes/60, minutes%60, 0, 0, time.UTC)
}

func (rs *Resource) lockSpontaneousStartRoom(ctx context.Context, roomID int64) error {
	return rs.TimetableData.LockSpontaneousStartRoom(ctx, roomID)
}

func (rs *Resource) lockSpontaneousActivityName(ctx context.Context, name string) error {
	return rs.TimetableData.LockSpontaneousActivityName(ctx, name)
}

func (rs *Resource) lockSpontaneousActivityCategory(ctx context.Context) error {
	return rs.TimetableData.LockSpontaneousActivityCategory(ctx)
}

func (rs *Resource) operationsCapabilities(w http.ResponseWriter, r *http.Request) {
	common.Respond(w, r, http.StatusOK, map[string]any{
		"web_spontaneous_activities_enabled": rs.webSpontaneousActivitiesEnabled(r),
	}, "Timetable operation capabilities retrieved")
}

func (rs *Resource) webSpontaneousActivitiesEnabled(r *http.Request) bool {
	logger := rs.Logger
	if logger == nil {
		logger = slog.Default()
	}
	careConcept := configSvc.ResolveStringOrDefault(
		r.Context(),
		rs.SettingsService,
		configModel.KeyCareConcept,
		configModel.CareConceptOpenRooms,
		logger,
	)
	if careConcept != configModel.CareConceptOpenRooms {
		return false
	}
	return configSvc.ResolveBoolOrDefault(
		r.Context(),
		rs.SettingsService,
		configModel.KeyWebSpontaneousActivities,
		true,
		logger,
	)
}

func (rs *Resource) operationsComplete(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		return rs.OperationsService.Complete(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID)
	}, "Timetable instance completed")
}

func (rs *Resource) operationsCheckInStudent(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.OperationsService.CheckInStudent(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Student checked in to timetable instance")
}

func (rs *Resource) operationsCheckOutStudent(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	instanceID, studentID, ok := parseOperationInstanceStudentIDs(w, r)
	if !ok {
		return
	}
	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.OperationsService.CheckOutStudent(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Student checked out from timetable instance")
}

func (rs *Resource) operationsPatchAttendance(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
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
	result, err := rs.OperationsService.PatchAttendance(r.Context(), int64(claims.ID), claims.IsAdmin, instanceID, studentID, patch)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable attendance updated")
}

func (rs *Resource) withOperationInstance(w http.ResponseWriter, r *http.Request, fn func(int64) (any, error), message string) {
	if rs.OperationsService == nil {
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
	case errors.Is(err, scheduleSvc.ErrSchulhofSupervisionRequired):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, schulhofSupervisionRequiredCode))
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
	case errors.Is(err, activeSvc.ErrStudentNotFound), errors.Is(err, activeSvc.ErrVisitNotFound),
		// A graduated (alumnus) student left on a roster is treated like an
		// unknown/absent student (404), matching the IoT check-in mapper (#405).
		errors.Is(err, activeSvc.ErrStudentGraduated):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, activeSvc.ErrInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
