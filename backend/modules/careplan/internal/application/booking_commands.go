package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MaterializeApprovedBookings writes one roster row per booked offering whose
// care offering points at an activity group, or whose offering a template
// sources. Offerings without either are skipped. It leaves out the
// multi-source union resync: the approval runs BEFORE the child's status
// flips to approved, so the union — which resolves children by their approved
// status — cannot see the child yet; the decision runs
// ResyncMultiSourceTemplatesForChild after the flip.
func (m *bookingCommands) MaterializeApprovedBookings(ctx context.Context, requestChildID, studentID int64, phase careplan.OfferingPhase) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.MaterializeApprovedBookings(txCtx, requestChildID, studentID, phase)
		return err
	})
	return
}

// ResyncMultiSourceTemplatesForChild re-reconciles every multi-source
// template fed by one of the child's offerings. Called AFTER the child's
// status flipped to approved: the union resync resolves children through
// their approved status, so the resync inside the approval's materialization
// pass cannot see the child yet.
//
// Mirrors the approval's careOfferingsEnabled gate: with the setting off the
// approval wrote no enrollment rows, so this post-flip pass must not start
// materializing either. It also takes the tenant recurrence lock itself,
// because with the setting off the locking materialization pass never ran in
// this transaction.
func (m *bookingCommands) ResyncMultiSourceTemplatesForChild(ctx context.Context, requestChildID int64, phase careplan.OfferingPhase) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ResyncMultiSourceTemplatesForChild(txCtx, requestChildID, phase)
		return err
	})
	return
}

// AdjustOfferings performs a booking switch of an approved child and records
// it. The source tells the two entry points apart: a direct correction by the
// office is subject to the live care-offerings setting and shows up in the
// central history as its own row kind (#2436), while a request-applied change
// carries the capability frozen when the parent submitted and is already
// visible there as the decided request.
func (m *bookingCommands) AdjustOfferings(ctx context.Context, in careplan.OfferingAdjustment) (value0 *careplan.OfferingAdjustmentResult, err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = m.BookingMaterialization.AdjustOfferings(txCtx, in)
		return err
	})
	return
}

// ReconcileOfferingPickupForStudents refreshes the remaining derived effects.
// It deliberately writes no schedule.student_pickup_schedules rows: regular
// offering pickup times are a date-aware projection of booking validity.
func (m *bookingCommands) ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ReconcileOfferingPickupForStudents(txCtx, studentIDs)
		return err
	})
	return
}

// ReconcileOfferingPickupForOffering refreshes every current or future child
// affected by an offering edit. The catalog calls it after an update.
func (m *bookingCommands) ReconcileOfferingPickupForOffering(ctx context.Context, offeringID int64) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ReconcileOfferingPickupForOffering(txCtx, offeringID)
		return err
	})
	return
}

// ResetStudentPickupDayToOffering removes the stored row for one weekday. A
// staff row is the manual override; an old care_offering row is legacy
// materialization. The row is removed only when the read-time projection has
// an offering time for the requested date.
func (m *bookingCommands) ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, date calendar.Date) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ResetStudentPickupDayToOffering(txCtx, studentID, date)
		return err
	})
	return
}

// CombinedOfferingSourceCounts validates the selection exactly like a save
// would (existence, active, one shared phase, phase-within-period) and then
// counts distinct approved children across the offerings. Dedup key is the
// request child — within one phase a child holds one request-child identity,
// which is also the provenance tag the union resync seeds rows under.
func (m *bookingCommands) CombinedOfferingSourceCounts(ctx context.Context, offeringIDs []int64, calendarPeriodID *int64) (value0 *careplan.OfferingSourceCombinedCounts, err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = m.BookingMaterialization.CombinedOfferingSourceCounts(txCtx, offeringIDs, calendarPeriodID)
		return err
	})
	return
}

// ListOfferingSourceOptions returns the offerings an admin may pick as a
// Regeltermin source. With a calendar period given, only offerings whose
// phase's service window lies within that period qualify — a source outside
// the Planungszeitraum could never materialize a single occurrence.
func (m *bookingCommands) ListOfferingSourceOptions(ctx context.Context, calendarPeriodID *int64) (value0 []careplan.OfferingSourceOption, err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = m.BookingMaterialization.ListOfferingSourceOptions(txCtx, calendarPeriodID)
		return err
	})
	return
}

// ResyncOfferingSourcedTemplates re-reconciles EVERY offering-sourced
// template of the tenant against its offering's approved children. Grade
// transitions call it (apply AND revert) after rewriting school classes:
// promotions move children between Jahrgänge, so a template with a grade
// filter must lose the children that left its filter and gain the ones that
// entered it — and graduations turn children into alumni, whose rows are
// capped. History before effectiveFrom stays untouched.
//
// A template whose source has drifted invalid (offering deleted mid-flight,
// phase outside the period) is skipped with a warning, mirroring the decision
// fan-out: one broken template must not block a whole grade transition.
// The caller must hold the tenant recurrence lock.
func (m *bookingCommands) ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ResyncOfferingSourcedTemplates(txCtx, effectiveFrom)
		return err
	})
	return
}

// ResyncTemplatesSourcedFromOffering re-reconciles every template sourcing
// ONE offering. Care-offering updates and phase service-window updates call
// it (#2147 review): editing the offering's days or phase changes the wanted
// roster of every template fed by it, and without this resync the sourced
// rows and already-materialized occurrences would keep the pre-edit shape
// until an unrelated template save. A template whose source the pending edit
// makes incompatible with its planning period surfaces as the Timetable's
// offering-source refusal — the caller must reject the edit then (#2147
// review round 7): swallowing it would commit a change whose sourced rows and
// materialized occurrences can never be resynced again. The caller must hold
// the tenant recurrence lock.
func (m *bookingCommands) ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.ResyncTemplatesSourcedFromOffering(txCtx, offeringID, effectiveFrom)
		return err
	})
	return
}

// DetachTemplatesSourcedFromOffering removes ONE offering from the source set
// of every template sourcing it. Templates left with other sources are
// resynced against the remaining set — the diff-based reconcile retires the
// departing offering's contribution and keeps (or re-shapes) the rest. A
// template whose LAST source departs runs the cleanup-only resync: all
// offering-derived enrollment rows are deleted (not yet effective) or capped
// at effectiveFrom (already started), legacy-owned rows stay protected, and
// the affected students' already-materialized future occurrences are
// reconciled. Finally the template's source columns themselves are rewritten:
// since the jsonb id array carries no FK, there is no ON DELETE backstop —
// this call is the ONLY thing keeping the array free of deleted ids. The
// care-offering and phase delete flows call it BEFORE the row delete (#2147
// review round 11), while the provenance tags still exist. A remaining
// source that has itself drifted invalid keeps the template's remaining
// sources and skips only the roster resync instead of blocking the delete.
// The caller must hold the tenant recurrence lock.
func (m *bookingCommands) DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) (err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		err = m.BookingMaterialization.DetachTemplatesSourcedFromOffering(txCtx, offeringID, effectiveFrom)
		return err
	})
	return
}

// TemplateRosterMaintenanceFeeds resolves, for every requested template, the
// source offerings, the offerings linked to any segment of its split series,
// and the care-offerings setting (#3140). It costs a constant number of reads
// regardless of how many templates are asked for.
func (m *bookingCommands) TemplateRosterMaintenanceFeeds(ctx context.Context, templates []careplan.TemplateRosterFeedQuery) (value0 map[int64]careplan.TemplateRosterFeeds, err error) {
	err = m.BookingMaterialization.deps.RunInTx(ctx, func(txCtx context.Context) error {
		value0, err = m.BookingMaterialization.TemplateRosterMaintenanceFeeds(txCtx, templates)
		return err
	})
	return
}

type bookingCommands struct{ *BookingMaterialization }

func NewBookingCommands(records *BookingMaterialization) careplan.BookingMaterializationCapability {
	return &bookingCommands{records}
}
