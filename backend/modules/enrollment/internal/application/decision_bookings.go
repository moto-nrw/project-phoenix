package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking materialization is Care Plan's (#3560): the rosters an approval
// derives, the dated offering adjustments and the offering pickup times. The
// decision flow orchestrates approvals and applies the change-request
// decisions through it; it decides nothing about bookings itself.

// lockOfferingDerivedWrites establishes the project-wide gate order before
// the flow locks student rows and then changes booking- or offering-derived
// state.
func (d *Decisions) lockOfferingDerivedWrites(ctx context.Context) error {
	if d.deps.Bookings == nil {
		return nil
	}
	return d.deps.Bookings.LockOfferingDerivedWrites(ctx)
}

func (d *Decisions) lockTemplateRecurrence(ctx context.Context) error {
	if d.deps.LockTemplateRecurrence == nil {
		return nil
	}
	if err := d.deps.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("decision: lock template recurrence: %w", err)
	}
	return nil
}

// resyncOfferingSourcedTemplates re-reconciles every offering-sourced
// template after a confirmed edit moved a child into another Jahrgang.
func (d *Decisions) resyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error {
	if d.deps.Bookings == nil {
		d.logger().Warn("offering roster resync: enrollment repositories not configured; skipping sourced-template resync")
		return nil
	}
	return d.deps.Bookings.ResyncOfferingSourcedTemplates(ctx, effectiveFrom)
}

// materializeEnrollmentsForApproval writes the roster rows of the child's
// bookings during an approval. Wired without Care Plan bookings it skips:
// approvals still create the student record; the admin can attach activity
// groups later via the activity admin UI.
func (d *Decisions) materializeEnrollmentsForApproval(ctx context.Context, requestChildID, studentID int64, phase *enrollment.Phase) error {
	if d.deps.Bookings == nil {
		d.logger().Warn("decision: enrollment repos missing; skipping activity materialization",
			slog.Int64("request_child_id", requestChildID),
			slog.Int64("student_id", studentID))
		return nil
	}
	return d.deps.Bookings.MaterializeApprovedBookings(ctx, requestChildID, studentID, offeringPhaseOf(phase))
}

// resyncMultiSourceTemplatesForChild re-reconciles the multi-source templates
// fed by the child's offerings once the child is approved.
func (d *Decisions) resyncMultiSourceTemplatesForChild(ctx context.Context, requestChildID int64, phase *enrollment.Phase) error {
	if d.deps.Bookings == nil {
		return nil
	}
	return d.deps.Bookings.ResyncMultiSourceTemplatesForChild(ctx, requestChildID, offeringPhaseOf(phase))
}

// syncOfferingPickupAfterApproval refreshes projected-pickup consumers after
// an approved child has been linked to a student. It writes no weekly rows.
func (d *Decisions) syncOfferingPickupAfterApproval(ctx context.Context, child *RequestChild) error {
	if d.deps.Bookings == nil || child == nil {
		return nil
	}
	switch {
	case child.CreatedStudentID != nil && *child.CreatedStudentID > 0:
		return d.deps.Bookings.ReconcileOfferingPickupForStudents(ctx, []int64{*child.CreatedStudentID})
	case child.MatchedStudentID != nil && *child.MatchedStudentID > 0:
		return d.deps.Bookings.ReconcileOfferingPickupForStudents(ctx, []int64{*child.MatchedStudentID})
	default:
		return nil
	}
}

// UpdateChildOfferings corrects a child's bookings through Care Plan.
func (d *Decisions) UpdateChildOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput) (*enrollment.RequestChild, error) {
	return d.adjustChildOfferings(ctx, input, careplan.OfferingAdjustmentSourceDirect)
}

// ApplyChangeRequestOfferings applies an offering proposal whose capability
// was validated and pinned when the parent created the change request. The
// offering-change request flow separately enforces the live care-offerings
// setting before using this shared path; generic form corrections
// intentionally preserve their frozen offering snapshot.
func (d *Decisions) ApplyChangeRequestOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput) (*enrollment.RequestChild, error) {
	return d.adjustChildOfferings(ctx, input, careplan.OfferingAdjustmentSourceRequest)
}

// adjustChildOfferings runs the booking switch in Care Plan and reads the
// adjusted child back.
func (d *Decisions) adjustChildOfferings(ctx context.Context, input enrollment.UpdateChildOfferingsInput, source string) (*enrollment.RequestChild, error) {
	if d.deps.Bookings == nil || d.deps.Children == nil {
		return nil, errors.New("decision: offering adjustment dependencies are not configured")
	}
	result, err := d.deps.Bookings.AdjustOfferings(ctx, careplan.OfferingAdjustment{
		RequestID: input.RequestID, ChildID: input.ChildID, Offerings: adjustmentChoices(input.Offerings),
		ExcludedAutoAddTargetIDs: input.ExcludedAutoAddTargetIDs, Reason: input.Reason,
		ActorAccountID: input.ActorAccountID, ActorRole: input.ActorRole, EffectiveFrom: input.EffectiveFrom,
		CompleteWithdrawalConfirmed: input.CompleteWithdrawalConfirmed, Source: source,
	})
	if err != nil {
		return nil, err
	}
	child, err := d.deps.Children.ChildByID(ctx, result.RequestChildID)
	if err != nil {
		return nil, err
	}
	if _, err := childValue(child); err != nil {
		return nil, err
	}
	return child, nil
}

// offeringPhaseOf hands an enrollment phase's service window to Care Plan.
func offeringPhaseOf(phase *enrollment.Phase) careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}
}

func adjustmentChoices(rows []enrollment.OfferingAdjustmentSelection) []careplan.OfferingAdjustmentChoice {
	choices := make([]careplan.OfferingAdjustmentChoice, 0, len(rows))
	for _, row := range rows {
		choices = append(choices, careplan.OfferingAdjustmentChoice{OfferingID: row.OfferingID, SelectedDays: row.SelectedDays})
	}
	return choices
}
