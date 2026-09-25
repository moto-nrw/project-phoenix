package timetablehttp

import (
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
)

// Router returns a configured router for timetable endpoints
func (rs *Resource) Router() chi.Router {
	r := chi.NewRouter()
	r.Use(render.SetContentType(render.ContentTypeJSON))

	common.ProtectedTenantRoutes(r, func(r chi.Router, withTx common.Middleware) {
		rs.mountCalendarRoutes(r, withTx)

		// WP-B8: manual materialization. Admin-only — reuses SchedulesManage
		// as the rough "you can do anything with the schedule" permission.
		// The scheduler job runs the same service; this endpoint exists so
		// admins can re-run ad hoc without waiting for the weekly cadence.
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/materialize", rs.materialize)

		// WP-B9: instance lifecycle + re-plan-week. All four routes gated on
		// SchedulesManage. They share the tenant tx so start/complete/cancel
		// are atomic end-to-end (no dangling bridge rows on rollback).
		r.Route("/instances", func(r chi.Router) {
			rs.mountInstancePlanningRoutes(r, withTx)
			rs.mountInstanceStaffingRoutes(r, withTx)
			rs.mountInstanceAttendanceRoutes(r, withTx)
		})

		rs.mountPlanningReadRoutes(r, withTx)
		rs.mountPlanningCoordinationRoutes(r, withTx)
		rs.mountExportRoutes(r, withTx)
		rs.mountOperationRoutes(r, withTx)
		rs.mountTemplateRoutes(r, withTx)
		rs.mountPlanningTrackRoutes(r, withTx)
	})

	return r
}

// mountCalendarRoutes registers the calendar periods and closing days.
func (rs *Resource) mountCalendarRoutes(r chi.Router, withTx common.Middleware) {
	r.Route("/periods", func(r chi.Router) {
		// Shift-series creation is available to time-tracking managers and
		// requires a period ID, so the read-only list must be available on
		// that reduced Dienstplan path as well.
		r.With(common.RequiresAnyPermission(permissions.SchedulesRead, permissions.TimeTrackingManage), withTx).Get("/", rs.listPeriods)
		r.With(common.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/", rs.createPeriod)
		// WP-B1: idempotent school-year bootstrap. Same permission and tx
		// middleware as POST /periods — it is just a specialized create.
		r.With(common.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/bootstrap", rs.bootstrapPeriods)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).Get("/{id}", rs.getPeriod)
		r.With(common.RequiresPermission(permissions.SchedulesUpdate), withTx).Put("/{id}", rs.updatePeriod)
		r.With(common.RequiresPermission(permissions.SchedulesDelete), withTx).Delete("/{id}", rs.deletePeriod)
	})

	// OGS-Schließtage (#1418 3b): closure ranges maintained on the same
	// admin page as the calendar periods, hence the same permissions.
	r.Route("/closing-days", func(r chi.Router) {
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).Get("/", rs.listClosingDays)
		r.With(common.RequiresPermission(permissions.SchedulesCreate), withTx).Post("/", rs.createClosingDay)
		r.With(common.RequiresPermission(permissions.SchedulesUpdate), withTx).Put("/{id}", rs.updateClosingDay)
		r.With(common.RequiresPermission(permissions.SchedulesDelete), withTx).Delete("/{id}", rs.deleteClosingDay)
	})
}

// mountInstancePlanningRoutes registers the planner's reads and writes of
// single instances.
func (rs *Resource) mountInstancePlanningRoutes(r chi.Router, withTx common.Middleware) {
	// WP-F2 backend prerequisite: list instances in a date window for
	// the admin weekly planner. Read-only, gated on SchedulesRead so
	// office staff with view-only permissions can browse the plan
	// without being able to mutate it.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/", rs.listInstances)
	// #1875: probe which planned occurrences were individually edited
	// in a window, so the planner can warn before a series re-plan
	// discards them. Read-only, SchedulesRead like the list endpoint.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/edited-in-window", rs.editedInWindow)
	// Spontaneous (and template-bound out-of-cycle) create. Returns
	// the same enriched shape as the list endpoint so the frontend
	// can splice the fresh row into its SWR cache.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/", rs.createInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Put("/{id}", rs.updateInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/{id}/convert-to-series", rs.convertInstanceToSeries)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Delete("/{id}", rs.deleteInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/re-plan-week", rs.replanWeek)
	// #3594: cancel and remove the planned occurrences of a range
	// (a closure entered after planning); dry_run counts them.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/bulk-cancel", rs.bulkCancelInstances)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/{id}/start", rs.startInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
		Post("/{id}/complete", rs.completeInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
		Post("/{id}/reopen", rs.reopenInstance)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx, rs.requireWebAttendanceForActiveInstance).
		Post("/{id}/cancel", rs.cancelInstance)
	// #2601: preview for "Eltern informieren" in the cancel dialog.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Get("/{id}/guardian-notice", rs.guardianNoticeReach)
}

// mountInstanceStaffingRoutes registers the Vertretungsplan and staff pool
// routes of single instances.
func (rs *Resource) mountInstanceStaffingRoutes(r chi.Router, withTx common.Middleware) {
	// #1840 Vertretungsplan: mark a block as deliberately left
	// unstaffed so it drops out of the gap list. SchedulesManage
	// like the other instance mutations.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/{id}/acknowledge-understaffed", rs.acknowledgeUnderstaffed)
	// #1840 Vertretungsplan: apply an entire slide-over save
	// (absences, substitute, ack, cancel) atomically in one tenant
	// tx so a mid-save failure never commits partial state.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/{id}/deviations", rs.applyDeviations)
	// #1884 Personalpool: who is available / already planned in this
	// block's window (read), and the atomic move of one person onto
	// this block — from another block or from the free pool (write).
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/{id}/staff-pool", rs.getStaffPool)
	// #2283 Leseansicht: per-instance participant names for
	// schedules:read holders, CanReadStudent-filtered — the narrow
	// replacement for the users:read-gated tenant roster.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/{id}/participants", rs.getInstanceParticipants)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/{id}/move-staff", rs.moveStaff)
}

// mountInstanceAttendanceRoutes registers the attendance of single children
// in an instance.
func (rs *Resource) mountInstanceAttendanceRoutes(r chi.Router, withTx common.Middleware) {
	// WP-B10: three-field attendance PATCH. Gated on SchedulesManage
	// like the lifecycle routes. Path params are {instance_id} and
	// {student_id} — distinct from the {id} param above so they live
	// in a sibling route subtree.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
		Patch("/{instance_id}/students/{student_id}", rs.patchInstanceStudent)

	// #2898: correcting a COMPLETED block. Deliberately a separate
	// route from the PATCH above, which stays frozen after completion.
	// Same SchedulesManage gate, but the correction demands a reason
	// and writes audit.attendance_corrections.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
		Post("/{instance_id}/students/{student_id}/correction", rs.correctInstanceStudent)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Get("/{instance_id}/students/{student_id}/corrections", rs.getInstanceStudentCorrections)
}

// mountPlanningReadRoutes registers the per-student reads, the gap list,
// the Sammel-Vertretung, the Änderungsprotokoll and the conflict probes.
func (rs *Resource) mountPlanningReadRoutes(r chi.Router, withTx common.Middleware) {
	// WP-B11: per-student day and week read endpoints. Both gated on
	// SchedulesRead; handler-level auth (securityruntime.CanReadStudent) adds
	// the per-student check.
	r.Route("/student/{id}", func(r chi.Router) {
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/day", rs.getStudentDay)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/week", rs.getStudentWeek)
	})

	// WP-B12: gap detection + one-click substitute. Gaps is SchedulesRead
	// (information view), substitute is SchedulesManage (mutation).
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/gaps", rs.getGaps)

	// #2284 Sammel-Vertretung: one atomic save applying a day-wide absence
	// or substitution for one person across several selected days.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/substitutions/bulk", rs.applyBulkSubstitution)

	// Änderungsprotokoll (#1886): append-only deviation history, read-only.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/deviations/history", rs.getDeviationHistory)
	// POST /substitute wurde konsolidiert (#1886): der einzige Caller
	// (Betreuungsplan-Gap-Fill) nutzt jetzt POST /instances/{id}/deviations.

	// WP-B13: exception-conflict warnings (planning-only, read-only).
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/exception-conflicts", rs.getExceptionConflicts)

	// Planning-time conflict probe for a hypothetical slot (read-only,
	// advisory). Same permission + tx middleware as /exception-conflicts.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/conflicts", rs.getPlannedConflicts)
}

// mountPlanningCoordinationRoutes registers the pickup extensions, the
// conflict acknowledgements and the shift-coverage probe.
func (rs *Resource) mountPlanningCoordinationRoutes(r chi.Router, withTx common.Middleware) {
	// Later pickup times without a block (#3261). Resolving changes
	// rosters, so both routes need SchedulesManage.
	r.Route("/pickup-extensions", func(r chi.Router) {
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
			Get("/", rs.listPickupExtensions)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
			Post("/{id}/resolve", rs.resolvePickupExtension)
	})

	// Per-user conflict acknowledgements (#2139). SchedulesRead on
	// purpose: whoever sees the banner may manage their own view state;
	// writes only touch the calling account's rows.
	r.Route("/conflict-acks", func(r chi.Router) {
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/", rs.listConflictAcks)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Put("/{fingerprint}", rs.acknowledgeConflict)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Delete("/{fingerprint}", rs.unacknowledgeConflict)
	})

	// Shift coverage exposes Dienstplan availability, so it requires both
	// timetable read access and shift-management access. It is separate from
	// /conflicts, which remains available to schedules:read-only users.
	r.With(common.RequiresAllPermissions(
		permissions.SchedulesRead,
		permissions.TimeTrackingManage,
		permissions.UsersRead,
	), withTx).Post("/shift-coverage", rs.checkShiftCoverage)
}

// mountExportRoutes registers the slot lists and the printable
// Betreuungsplan.
func (rs *Resource) mountExportRoutes(r chi.Router, withTx common.Middleware) {
	// Issue #1565: lists from planned care slots (Plan / Ist / Abgleich).
	// Read-only, but they expose named children + presence, so they require
	// student-data read on top of schedule read (GDPR) — same bar as the
	// emergency snapshot export (UsersRead).
	r.Route("/lists", func(r chi.Router) {
		r.With(common.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
			Post("/options", rs.listSlotListOptions)
		r.With(common.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
			Post("/preview", rs.previewSlotList)
		r.With(common.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
			Post("/export", rs.exportSlotList)
	})

	// Printable Betreuungsplan week (#2079). Staff names are on the sheet,
	// so it needs users:read on top of the schedules:read the plan itself
	// requires — the same pair the slot lists use. exportBetreuungsplan
	// additionally gates the sensitive internal variant by its request body.
	r.Route("/betreuungsplan", func(r chi.Router) {
		r.With(common.RequiresAllPermissions(permissions.SchedulesRead, permissions.UsersRead), withTx).
			Post("/export", rs.exportBetreuungsplan)
	})
}

// mountOperationRoutes registers the operational day of the supervisors.
func (rs *Resource) mountOperationRoutes(r chi.Router, withTx common.Middleware) {
	r.Route("/operations", func(r chi.Router) {
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/capabilities", rs.operationsCapabilities)
		// Roster endpoints stay available to operational supervisors; their
		// pickup-time fields are redacted without users:read.
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/planned-now", rs.operationsPlannedNow)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/active-sessions", rs.operationsActiveSessions)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/instances/{id}/roster", rs.operationsRoster)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Get("/active-groups/{id}/roster", rs.operationsRosterByActiveGroup)
		// Operational mutations are available to normal supervisors with
		// SchedulesRead; the service enforces assignment/admin access via
		// requireCanOperate before touching schedule or active state.
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Post("/spontaneous/start", rs.operationsCreateAndStartSpontaneous)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
			Post("/instances/{id}/start", rs.operationsStart)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
			Post("/instances/{id}/complete", rs.operationsComplete)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
			Post("/instances/{id}/reopen", rs.operationsReopen)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
			Post("/instances/{id}/students/{student_id}/check-in", rs.operationsCheckInStudent)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
			Post("/instances/{id}/students/{student_id}/check-out", rs.operationsCheckOutStudent)
		r.With(common.RequiresPermission(permissions.SchedulesRead), withTx, common.RequireWebAttendanceEnabled(rs.SettingsService)).
			Patch("/instances/{id}/students/{student_id}/attendance", rs.operationsPatchAttendance)
	})
}

// mountTemplateRoutes registers the recurring templates and their
// offering-source editor support.
func (rs *Resource) mountTemplateRoutes(r chi.Router, withTx common.Middleware) {
	// Templates — admin shortcut to add a recurring activity (Mensa,
	// Lernzeit, AGs) without leaving the planner. Bundles timeframe +
	// activities.groups + activities.schedules into one HTTP call and
	// optionally materializes the visible week so the new instances
	// appear immediately on the grid.
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/templates", rs.listTemplates)
	// Offering-source editor support (#2137): Angebots-Auswahl mit
	// Jahrgangs-Zählern für den Regeltermin-Editor.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Get("/offering-sources", rs.listOfferingSources)
	// Deduplizierte Kinderzahl über eine Auswahl mehrerer Angebote
	// (Mehrfach-Quelle): exakte Vorschau für den Regeltermin-Editor.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Get("/offering-sources/combined-counts", rs.getCombinedOfferingSourceCounts)
	r.With(common.RequiresPermission(permissions.SchedulesRead), withTx).
		Get("/templates/{id}", rs.getTemplate)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/templates", rs.createTemplate)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Put("/templates/{id}", rs.updateTemplate)
	// WP-B3: "Dieser und alle folgenden" — cap the old template at the
	// effective date and continue with a successor template.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/templates/{id}/split", rs.splitTemplate)
	// "Dieser und alle folgenden löschen" — cap the template at the
	// effective date without creating a successor and remove planned
	// future instances.
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Post("/templates/{id}/end", rs.endTemplate)
	r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).
		Delete("/templates/{id}", rs.archiveTemplate)
}

// mountPlanningTrackRoutes registers the planning tracks of the planner.
func (rs *Resource) mountPlanningTrackRoutes(r chi.Router, withTx common.Middleware) {
	r.Route("/planning-tracks", func(r chi.Router) {
		r.With(common.RequiresAnyPermission(permissions.SchedulesRead, permissions.SchedulesManage), withTx).Get("/", rs.listPlanningTracks)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).Post("/", rs.createPlanningTrack)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).Put("/order", rs.reorderPlanningTracks)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).Put("/{id}", rs.updatePlanningTrack)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).Delete("/{id}", rs.archivePlanningTrack)
		r.With(common.RequiresPermission(permissions.SchedulesManage), withTx).Post("/{id}/restore", rs.restorePlanningTrack)
	})
}
