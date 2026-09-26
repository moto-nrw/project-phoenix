package timetablehttp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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

// operationActor resolves the caller of an operations call.
//
// The isAdmin half is forced to false for assignment-bound portals (#2527):
// an account can be an OGS admin AND a Lehrkraft, and the school portal must
// answer the same for both. This is belt-and-braces — the operational-overview
// gate already collapses to "own" for a school token — but it keeps the second
// path (the explicit isAdmin argument) from ever becoming the way in.
func operationActor(ctx context.Context) (accountID int64, isAdmin bool) {
	claims := jwt.ClaimsFromCtx(ctx)
	if common.IsAssignmentBoundPortal(ctx) {
		return int64(claims.ID), false
	}
	return int64(claims.ID), claims.IsAdmin
}

// operationsActiveSessions lists today's running instances with their plan
// windows so the supervision UI can label session tabs (#2265).
func (rs *Resource) operationsActiveSessions(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	result, err := rs.OperationsService.ActiveSessions(r.Context(), rs.todayDate())
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"sessions": result}, "Active timetable sessions retrieved")
}

func (rs *Resource) operationsPlannedNow(w http.ResponseWriter, r *http.Request) {
	if rs.OperationsService == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations service not wired")))
		return
	}
	opts, ok := parsePlannedNowOptions(w, r)
	if !ok {
		return
	}
	today := rs.todayDate()
	date := today
	if raw := r.URL.Query().Get("date"); raw != "" {
		parsed, err := calendar.ParseDate(raw)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid date")))
			return
		}
		if opts.Scope == timetable.PlannedNowScopePast && parsed != today {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("past scope only supports today's date")))
			return
		}
		date = parsed
	}
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.PlannedNow(r.Context(), accountID, isAdmin, date, calendar.Now(), opts)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	if opts.IncludeRoster && !canViewOperationPickupTimes(r.Context()) {
		redactOperationPlannedPickupTimes(result)
	}
	common.Respond(w, r, http.StatusOK, map[string]any{"instances": result}, "Planned timetable instances retrieved")
}

func parsePlannedNowOptions(w http.ResponseWriter, r *http.Request) (timetable.PlannedNowOptions, bool) {
	query := r.URL.Query()
	var opts timetable.PlannedNowOptions
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
	if raw := query.Get("scope"); raw != "" {
		if raw != timetable.PlannedNowScopePast && raw != timetable.PlannedNowScopeDay {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid scope")))
			return opts, false
		}
		opts.Scope = raw
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
		accountID, isAdmin := operationActor(r.Context())
		roster, err := rs.OperationsService.Roster(r.Context(), accountID, isAdmin, instanceID)
		if err == nil && !canViewOperationPickupTimes(r.Context()) {
			redactOperationRosterPickupTimes(roster)
		}
		return roster, err
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
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.RosterByActiveGroup(r.Context(), accountID, isAdmin, activeGroupID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	if !canViewOperationPickupTimes(r.Context()) {
		redactOperationRosterPickupTimes(result)
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable roster retrieved")
}

func (rs *Resource) operationsStart(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		accountID, isAdmin := operationActor(r.Context())
		result, err := rs.OperationsService.Start(r.Context(), accountID, isAdmin, instanceID)
		if err != nil {
			return nil, err
		}
		return startOperationResponse{
			InstanceID:    result.InstanceID,
			Status:        result.Status,
			ActiveGroupID: result.ActiveGroupID,
			Warnings:      result.Warnings,
		}, nil
	}, "Timetable instance started")
}

func (rs *Resource) operationsReopen(w http.ResponseWriter, r *http.Request) {
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		claims := jwt.ClaimsFromCtx(r.Context())
		result, err := rs.OperationsService.Reopen(r.Context(), int64(claims.ID), common.HasEffectiveAdminScope(r.Context()), instanceID)
		if err != nil {
			return nil, err
		}
		return startOperationResponse{InstanceID: result.InstanceID, Status: result.Status, ActiveGroupID: result.ActiveGroupID, Warnings: result.Warnings}, nil
	}, "Timetable instance reopened")
}

func (rs *Resource) operationsCreateAndStartSpontaneous(w http.ResponseWriter, r *http.Request) {
	req, currentStaffID, ok := rs.admitSpontaneousStart(w, r)
	if !ok {
		return
	}

	req.StaffIDs = appendUniquePositive(req.StaffIDs, currentStaffID)
	createdBy := currentStaffID
	// Room and caller validation can span a Berlin day boundary. Capture the
	// authoritative start window immediately before the first write-capable
	// step so a request that crosses into a weekend cannot mutate anything.
	window, err := spontaneousStartWorkdayWindow(rs.Now())
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}
	activityGroupID, err := rs.TimetableData.ResolveSpontaneousActivity(r.Context(), req.Title, req.ActivityGroupID, createdBy)
	if err != nil {
		renderSpontaneousActivityResolutionError(w, r, err)
		return
	}
	// Activity resolution can create metadata and therefore cross a Berlin day
	// boundary. Recheck immediately before creating the activity instance.
	window, err = spontaneousStartWorkdayWindow(rs.Now())
	if err != nil {
		tenant.MarkRollback(r.Context())
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	claims := jwt.ClaimsFromCtx(r.Context())
	result, err := rs.OperationsService.CreateAndStartSpontaneous(r.Context(), int64(claims.ID), claims.IsAdmin, timetable.SpontaneousStart{
		Date:             window.date,
		StartTime:        window.startTime,
		EndTime:          window.endTime,
		Title:            req.Title,
		Description:      req.Description,
		Notes:            req.Notes,
		RoomID:           req.RoomID,
		ActivityGroupID:  activityGroupID,
		StaffIDs:         req.StaffIDs,
		CreatedByStaffID: &createdBy,
	})
	if err != nil {
		rs.renderSpontaneousStartError(w, r, err)
		return
	}
	common.Respond(w, r, http.StatusCreated, startOperationResponse{
		InstanceID:    result.InstanceID,
		Status:        result.Status,
		ActiveGroupID: result.ActiveGroupID,
		Warnings:      result.Warnings,
	}, "Spontaneous timetable instance created and started")
}

// admitSpontaneousStart runs the checks before any write of a spontaneous
// start: wiring, the school's feature switch, the request, the workday, the
// room and the caller's staff identity. It returns ok=false after rendering
// the refusal.
func (rs *Resource) admitSpontaneousStart(w http.ResponseWriter, r *http.Request) (*spontaneousStartRequest, int64, bool) {
	if rs.OperationsService == nil || rs.TimetableData == nil {
		common.RenderError(w, r, common.ErrorInternalServer(errors.New("timetable operations resource not fully wired")))
		return nil, 0, false
	}
	if !rs.webSpontaneousActivitiesEnabled(r) {
		common.RenderError(w, r, common.ErrorForbidden(timetable.ErrTimetableOperationForbidden))
		return nil, 0, false
	}
	req, ok := bindSpontaneousStartRequest(w, r)
	if !ok {
		return nil, 0, false
	}
	if _, err := spontaneousStartWorkdayWindow(rs.Now()); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return nil, 0, false
	}
	if len(req.StudentIDs) > 0 {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("student_ids are not accepted for spontaneous operational starts")))
		return nil, 0, false
	}
	if !rs.validateSpontaneousRoom(w, r, req.RoomID) {
		return nil, 0, false
	}

	currentStaffID := rs.resolveStartedByStaffID(r.Context())
	if currentStaffID <= 0 {
		common.RenderError(w, r, common.ErrorForbidden(timetable.ErrTimetableOperationForbidden))
		return nil, 0, false
	}
	return req, currentStaffID, true
}

// renderSpontaneousStartError maps a failed create+start. The composition
// owns its own rollback (both phases share the request tx). A Create-phase
// failure is wrapped so it keeps the create-specific error mapping; a
// Start-phase failure uses the operations mapping.
func (rs *Resource) renderSpontaneousStartError(w http.ResponseWriter, r *http.Request, err error) {
	var createErr *timetable.SpontaneousCreateError
	if errors.As(err, &createErr) {
		renderCreateInstanceError(w, r, createErr.Err)
		return
	}
	rs.renderOperationsError(w, r, err)
}

// validateSpontaneousRoom checks the target room exists and is currently
// unoccupied — taking the spontaneous-start room lock in between so the
// existence-vs-conflict check is serialized. Renders the appropriate error and
// returns false on any failure.
func (rs *Resource) validateSpontaneousRoom(w http.ResponseWriter, r *http.Request, roomID int64) bool {
	exists, err := rs.TimetableData.SpontaneousRoomExists(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("load spontaneous room failed", err))
		return false
	}
	if !exists {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("room not found")))
		return false
	}
	if err := rs.TimetableData.LockSpontaneousStartRoom(r.Context(), roomID); err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("lock spontaneous start room failed", err))
		return false
	}
	hasRoomConflict, err := rs.TimetableData.SpontaneousRoomOccupied(r.Context(), roomID)
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap("check room conflict failed", err))
		return false
	}
	if hasRoomConflict {
		common.RenderError(w, r, common.ErrorConflict(studentpresence.ErrRoomConflict))
		return false
	}
	return true
}

type spontaneousActivityWindow struct {
	date      calendar.Date
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

func serverSpontaneousActivityWindow(now time.Time) spontaneousActivityWindow {
	now = now.In(calendar.Berlin)
	currentMinutes := now.Hour()*60 + now.Minute()
	startMinutes := min(currentMinutes, 23*60+30)
	endMinutes := min(startMinutes+60, 23*60+59)
	return spontaneousActivityWindow{
		date:      calendar.DateFromTime(now),
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
	careConcept := settings.ResolveStringOrDefault(
		r.Context(),
		rs.SettingsService,
		settings.KeyCareConcept,
		settings.CareConceptOpenRooms,
		logger,
	)
	if careConcept != settings.CareConceptOpenRooms {
		return false
	}
	return settings.ResolveBoolOrDefault(
		r.Context(),
		rs.SettingsService,
		settings.KeyWebSpontaneousActivities,
		true,
		logger,
	)
}

func (rs *Resource) operationsComplete(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ConfirmedPresentStudentIDs []int64 `json:"confirmed_present_student_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("confirmed_present_student_ids is required")))
		return
	}
	r = r.WithContext(timetable.WithCompletionConfirmation(r.Context(), body.ConfirmedPresentStudentIDs))
	rs.withOperationInstance(w, r, func(instanceID int64) (any, error) {
		accountID, isAdmin := operationActor(r.Context())
		return rs.OperationsService.Complete(r.Context(), accountID, isAdmin, instanceID)
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
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.CheckInStudent(r.Context(), accountID, isAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	if !canViewOperationPickupTimes(r.Context()) {
		redactOperationRosterPickupTimes(result)
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
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.CheckOutStudent(r.Context(), accountID, isAdmin, instanceID, studentID)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	if !canViewOperationPickupTimes(r.Context()) {
		redactOperationRosterPickupTimes(result)
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
	accountID, isAdmin := operationActor(r.Context())
	result, err := rs.OperationsService.PatchAttendance(r.Context(), accountID, isAdmin, instanceID, studentID, patch)
	if err != nil {
		rs.renderOperationsError(w, r, err)
		return
	}
	if !canViewOperationPickupTimes(r.Context()) {
		result.PickupTime = nil
	}
	common.Respond(w, r, http.StatusOK, result, "Timetable attendance updated")
}

func canViewOperationPickupTimes(ctx context.Context) bool {
	return common.IsAssignmentBoundPortal(ctx) ||
		securityruntime.HasPermission(permissions.UsersRead, jwt.PermissionsFromCtx(ctx))
}

func redactOperationRosterPickupTimes(roster *timetable.OperationRoster) {
	if roster == nil {
		return
	}
	roster.PickupTimesLoaded = false
	roster.PickupTimesRedacted = true
	for i := range roster.Rows {
		roster.Rows[i].PickupTime = nil
	}
}

func redactOperationPlannedPickupTimes(instances []timetable.OperationPlannedInstance) {
	for i := range instances {
		instances[i].PickupTimesLoaded = false
		instances[i].PickupTimesRedacted = true
		for j := range instances[i].RosterPreview {
			instances[i].RosterPreview[j].PickupTime = nil
		}
	}
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
	InstanceID    int64                               `json:"instance_id"`
	Status        string                              `json:"status"`
	ActiveGroupID int64                               `json:"active_group_id"`
	Warnings      []timetable.InstanceConflictWarning `json:"warnings"`
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

func renderSpontaneousActivityResolutionError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, timetable.ErrSpontaneousCategoryArchived) {
		common.RenderError(w, r, common.ErrorConflict(err))
		return
	}
	common.RenderError(w, r, common.ErrorInternalServerWrap("resolve spontaneous activity group failed", err))
}

func (rs *Resource) renderOperationsError(w http.ResponseWriter, r *http.Request, err error) {
	var validationErr *timetable.AttendanceValidationError
	switch {
	case errors.As(err, &validationErr):
		renderValidationErrors(w, r, attendancePatchFieldErrors(validationErr.Fields))
	case errors.Is(err, timetable.ErrTimetableOperationForbidden):
		common.RenderError(w, r, common.ErrorForbidden(err))
	case errors.Is(err, timetable.ErrTimetableOperationNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, timetable.ErrTimetableOperationConflict), errors.Is(err, timetable.ErrInvalidInstanceTransition),
		errors.Is(err, timetable.ErrInstanceStartTooEarly), errors.Is(err, timetable.ErrInstanceStartExpired),
		errors.Is(err, timetable.ErrInstanceCompleteEarly):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, timetable.ErrInstanceWeekend):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	case errors.Is(err, timetable.ErrCompletionConfirmationStale):
		common.RenderError(w, r, common.ErrorConflictWithCode(err, "completion_confirmation_stale"))
	case errors.Is(err, timetable.ErrInstanceNotFound):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, studentpresence.ErrRoomCapacityExceeded):
		// A full room names itself with code and numbers (#3633).
		common.RenderError(w, r, common.ErrorBusinessRejectionOr(studentpresence.RoomCapacityCode)(err))
	case errors.Is(err, studentpresence.ErrStudentAlreadyActive), errors.Is(err, studentpresence.ErrRoomConflict),
		errors.Is(err, studentpresence.ErrStudentsNotPresent), errors.Is(err, studentpresence.ErrGroupAlreadyEnded):
		common.RenderError(w, r, common.ErrorConflict(err))
	case errors.Is(err, studentpresence.ErrStudentNotFound), errors.Is(err, studentpresence.ErrVisitNotFound),
		// A graduated (alumnus) student left on a roster is treated like an
		// unknown/absent student (404), matching the IoT check-in mapper (#405).
		errors.Is(err, studentpresence.ErrStudentGraduated), errors.Is(err, studentpresence.ErrStudentCareEnded):
		common.RenderError(w, r, common.ErrorNotFound(err))
	case errors.Is(err, studentpresence.ErrInvalidData):
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
	default:
		common.RenderError(w, r, common.ErrorInternalServer(err))
	}
}
