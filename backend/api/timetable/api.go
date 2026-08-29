package timetable

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	enrollmentSvc "github.com/moto-nrw/project-phoenix/services/enrollment"
	"github.com/moto-nrw/project-phoenix/services/planexport"
	scheduleSvc "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/services/slotlists"
	usercontextSvc "github.com/moto-nrw/project-phoenix/services/usercontext"
	userSvc "github.com/moto-nrw/project-phoenix/services/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
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
// overlap rule. When the service exposes the conflicting periods, the message
// names the first one (mirroring the advisory warning wording) and the
// details carry all IDs/names; a bare sentinel keeps the static text.
func calendarPeriodOverlapRenderer(err error) render.Renderer {
	var overlapErr *schedule.CalendarPeriodOverlapError
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
		first.StartDate.Format(germanDateLayout),
		first.EndDate.Format(germanDateLayout),
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
	CalendarPeriodService   scheduleSvc.CalendarPeriodService
	ClosingDayService       scheduleSvc.ClosingDayService
	MaterializationService  scheduleSvc.MaterializationService
	InstanceService         scheduleSvc.InstanceService
	InstanceSeriesConverter scheduleSvc.InstanceSeriesConverter
	OperationsService       scheduleSvc.TimetableOperationsService
	TemplateSplitService    *scheduleSvc.TemplateSplitService
	PersonService           userSvc.PersonService
	TimetableData           *scheduleSvc.TimetableDataService
	CareDayService          scheduleSvc.CareDayService
	UserContextService      usercontextSvc.UserContextService
	SettingsService         configSvc.SettingsService
	SlotListsService        slotlists.Service
	// OfferingSourceOptions serves the offering-source editor support
	// endpoint (#2137); implemented by the enrollment decision service.
	OfferingSourceOptions enrollmentSvc.OfferingSourceOptionLister
	// ReportService serves the per-child supervision sheet of the school
	// portal (#2527). Only SchoolSupervisionRouter consumes it.
	ReportService enrollmentSvc.ReportService
	// PlanExportService renders the printable Betreuungsplan week (#2079).
	PlanExportService    planexport.Service
	PlanningTrackService scheduleSvc.PlanningTrackService
	Broadcaster          realtime.Broadcaster
	Logger               *slog.Logger
	DB                   *bun.DB
	Now                  func() time.Time
}

// NewResource creates a new timetable resource from the given Dependencies.
// Nil deps are tolerated at construction time; the dependent handler returns
// 500 at request time if one of its deps is unset.
func NewResource(deps Dependencies) *Resource {
	if deps.Now == nil {
		deps.Now = timezone.Now
	}
	return &Resource{Dependencies: deps}
}

// Router returns a configured router for timetable endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantGroup(r, rs.DB, func(r chi.Router, withTx common.Middleware) {

		r.Route("/periods", func(r chi.Router) {
			// Shift-series creation is available to time-tracking managers and
			// requires a period ID, so the read-only list must be available on
			// that reduced Dienstplan path as well.
			r.With(authorize.RequiresAnyPermission(permissions.SchedulesRead, permissions.TimeTrackingManage), withTx).Get("/", rs.listPeriods)
			r.With(authorize.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/", rs.createPeriod)
			// WP-B1: idempotent school-year bootstrap. Same permission and tx
			// middleware as POST /periods — it is just a specialized create.
			r.With(authorize.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/bootstrap", rs.bootstrapPeriods)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).Get("/{id}", rs.getPeriod)
			r.With(authorize.RequiresPermission(permissions.SchedulesUpdate), withTx).Put("/{id}", rs.updatePeriod)
			r.With(authorize.RequiresPermission(permissions.SchedulesDelete), withTx).Delete("/{id}", rs.deletePeriod)
		})

		// OGS-Schließtage (#1418 3b): closure ranges maintained on the same
		// admin page as the calendar periods, hence the same permissions.
		r.Route("/closing-days", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).Get("/", rs.listClosingDays)
			r.With(authorize.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/", rs.createClosingDay)
			r.With(authorize.RequiresPermission(permissions.SchedulesUpdate), withTx).Put("/{id}", rs.updateClosingDay)
			r.With(authorize.RequiresPermission(permissions.SchedulesDelete), withTx).Delete("/{id}", rs.deleteClosingDay)
		})

		// WP-B8: manual materialization. Admin-only — reuses SchedulesManage
		// as the rough "you can do anything with the schedule" permission.
		// The scheduler job runs the same service; this endpoint exists so
		// admins can re-run ad hoc without waiting for the weekly cadence.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/materialize", rs.materialize)

		// WP-B9: instance lifecycle + re-plan-week. All four routes gated on
		// SchedulesManage. They share the tenant tx so start/complete/cancel
		// are atomic end-to-end (no dangling bridge rows on rollback).
		r.Route("/instances", func(r chi.Router) {
			// WP-F2 backend prerequisite: list instances in a date window for
			// the admin weekly planner. Read-only, gated on SchedulesRead so
			// office staff with view-only permissions can browse the plan
			// without being able to mutate it.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/", rs.listInstances)
			// #1875: probe which planned occurrences were individually edited
			// in a window, so the planner can warn before a series re-plan
			// discards them. Read-only, SchedulesRead like the list endpoint.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/edited-in-window", rs.editedInWindow)
			// Spontaneous (and template-bound out-of-cycle) create. Returns
			// the same enriched shape as the list endpoint so the frontend
			// can splice the fresh row into its SWR cache.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/", rs.createInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Put("/{id}", rs.updateInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/convert-to-series", rs.convertInstanceToSeries)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Delete("/{id}", rs.deleteInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/re-plan-week", rs.replanWeek)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/start", rs.startInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/{id}/complete", rs.completeInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/{id}/reopen", rs.reopenInstance)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx, rs.requireWebAttendanceForActiveInstance).
				Post("/{id}/cancel", rs.cancelInstance)
			// #2601: preview for "Eltern informieren" in the cancel dialog.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Get("/{id}/guardian-notice", rs.guardianNoticeReach)
				// #1840 Vertretungsplan: mark a block as deliberately left
				// unstaffed so it drops out of the gap list. SchedulesManage
				// like the other instance mutations.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)
				// #1840 Vertretungsplan: apply an entire slide-over save
				// (absences, substitute, ack, cancel) atomically in one tenant
				// tx so a mid-save failure never commits partial state.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/deviations", rs.applyDeviations)
				// #1884 Personalpool: who is available / already planned in this
				// block's window (read), and the atomic move of one person onto
				// this block — from another block or from the free pool (write).
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/{id}/staff-pool", rs.getStaffPool)
			// #2283 Leseansicht: per-instance participant names for
			// schedules:read holders, CanReadStudent-filtered — the narrow
			// replacement for the users:read-gated tenant roster.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/{id}/participants", rs.getInstanceParticipants)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
				Post("/{id}/move-staff", rs.moveStaff)

			// WP-B10: three-field attendance PATCH. Gated on SchedulesManage
			// like the lifecycle routes. Path params are {instance_id} and
			// {student_id} — distinct from the {id} param above so they live
			// in a sibling route subtree.
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Patch("/{instance_id}/students/{student_id}", rs.patchInstanceStudent)
		})

		// WP-B11: per-student day and week read endpoints. Both gated on
		// SchedulesRead; handler-level auth (authorize.CanReadStudent) adds
		// the per-student check.
		r.Route("/student/{id}", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/day", rs.getStudentDay)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/week", rs.getStudentWeek)
		})

		// WP-B12: gap detection + one-click substitute. Gaps is SchedulesRead
		// (information view), substitute is SchedulesManage (mutation).
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/gaps", rs.getGaps)

		// #2284 Sammel-Vertretung: one atomic save applying a day-wide absence
		// or substitution for one person across several selected days.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/substitutions/bulk", rs.applyBulkSubstitution)

		// Änderungsprotokoll (#1886): append-only deviation history, read-only.
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/deviations/history", rs.getDeviationHistory)
		// POST /substitute wurde konsolidiert (#1886): der einzige Caller
		// (Betreuungsplan-Gap-Fill) nutzt jetzt POST /instances/{id}/deviations.

		// WP-B13: exception-conflict warnings (planning-only, read-only).
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/exception-conflicts", rs.getExceptionConflicts)

		// Planning-time conflict probe for a hypothetical slot (read-only,
		// advisory). Same permission + tx middleware as /exception-conflicts.
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/conflicts", rs.getPlannedConflicts)

		// Per-user conflict acknowledgements (#2139). SchedulesRead on
		// purpose: whoever sees the banner may manage their own view state;
		// writes only touch the calling account's rows.
		r.Route("/conflict-acks", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/", rs.listConflictAcks)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Put("/{fingerprint}", rs.acknowledgeConflict)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Delete("/{fingerprint}", rs.unacknowledgeConflict)
		})

		// Shift coverage exposes Dienstplan availability, so it requires both
		// timetable read access and shift-management access. It is separate from
		// /conflicts, which remains available to schedules:read-only users.
		r.With(authorize.RequiresAllPermissions(
			permissions.SchedulesRead,
			permissions.TimeTrackingManage,
			permissions.UsersRead,
		), withTx).Post("/shift-coverage", rs.checkShiftCoverage)

		// Issue #1565: lists from planned care slots (Plan / Ist / Abgleich).
		// Read-only, but they expose named children + presence, so they require
		// student-data read on top of schedule read (GDPR) — same bar as the
		// emergency snapshot export (UsersRead).
		r.Route("/lists", func(r chi.Router) {
			r.With(authorize.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
				Post("/options", rs.listSlotListOptions)
			r.With(authorize.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
				Post("/preview", rs.previewSlotList)
			r.With(authorize.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
				Post("/export", rs.exportSlotList)
		})

		// Printable Betreuungsplan week (#2079). Staff names are on the sheet,
		// so it needs users:read on top of the schedules:read the plan itself
		// requires — the same pair the slot lists use. exportBetreuungsplan
		// additionally gates the sensitive internal variant by its request body.
		r.Route("/betreuungsplan", func(r chi.Router) {
			r.With(authorize.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
				Post("/export", rs.exportBetreuungsplan)
		})

		r.Route("/operations", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/capabilities", rs.operationsCapabilities)
			// Roster endpoints stay available to operational supervisors; their
			// pickup-time fields are redacted without users:read.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/planned-now", rs.operationsPlannedNow)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/active-sessions", rs.operationsActiveSessions)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/instances/{id}/roster", rs.operationsRoster)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Get("/active-groups/{id}/roster", rs.operationsRosterByActiveGroup)
			// Operational mutations are available to normal supervisors with
			// SchedulesRead; the service enforces assignment/admin access via
			// requireCanOperate before touching schedule or active state.
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/spontaneous/start", rs.operationsCreateAndStartSpontaneous)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
				Post("/instances/{id}/start", rs.operationsStart)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/instances/{id}/complete", rs.operationsComplete)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/instances/{id}/reopen", rs.operationsReopen)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/instances/{id}/students/{student_id}/check-in", rs.operationsCheckInStudent)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Post("/instances/{id}/students/{student_id}/check-out", rs.operationsCheckOutStudent)
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
				Patch("/instances/{id}/students/{student_id}/attendance", rs.operationsPatchAttendance)
		})

		// Templates — admin shortcut to add a recurring activity (Mensa,
		// Lernzeit, AGs) without leaving the planner. Bundles timeframe +
		// activities.groups + activities.schedules into one HTTP call and
		// optionally materializes the visible week so the new instances
		// appear immediately on the grid.
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/templates", rs.listTemplates)
		// Offering-source editor support (#2137): Angebots-Auswahl mit
		// Jahrgangs-Zählern für den Regeltermin-Editor.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Get("/offering-sources", rs.listOfferingSources)
		// Deduplizierte Kinderzahl über eine Auswahl mehrerer Angebote
		// (Mehrfach-Quelle): exakte Vorschau für den Regeltermin-Editor.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Get("/offering-sources/combined-counts", rs.getCombinedOfferingSourceCounts)
		r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/templates/{id}", rs.getTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates", rs.createTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Put("/templates/{id}", rs.updateTemplate)
		// WP-B3: "Dieser und alle folgenden" — cap the old template at the
		// effective date and continue with a successor template.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates/{id}/split", rs.splitTemplate)
		// "Dieser und alle folgenden löschen" — cap the template at the
		// effective date without creating a successor and remove planned
		// future instances.
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/templates/{id}/end", rs.endTemplate)
		r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).
			Delete("/templates/{id}", rs.archiveTemplate)

		r.Route("/planning-tracks", func(r chi.Router) {
			r.With(authorize.RequiresPermission(permissions.SchedulesRead), withTx).Get("/", rs.listPlanningTracks)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).Post("/", rs.createPlanningTrack)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).Put("/order", rs.reorderPlanningTracks)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).Put("/{id}", rs.updatePlanningTrack)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).Delete("/{id}", rs.archivePlanningTrack)
			r.With(authorize.RequiresPermission(permissions.SchedulesManage), withTx).Post("/{id}/restore", rs.restorePlanningTrack)
		})
	})

	return r
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
	if !schedule.IsValidPeriodType(req.PeriodType) {
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

func mapPeriodToResponse(p *schedule.CalendarPeriod) CalendarPeriodResponse {
	resp := CalendarPeriodResponse{
		ID:              p.ID,
		Name:            p.Name,
		PeriodType:      p.PeriodType,
		StartDate:       p.StartDate.String(),
		EndDate:         p.EndDate.String(),
		WeekCycleLength: p.WeekCycleLength,
		IsActive:        p.IsActive,
		CreatedAt:       p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.Format(time.RFC3339),
	}
	if p.WeekCycleAnchor != nil {
		anchor := p.WeekCycleAnchor.String()
		resp.WeekCycleAnchor = &anchor
	}
	return resp
}

// parseDates extracts start_date, end_date, and optional week_cycle_anchor from a request.
// Returns parsed calendar dates and true on success, or renders an error and returns false.
func parseDates(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest) (startDate, endDate timezone.Date, anchor *timezone.Date, ok bool) {
	var err error
	startDate, err = timezone.ParseDate(req.StartDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid start_date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, nil, false
	}

	endDate, err = timezone.ParseDate(req.EndDate)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid end_date format, expected YYYY-MM-DD")))
		return timezone.Date{}, timezone.Date{}, nil, false
	}

	if req.WeekCycleAnchor != nil {
		a, err := timezone.ParseDate(*req.WeekCycleAnchor)
		if err != nil {
			common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid week_cycle_anchor format, expected YYYY-MM-DD")))
			return timezone.Date{}, timezone.Date{}, nil, false
		}
		anchor = &a
	}

	return startDate, endDate, anchor, true
}

// validatePeriodRules checks business rules after dates have been parsed.
// Returns true on success, or renders an error and returns false.
func validatePeriodRules(w http.ResponseWriter, r *http.Request, req *CalendarPeriodRequest, startDate, endDate timezone.Date, anchor *timezone.Date) bool {
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
func (rs *Resource) attachOverlapWarnings(ctx context.Context, period *schedule.CalendarPeriod, resp *CalendarPeriodResponse) {
	overlaps, err := rs.CalendarPeriodService.FindActiveOverlaps(ctx, period)
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
				o.StartDate.Format(germanDateLayout),
				o.EndDate.Format(germanDateLayout),
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
func (rs *Resource) periodUsageCounts(ctx context.Context) (map[int64]schedule.CalendarPeriodUsage, error) {
	return rs.CalendarPeriodService.GetUsageCounts(ctx)
}

func applyPeriodUsage(resp *CalendarPeriodResponse, usage map[int64]schedule.CalendarPeriodUsage) {
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
	periods, err := rs.CalendarPeriodService.GetAllPeriods(r.Context())
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

	period, err := rs.CalendarPeriodService.GetPeriodByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	period := &schedule.CalendarPeriod{
		Name:            req.Name,
		PeriodType:      req.PeriodType,
		StartDate:       startDate,
		EndDate:         endDate,
		WeekCycleLength: req.WeekCycleLength,
		WeekCycleAnchor: anchor,
		IsActive:        req.IsActive,
	}

	if err := rs.CalendarPeriodService.CreatePeriod(r.Context(), period); err != nil {
		switch {
		case errors.Is(err, schedule.ErrCalendarPeriodNameConflict):
			common.RenderError(w, r, common.ErrorConflict(schedule.ErrCalendarPeriodNameConflict))
		case errors.Is(err, schedule.ErrCalendarPeriodOverlapConflict):
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
	periods, created, err := rs.CalendarPeriodService.EnsureDefaultSchoolYear(r.Context())
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

	existing, err := rs.CalendarPeriodService.GetPeriodByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
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

	existing.Name = req.Name
	existing.PeriodType = req.PeriodType
	existing.StartDate = startDate
	existing.EndDate = endDate
	existing.WeekCycleLength = req.WeekCycleLength
	existing.WeekCycleAnchor = anchor
	existing.IsActive = req.IsActive

	if err := rs.CalendarPeriodService.UpdatePeriod(r.Context(), existing); err != nil {
		switch {
		case errors.Is(err, schedule.ErrCalendarPeriodNameConflict):
			common.RenderError(w, r, common.ErrorConflict(schedule.ErrCalendarPeriodNameConflict))
		case errors.Is(err, schedule.ErrCalendarPeriodCareOfferingConflict):
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, common.ErrorConflictWithCode(
				errCalendarPeriodCareOfferingConflict,
				calendarPeriodCareOfferingConflictCode,
			))
		case errors.Is(err, schedule.ErrCalendarPeriodOverlapConflict):
			tenant.MarkRollback(r.Context())
			common.RenderError(w, r, calendarPeriodOverlapRenderer(err))
		default:
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht aktualisiert werden", err))
		}
		return
	}

	resp := mapPeriodToResponse(existing)
	rs.attachOverlapWarnings(r.Context(), existing, &resp)
	common.Respond(w, r, http.StatusOK, resp, "Calendar period updated successfully")
}

func (rs *Resource) deletePeriod(w http.ResponseWriter, r *http.Request) {
	id, err := common.ParseID(r)
	if err != nil {
		common.RenderError(w, r, common.ErrorInvalidRequest(errors.New("invalid period ID")))
		return
	}

	if _, err := rs.CalendarPeriodService.GetPeriodByID(r.Context(), id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			common.RenderError(w, r, common.ErrorNotFound(errors.New("calendar period not found")))
		} else {
			common.RenderError(w, r, common.ErrorInternalServerWrap("Kalenderzeitraum konnte nicht geladen werden", err))
		}
		return
	}

	if err := rs.CalendarPeriodService.DeletePeriod(r.Context(), id); err != nil {
		if errors.Is(err, schedule.ErrCalendarPeriodCareOfferingConflict) {
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

	if base.IsUniqueViolationOn(err, "idx_student_enrollments_active") ||
		base.IsUniqueViolationOn(err, "idx_supervisors_active") {
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
// request so per-instance/per-template capacity math (RequiredStaffForChildren)
// can apply a single resolved value instead of re-querying the settings
// service for every row. Takes a context rather than *http.Request so it can
// be called from handlers and from context-only helpers alike (e.g.
// loadTemplates, which is shared by list/update/exists call sites).
func (rs *Resource) childrenPerStaffRatio(ctx context.Context) int {
	return configSvc.ResolveIntOrDefault(
		ctx,
		rs.SettingsService,
		configModel.KeyTimetableChildrenPerStaffRatio,
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
	enforce, err := rs.SettingsService.ResolveBool(ctx, configModel.KeyTimetableEnforcePlannedEnd)
	if err != nil {
		return false, fmt.Errorf("%w: resolve planned end policy: %v", scheduleSvc.ErrLifecycleSettings, err)
	}
	return enforce, nil
}
