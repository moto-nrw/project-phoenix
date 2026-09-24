package timetablehttp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

const dateLayout = "2006-01-02"

const calendarPeriodRosterDeleteConflictMessage = "Kalenderzeitraum kann nicht gelöscht werden: " +
	"Durch das Entfernen der Verknüpfungen würden doppelte aktive Kinder- oder Personalzuordnungen entstehen."

const calendarPeriodCareOfferingConflictCode = "calendar_period_care_offering_conflict"

const calendarPeriodOverlapConflictCode = "calendar_period_overlap_conflict"

const (
	calendarPeriodsLoadErrorMessage      = "Kalenderzeiträume konnten nicht geladen werden"
	calendarPeriodUsageErrorMessage      = "Verwendung der Kalenderzeiträume konnte nicht geladen werden"
	calendarPeriodCreateErrorMessage     = "Kalenderzeitraum konnte nicht erstellt werden"
	calendarPeriodsBootstrapErrorMessage = "Kalenderzeiträume konnten nicht initialisiert werden"
)

var errCalendarPeriodCareOfferingConflict = errors.New(
	"der Kalenderzeitraum wird von einem verknüpften Betreuungsangebot benötigt; bitte zuerst das Angebot oder den Regeltermin anpassen",
)

var errCalendarPeriodOverlapConflict = errors.New(
	"es besteht bereits ein aktiver Zeitraum desselben Typs im gewählten Zeitraum; bitte Datum, Typ oder Aktiv-Status anpassen",
)

// calendarPeriodOverlapRenderer builds the 409 payload for the hard same-type
// overlap rule. When the calendar exposes the conflicting periods, the message
// names the first one (mirroring the advisory warning wording) and the
// details carry all IDs/names; a bare sentinel keeps the static text.
func calendarPeriodOverlapRenderer(err error) render.Renderer {
	var overlapErr *schoolcalendar.CalendarPeriodOverlapError
	if !errors.As(err, &overlapErr) || len(overlapErr.Overlaps) == 0 {
		return common.ErrorConflictWithCode(errCalendarPeriodOverlapConflict, calendarPeriodOverlapConflictCode)
	}
	first := overlapErr.Overlaps[0]
	ids := make([]int64, 0, len(overlapErr.Overlaps))
	names := make([]string, 0, len(overlapErr.Overlaps))
	for _, o := range overlapErr.Overlaps {
		ids = append(ids, o.ID)
		names = append(names, o.Name)
	}
	message := fmt.Errorf(
		"es besteht bereits ein aktiver Zeitraum desselben Typs: „%s“ (%s – %s); bitte Datum, Typ oder Aktiv-Status anpassen",
		first.Name,
		germanCalendarDate(first.StartDate),
		germanCalendarDate(first.EndDate),
	)
	return common.ErrorConflictWithDetails(message, calendarPeriodOverlapConflictCode, map[string]any{
		"overlapping_period_ids":   ids,
		"overlapping_period_names": names,
	})
}

// Resource defines the timetable API resource.
//
// Optional services may be nil in tests that only exercise a subset of routes
// — the dependent handler will return 500 instead of panicking. Production
// wiring must populate every field.
type Resource struct {
	Dependencies
}

// Dependencies bundles everything NewResource needs. Using a struct instead
// of a positional signature keeps tests — which frequently only populate a
// subset — readable and lets us add future deps without churning every call
// site.
type Dependencies struct {
	// CalendarPeriods and ClosingDays are the School Calendar owner; the
	// period usage counts come from the planning owners (#3124).
	CalendarPeriods         CalendarPeriods
	ClosingDays             ClosingDays
	CalendarPeriodUsage     CalendarPeriodUsage
	MaterializationService  timetable.MaterializationCapability
	InstanceService         timetable.InstanceLifecycleCapability
	InstanceSeriesConverter timetable.InstanceSeriesConversion
	OperationsService       timetable.OperationCapability
	// People serves display names and whether a child still attends from
	// the People Directory (#2732).
	People People
	// Templates are the Timetable owner's template writes including split
	// and end (#3424 slice S2); RecurrenceLock is its tenant recurrence gate.
	// AttendanceCorrections corrects completed blocks and Deviations applies
	// the Vertretungsplan saves (slice S3). TimetableData serves the planner's
	// reads from the Timetable owner (#3551).
	Templates             timetable.TemplateAdministration
	RecurrenceLock        timetable.RecurrenceWriteLock
	AttendanceCorrections timetable.AttendanceCorrections
	Deviations            timetable.StaffDeviations
	TimetableData         timetable.TimetableDataCapability
	// ConflictDetection serves the conflict warnings, the staff pool and the
	// shift-coverage probe from the Timetable owner (#3550).
	ConflictDetection  timetable.ConflictDetectionCapability
	CareDayService     careplan.CareDayQuery
	UserContextService securityruntime.StudentAccessUserContext
	SettingsService    settings.Resolver
	SlotListsService   classday.SlotLists
	// OfferingSourceOptions serves the offering-source editor support
	// endpoint, the empty-roster explanation of the instance list and the
	// roster indicator of the template list (#2137, #3140).
	OfferingSourceOptions timetable.OfferingSourceSupport
	// SupervisionSheets serves the per-child supervision sheet of the school
	// portal (#2527). Only SchoolSupervisionRouter consumes it.
	SupervisionSheets SupervisionSheets
	// PlanExportService renders the printable Betreuungsplan week (#2079).
	PlanExportService planexport.Service
	PlanningTracks    timetable.PlanningTrackAdministration
	// PickupExtensions serves the open block decisions for later pickup
	// times (#3261).
	PickupExtensions timetable.PickupExtensionCapability
	// Staffing wakes the tenant's open staff pages after a staffing save
	// commits (#1844); nil sends nothing.
	Staffing StaffingAnnouncer
	Logger   *slog.Logger
	Now      func() time.Time
}

// NewResource creates a new timetable resource from the given Dependencies.
// Nil deps are tolerated at construction time; the dependent handler returns
// 500 at request time if one of its deps is unset.
func NewResource(deps Dependencies) *Resource {
	if deps.Now == nil {
		deps.Now = calendar.Now
	}
	return &Resource{Dependencies: deps}
}

func (rs *Resource) todayDate() calendar.Date {
	return calendar.DateFromTime(rs.Now())
}

// Request / Response types

// CalendarPeriodRequest represents a create/update request for a calendar period
type CalendarPeriodRequest struct {
	Name            string  `json:"name"`
	PeriodType      string  `json:"period_type"`
	StartDate       string  `json:"start_date"`
	EndDate         string  `json:"end_date"`
	WeekCycleLength int     `json:"week_cycle_length"`
	WeekCycleAnchor *string `json:"week_cycle_anchor,omitempty"`
	IsActive        bool    `json:"is_active"`
}

// Bind validates the request
func (req *CalendarPeriodRequest) Bind(_ *http.Request) error {
	if req.Name == "" {
		return errors.New("name is required")
	}
	if len(req.Name) > 255 {
		return errors.New("name cannot exceed 255 characters")
	}
	if req.PeriodType == "" {
		return errors.New("period_type is required")
	}
	if !schoolcalendar.IsValidPeriodType(req.PeriodType) {
		return errors.New("invalid period_type, must be one of: school_year, semester, holiday, custom")
	}
	if req.StartDate == "" {
		return errors.New("start_date is required")
	}
	if req.EndDate == "" {
		return errors.New("end_date is required")
	}
	if req.WeekCycleLength <= 0 {
		req.WeekCycleLength = 1
	}
	return nil
}

// PeriodWarning is an advisory, never-blocking notice attached to a calendar
// period response (WP-B2, QA #1577 H3). One entry is emitted per overlapping
// active period.
type PeriodWarning struct {
	Code                   string   `json:"code"`
	Message                string   `json:"message"`
	OverlappingPeriodIDs   []int64  `json:"overlapping_period_ids"`
	OverlappingPeriodNames []string `json:"overlapping_period_names"`
}

// warningCodeOverlappingActivePeriods is the machine-readable code the
// frontend matches on; the German message is display-only.
const warningCodeOverlappingActivePeriods = "overlapping_active_periods"

// germanDateLayout renders calendar dates as dd.mm.yyyy for user-facing text.
const germanDateLayout = "02.01.2006"

// CalendarPeriodResponse represents a calendar period in API responses
type CalendarPeriodResponse struct {
	ID              int64           `json:"id"`
	Name            string          `json:"name"`
	PeriodType      string          `json:"period_type"`
	StartDate       string          `json:"start_date"`
	EndDate         string          `json:"end_date"`
	WeekCycleLength int             `json:"week_cycle_length"`
	WeekCycleAnchor *string         `json:"week_cycle_anchor,omitempty"`
	IsActive        bool            `json:"is_active"`
	CreatedAt       string          `json:"created_at"`
	UpdatedAt       string          `json:"updated_at"`
	Warnings        []PeriodWarning `json:"warnings,omitempty"`

	// Reference counts returned by list, detail, and bootstrap reads. The
	// frontend uses these for the "Verwendung" column and delete-impact warning.
	EnrollmentPhaseCount   int `json:"enrollment_phase_count"`
	ActivityGroupCount     int `json:"activity_group_count"`
	ScheduleCount          int `json:"schedule_count"`
	StudentEnrollmentCount int `json:"student_enrollment_count"`
	SupervisorCount        int `json:"supervisor_count"`
	ActivityInstanceCount  int `json:"activity_instance_count"`
}

func mapPeriodToResponse(p schoolcalendar.CalendarPeriod) CalendarPeriodResponse {
	resp := CalendarPeriodResponse{
		ID:              p.ID,
		Name:            p.Name,
		PeriodType:      p.PeriodType,
		StartDate:       p.StartDate,
		EndDate:         p.EndDate,
		WeekCycleLength: p.WeekCycleLength,
		IsActive:        p.IsActive,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
	}
	if p.WeekCycleAnchor != "" {
		anchor := p.WeekCycleAnchor
		resp.WeekCycleAnchor = &anchor
	}
	return resp
}

// parseDates extracts start_date, end_date, and optional week_cycle_anchor from a request.
// Returns parsed calendar dates and true on success, or renders an error and returns false.
func parseDates(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest) (startDate, endDate calendar.Date, anchor *calendar.Date, ok bool) {
	var err error
	startDate, err = calendar.ParseDate(req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return calendar.Date(""), calendar.Date(""), nil, false
	}

	endDate, err = calendar.ParseDate(req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return calendar.Date(""), calendar.Date(""), nil, false
	}

	if req.WeekCycleAnchor != nil {
		a, err := calendar.ParseDate(*req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return calendar.Date(""), calendar.Date(""), nil, false
		}
		anchor = &a
	}

	return startDate, endDate, anchor, true
}

// periodFieldsFromRequest renders the validated request as the calendar's
// writable period fields.
func periodFieldsFromRequest(req *CalendarPeriodRequest, startDate, endDate calendar.Date, anchor *calendar.Date) schoolcalendar.CalendarPeriodFields {
	fields := schoolcalendar.CalendarPeriodFields{
		Name:            req.Name,
		PeriodType:      req.PeriodType,
		StartDate:       startDate.String(),
		EndDate:         endDate.String(),
		WeekCycleLength: req.WeekCycleLength,
		IsActive:        req.IsActive,
	}
	if anchor != nil {
		fields.WeekCycleAnchor = anchor.String()
	}
	return fields
}

// validatePeriodRules checks business rules after dates have been parsed.
// Returns true on success, or renders an error and returns false.
func validatePeriodRules(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest, startDate, endDate calendar.Date, anchor *calendar.Date) bool {
	if !endDate.After(startDate) {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("end_date must be after start_date")))
		return false
	}
	if req.WeekCycleLength > 1 && anchor == nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("week_cycle_anchor is required when week_cycle_length > 1")))
		return false
	}
	return true
}

// attachOverlapWarnings decorates resp with one advisory warning per active
// period overlapping the just-saved period (WP-B2, QA #1577 H3). The check
// runs after a successful save and must never turn the 2xx into an error:
// lookup failures are logged at Warn and the response simply ships without
// warnings.
func (rs *Resource) attachOverlapWarnings(ctx context.Context, period schoolcalendar.CalendarPeriod, resp *CalendarPeriodResponse) {
	overlaps, err := rs.CalendarPeriods.ListActiveOverlaps(ctx, period)
	if err != nil {
		rs.getLogger().Warn("calendar period overlap check failed, omitting warnings",
			slog.Int64("period_id", period.ID),
			slog.String("error", err.Error()),
		)
		return
	}
	for _, o := range overlaps {
		resp.Warnings = append(resp.Warnings, PeriodWarning{
			Code: warningCodeOverlappingActivePeriods,
			Message: fmt.Sprintf(
				"Überschneidet sich mit aktivem Zeitraum „%s“ (%s – %s). Termine werden im Zweifel dem älteren Zeitraum zugeordnet.",
				o.Name,
				germanCalendarDate(o.StartDate),
				germanCalendarDate(o.EndDate),
			),
			OverlappingPeriodIDs:   []int64{o.ID},
			OverlappingPeriodNames: []string{o.Name},
		})
	}
}

// Handlers

// periodUsageCounts loads the reference counts that drive deletion impact.
// A failure is not advisory: rendering zeroes would falsely describe a used
// period as unused and can lead an administrator into a destructive action.
func (rs *Resource) periodUsageCounts(ctx context.Context) (map[int64]CalendarPeriodUsageCounts, error) {
	if rs.CalendarPeriodUsage == nil {
		return nil, errors.New("calendar period usage not wired")
	}
	return rs.CalendarPeriodUsage.UsageCounts(ctx)
}

func applyPeriodUsage(resp *CalendarPeriodResponse, usage map[int64]CalendarPeriodUsageCounts) {
	if u, ok := usage[resp.ID]; ok {
		resp.EnrollmentPhaseCount = u.EnrollmentPhases
		resp.ActivityGroupCount = u.ActivityGroups
		resp.ScheduleCount = u.Schedules
		resp.StudentEnrollmentCount = u.StudentEnrollments
		resp.SupervisorCount = u.Supervisors
		resp.ActivityInstanceCount = u.ActivityInstances
	}
}

func (rs *Resource) listPeriods(w http.ResponseWriter, r *http.Request) {
	periods, err := rs.CalendarPeriods.ListCalendarPeriods(r.Context(), schoolcalendar.CalendarPeriodFilter{})
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodsLoadErrorMessage, err))
		return
	}

	usage, err := rs.periodUsageCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodUsageErrorMessage, err))
		return
	}
	responses := make([]CalendarPeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = mapPeriodToResponse(p)
		applyPeriodUsage(&responses[i], usage)
	}

	common.Respond(w, r, http.StatusOK, responses, "Calendar periods retrieved successfully")
}

func (rs *Resource) getPeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	period, err := rs.CalendarPeriods.FindCalendarPeriod(r.Context(), id)
	if err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht geladen werden", err))
		}
		return
	}

	resp := mapPeriodToResponse(period)
	usage, err := rs.periodUsageCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodUsageErrorMessage, err))
		return
	}
	applyPeriodUsage(&resp, usage)
	common.Respond(w, r, http.StatusOK, resp, "Calendar period retrieved successfully")
}

func (rs *Resource) createPeriod(w http.ResponseWriter, r *http.Request) {
	req := &CalendarPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, anchor, ok := parseDates(w, r, req)
	if !ok {
		return
	}

	if !validatePeriodRules(w, r, req, startDate, endDate, anchor) {
		return
	}

	period, err := rs.CalendarPeriods.AddCalendarPeriod(r.Context(), schoolcalendar.CreateCalendarPeriod{
		CalendarPeriodFields: periodFieldsFromRequest(req, startDate, endDate, anchor),
	})
	if err != nil {
		switch {
		case errors.Is(err, schoolcalendar.ErrCalendarPeriodNameConflict):
			common.RenderError(w, r, common.ErrorConflict(schoolcalendar.ErrCalendarPeriodNameConflict))
		case errors.Is(err, schoolcalendar.ErrCalendarPeriodOverlapConflict):
			common.RenderError(w, r, calendarPeriodOverlapRenderer(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodCreateErrorMessage, err))
		}
		return
	}

	resp := mapPeriodToResponse(period)
	rs.attachOverlapWarnings(r.Context(), period, &resp)
	common.Respond(w, r, http.StatusCreated, resp, "Calendar period created successfully")
}

// bootstrapPeriodsResponse is the wire shape of POST /periods/bootstrap.
type bootstrapPeriodsResponse struct {
	Periods []CalendarPeriodResponse `json:"periods"`
	Created bool                     `json:"created"`
}

// bootstrapPeriods handles POST /api/timetable/periods/bootstrap (WP-B1).
// It ensures the tenant has at least one calendar period, creating the
// current school year when none exists. Idempotent by design: the response
// is always 200 with the tenant's periods plus a created flag — repeated
// calls and concurrent calls never yield a 409.
func (rs *Resource) bootstrapPeriods(w http.ResponseWriter, r *http.Request) {
	periods, created, err := rs.CalendarPeriods.EnsureDefaultSchoolYear(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodsBootstrapErrorMessage, err))
		return
	}
	usage, err := rs.periodUsageCounts(r.Context())
	if err != nil {
		common.RenderError(w, r, common.ErrorInternalServerWrap(calendarPeriodUsageErrorMessage, err))
		return
	}

	responses := make([]CalendarPeriodResponse, len(periods))
	for i, p := range periods {
		responses[i] = mapPeriodToResponse(p)
		applyPeriodUsage(&responses[i], usage)
	}

	common.Respond(w, r, http.StatusOK, bootstrapPeriodsResponse{
		Periods: responses,
		Created: created,
	}, "Calendar periods bootstrapped successfully")
}

func (rs *Resource) updatePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	existing, err := rs.CalendarPeriods.FindCalendarPeriod(r.Context(), id)
	if err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht geladen werden", err))
		}
		return
	}

	req := &CalendarPeriodRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(err))
		return
	}

	startDate, endDate, anchor, ok := parseDates(w, r, req)
	if !ok {
		return
	}

	if !validatePeriodRules(w, r, req, startDate, endDate, anchor) {
		return
	}

	updated, err := rs.CalendarPeriods.ChangeCalendarPeriod(r.Context(), schoolcalendar.UpdateCalendarPeriod{
		ID: existing.ID, CalendarPeriodFields: periodFieldsFromRequest(req, startDate, endDate, anchor),
	})
	if err != nil {
		switch {
		case errors.Is(err, schoolcalendar.ErrCalendarPeriodNameConflict):
			common.RenderError(w, r, common.ErrorConflict(schoolcalendar.ErrCalendarPeriodNameConflict))
		case errors.Is(err, schoolcalendar.ErrCalendarPeriodRequiredByCareOffering):
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, common.ErrorConflictWithCode(
				errCalendarPeriodCareOfferingConflict,
				calendarPeriodCareOfferingConflictCode,
			))
		case errors.Is(err, schoolcalendar.ErrCalendarPeriodOverlapConflict):
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, calendarPeriodOverlapRenderer(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht aktualisiert werden", err))
		}
		return
	}

	resp := mapPeriodToResponse(updated)
	rs.attachOverlapWarnings(r.Context(), updated, &resp)
	common.Respond(w, r, http.StatusOK, resp, "Calendar period updated successfully")
}

func (rs *Resource) deletePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	if _, err := rs.CalendarPeriods.FindCalendarPeriod(r.Context(), id); err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodNotFound) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht geladen werden", err))
		}
		return
	}

	if err := rs.CalendarPeriods.RemoveCalendarPeriod(r.Context(), id); err != nil {
		if errors.Is(err, schoolcalendar.ErrCalendarPeriodRequiredByCareOffering) {
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, common.ErrorConflictWithCode(
				errCalendarPeriodCareOfferingConflict,
				calendarPeriodCareOfferingConflictCode,
			))
			return
		}
		if isCalendarPeriodRosterDeleteConflict(err) {
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, common.ErrorConflictMessage(calendarPeriodRosterDeleteConflictMessage))
			return
		}
		common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht gelöscht werden", err))
		return
	}

	common.Respond(w, r, http.StatusOK, nil, "Calendar period deleted successfully")
}

func isCalendarPeriodRosterDeleteConflict(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, schoolcalendar.ErrCalendarPeriodRosterConflict) {
		return true
	}

	msg := err.Error()
	return strings.Contains(msg, "duplicate key value violates unique constraint \"idx_student_enrollments_active\"") ||
		strings.Contains(msg, "duplicate key value violates unique constraint \"idx_supervisors_active\"")
}

// defaultChildrenPerStaffRatio mirrors the registry default for
// timetable.children_per_staff_ratio (services/config/defaults/timetable.go)
// and is the fallback used when the settings service is unwired (tests) or a
// tenant override lookup fails.
const defaultChildrenPerStaffRatio = 12

// childrenPerStaffRatio resolves the Betreuungsschlüssel setting once per
// request so per-instance/per-template capacity math (timetable.RequiredStaffForChildren)
// can apply a single resolved value instead of re-querying the settings
// service for every row. Takes a context rather than *http.Request so it can
// be called from handlers and from context-only helpers alike (e.g.
// loadTemplates, which is shared by list/update/exists call sites).
func (rs *Resource) childrenPerStaffRatio(ctx context.Context) int {
	return settings.ResolveIntOrDefault(
		ctx,
		rs.SettingsService,
		settings.KeyTimetableChildrenPerStaffRatio,
		defaultChildrenPerStaffRatio,
		rs.getLogger(),
	)
}

func (rs *Resource) enforcePlannedEnd(ctx context.Context) (bool, error) {
	if rs.SettingsService == nil {
		// Registry default for timetable.enforce_planned_end. Tests and
		// partial facades may leave settings unwired; a real lookup failure
		// below must still surface.
		return true, nil
	}
	enforce, err := rs.SettingsService.ResolveBool(ctx, settings.KeyTimetableEnforcePlannedEnd)
	if err != nil {
		return false, fmt.Errorf("%w: resolve planned end policy: %v", timetable.ErrLifecycleSettings, err)
	}
	return enforce, nil
}
