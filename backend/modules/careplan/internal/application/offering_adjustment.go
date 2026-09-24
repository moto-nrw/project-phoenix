package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// offeringAdjustmentWork carries one adjustment from its validation through
// the persisted rows.
type offeringAdjustmentWork struct {
	input                                  careplan.OfferingAdjustment
	reason                                 string
	child                                  ports.BookingChild
	student                                ports.BookingStudent
	phase                                  ports.BookingPhase
	effectiveFrom                          *calendar.Date
	selectionDate                          calendar.Date
	offeringByID                           map[int64]*careplan.CareOffering
	activeOfferingByID, beforeOfferingByID map[int64]*careplan.CareOffering
	beforeLinks                            []*careplan.BookedOffering
	beforeJSON, afterJSON                  []byte
	selections                             []careplan.OfferingSelection
	replacement                            []*careplan.BookedOffering
	overridden                             []careplan.OfferingOverride
	authoritative, afterHasCareDays        bool
	isCompleteWithdrawal                   bool
}

// AdjustOfferings performs a booking switch of an approved child and records
// it. The source tells the two entry points apart: a direct correction by the
// office is subject to the live care-offerings setting and shows up in the
// central history as its own row kind (#2436), while a request-applied change
// carries the capability frozen when the parent submitted and is already
// visible there as the decided request.
func (m *BookingMaterialization) AdjustOfferings(ctx context.Context, in careplan.OfferingAdjustment) (*careplan.OfferingAdjustmentResult, error) {
	work, err := m.prepareOfferingAdjustment(ctx, in)
	if err != nil {
		return nil, err
	}
	if err := m.persistOfferingAdjustment(ctx, work); err != nil {
		return nil, err
	}
	adjustmentID, err := m.recordOfferingAdjustment(ctx, work)
	if err != nil {
		return nil, err
	}
	if err := m.reconcileOfferingAdjustmentWithdrawal(ctx, work, adjustmentID); err != nil {
		return nil, err
	}
	if err := m.ReconcileOfferingPickupForStudents(ctx, []int64{*work.child.CreatedStudentID}); err != nil {
		return nil, fmt.Errorf("decision: reconcile offering pickup times: %w", err)
	}
	return &careplan.OfferingAdjustmentResult{
		RequestChildID: work.child.ID, Before: derefBookedOfferings(work.beforeLinks), Selections: work.selections,
		Offerings: derefOfferings(work.offeringByID), Overridden: work.overridden,
		CompleteWithdrawal: work.isCompleteWithdrawal,
	}, nil
}

func (m *BookingMaterialization) prepareOfferingAdjustment(ctx context.Context, in careplan.OfferingAdjustment) (*offeringAdjustmentWork, error) {
	reason, err := m.validateOfferingAdjustmentInput(ctx, in)
	if err != nil {
		return nil, err
	}
	work := &offeringAdjustmentWork{input: in, reason: reason}
	if err := m.loadOfferingAdjustmentSubject(ctx, work); err != nil {
		return nil, err
	}
	if err := m.loadOfferingAdjustmentCatalog(ctx, work); err != nil {
		return nil, err
	}
	if err := m.materializeOfferingAdjustment(ctx, work); err != nil {
		return nil, err
	}
	return work, nil
}

func (m *BookingMaterialization) validateOfferingAdjustmentInput(ctx context.Context, in careplan.OfferingAdjustment) (string, error) {
	if in.Source == careplan.OfferingAdjustmentSourceDirect {
		enabled, err := m.careOfferingsEnabled(ctx)
		if err != nil {
			return "", fmt.Errorf("offering adjustment: resolve care offerings setting: %w", err)
		}
		if !enabled {
			return "", careplan.ErrCareOfferingsDisabled
		}
	}
	if in.RequestID <= 0 || in.ChildID <= 0 {
		return "", fmt.Errorf("%w: request_id and child_id are required", careplan.ErrOfferingAdjustmentInvalid)
	}
	if in.ActorAccountID <= 0 {
		return "", fmt.Errorf("%w: actor account id is required", careplan.ErrOfferingAdjustmentInvalid)
	}
	reason := strings.TrimSpace(in.Reason)
	if reason == "" {
		return "", fmt.Errorf("%w: reason is required", careplan.ErrOfferingAdjustmentInvalid)
	}
	return reason, nil
}

func (m *BookingMaterialization) loadOfferingAdjustmentSubject(ctx context.Context, work *offeringAdjustmentWork) error {
	request, err := m.deps.Enrollment.Request(ctx, work.input.RequestID)
	if err != nil {
		return careplan.ErrBookingRequestNotFound
	}
	child, err := m.deps.Enrollment.Child(ctx, work.input.ChildID)
	if err != nil || child.RequestID != request.ID {
		return careplan.ErrBookingChildNotFound
	}
	if !child.Approved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return fmt.Errorf("%w: only approved children with a linked student can be adjusted", careplan.ErrOfferingAdjustmentInvalid)
	}
	work.child = child
	if err := m.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	student, err := m.deps.Students.Student(ctx, *child.CreatedStudentID, true)
	if err != nil {
		return fmt.Errorf("decision: lock adjustment student: %w", err)
	}
	phase, err := m.deps.Enrollment.Phase(ctx, request.PhaseID)
	if err != nil {
		return fmt.Errorf("decision: load adjustment phase: %w", err)
	}
	today := m.todayDate()
	effectiveFrom, err := validateAdjustmentEffectiveFrom(work.input.EffectiveFrom, phase.OfferingPhase, today)
	if err != nil {
		return err
	}
	work.student, work.phase = student, phase
	work.effectiveFrom = effectiveFrom
	work.selectionDate = adjustmentSelectionDate(phase.OfferingPhase, effectiveFrom, today)
	return nil
}

// validateAdjustmentEffectiveFrom keeps a dated switch inside the window it
// can actually describe. A date in the past would silently rewrite days that
// were already attended (the whole reason the dated path exists), and a date
// after the phase ends would produce a row whose exclusive end is not after
// its start, which the DB check would reject with a far less useful message.
func validateAdjustmentEffectiveFrom(effectiveFrom *calendar.Date, phase careplan.OfferingPhase, today calendar.Date) (*calendar.Date, error) {
	if effectiveFrom == nil {
		return nil, nil
	}
	if effectiveFrom.Before(today) {
		return nil, fmt.Errorf("%w: effective_from must not be in the past", careplan.ErrOfferingAdjustmentInvalid)
	}
	if effectiveFrom.After(phase.ServiceEnd) {
		return nil, fmt.Errorf("%w: effective_from must not be after the care period ends", careplan.ErrOfferingAdjustmentInvalid)
	}
	return effectiveFrom, nil
}

// adjustmentSelectionDate is the day the child's persisted selection is read
// at: today clamped into the phase's service window, or the dated switch when
// that lies later. Since a dated change splits the links into intervals,
// reading them without a date returns the whole history and reading them at
// the service start returns the superseded booking.
func adjustmentSelectionDate(phase careplan.OfferingPhase, effectiveFrom *calendar.Date, today calendar.Date) calendar.Date {
	selectionDate := today
	switch {
	case today.Before(phase.ServiceStart):
		selectionDate = phase.ServiceStart
	case today.After(phase.ServiceEnd):
		selectionDate = phase.ServiceEnd
	}
	if effectiveFrom != nil && effectiveFrom.After(selectionDate) {
		selectionDate = *effectiveFrom
	}
	return selectionDate
}

func (m *BookingMaterialization) loadOfferingAdjustmentCatalog(ctx context.Context, work *offeringAdjustmentWork) error {
	phaseID := work.phase.ID
	offerings, err := m.catalog.listRecords(ctx, phaseFilter(phaseID), "failed to list care offerings by phase")
	if err != nil {
		return fmt.Errorf("decision: list phase offerings for adjustment: %w", err)
	}
	activeFilter := phaseFilter(phaseID)
	activeFilter.ActiveOnly = true
	activeOfferings, err := m.catalog.listRecords(ctx, activeFilter, "failed to list active offerings by phase")
	if err != nil {
		return fmt.Errorf("decision: list active phase offerings for adjustment: %w", err)
	}
	offeringByID := offeringsByID(offerings)
	activeOfferingByID := offeringsByID(activeOfferings)
	addOfferingMap(offeringByID, activeOfferingByID)
	selections, err := m.deps.Enrollment.SelectionsAt(ctx, work.child.ID, work.selectionDate)
	if err != nil {
		return fmt.Errorf("decision: list current child offerings: %w", err)
	}
	beforeLinks := bookedOfferingPointers(selections)
	beforeOfferingByID := map[int64]*careplan.CareOffering{}
	if ids := bookedOfferingIDs(beforeLinks); len(ids) > 0 {
		beforeOfferings, listErr := m.catalog.listByIDs(ctx, ids)
		if listErr != nil {
			return fmt.Errorf("decision: list existing child offerings for adjustment: %w", listErr)
		}
		beforeOfferingByID = offeringsByID(beforeOfferings)
		addOfferingMap(offeringByID, beforeOfferingByID)
	}
	beforeJSON, err := adjustmentSnapshotJSON(beforeLinks, offeringByID)
	if err != nil {
		return err
	}
	work.offeringByID, work.activeOfferingByID = offeringByID, activeOfferingByID
	work.beforeOfferingByID, work.beforeLinks, work.beforeJSON = beforeOfferingByID, beforeLinks, beforeJSON
	return nil
}

func addOfferingMap(target, source map[int64]*careplan.CareOffering) {
	for id, offering := range source {
		target[id] = offering
	}
}

func derefBookedOfferings(links []*careplan.BookedOffering) []careplan.BookedOffering {
	if links == nil {
		return nil
	}
	values := make([]careplan.BookedOffering, 0, len(links))
	for _, link := range links {
		if link != nil {
			values = append(values, *link)
		}
	}
	return values
}

func derefOfferings(offerings map[int64]*careplan.CareOffering) map[int64]careplan.CareOffering {
	values := make(map[int64]careplan.CareOffering, len(offerings))
	for id, offering := range offerings {
		values[id] = *offering
	}
	return values
}
