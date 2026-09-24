package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Booking materialization is Care Plan's (#3560): the rosters an approval
// derives, the dated offering adjustments and the offering pickup times. The
// decision service still orchestrates approvals and the change-request
// services still apply their decisions through it, so it drives the owner
// through the port below and translates the owner's results into the
// enrollment rows its callers speak. It decides nothing itself.

// ErrCompleteWithdrawalConfirmationRequired points the enrollment name at the
// Care Plan value.
var ErrCompleteWithdrawalConfirmationRequired = careplan.ErrCompleteWithdrawalConfirmationRequired

// DecisionBookings is the Care Plan booking materialization an Enrollment
// decision drives.
type DecisionBookings interface {
	careplan.BookingMaterializer
	careplan.OfferingAdjustments
	ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error
}

// appliedOfferingAdjustment carries the exact materialization persisted by the
// shared adjustment path. The change-request decision uses it for its snapshot
// so a second catalog read cannot describe a different booking.
type appliedOfferingAdjustment struct {
	Child              *RequestChild
	Before             []*RequestChildOffering
	Selections         []materializedOfferingSelection
	Offerings          map[int64]*enrollmentModels.CareOffering
	Overridden         []enrollmentModels.OfferingChangeSnapshotOffering
	CompleteWithdrawal bool
}

// LockOfferingDerivedWrites establishes the project-wide gate order before a
// caller locks student rows and then changes booking- or offering-derived
// state.
func (s *decisionService) LockOfferingDerivedWrites(ctx context.Context) error {
	if s.CareBookings == nil {
		return nil
	}
	return s.CareBookings.LockOfferingDerivedWrites(ctx)
}

func (s *decisionService) lockTemplateRecurrence(ctx context.Context) error {
	if s.LockTemplateRecurrence == nil {
		return nil
	}
	if err := s.LockTemplateRecurrence(ctx); err != nil {
		return fmt.Errorf("decision: lock template recurrence: %w", err)
	}
	return nil
}

// ResyncOfferingSourcedTemplates re-reconciles every offering-sourced
// template after a confirmed edit moved a child into another Jahrgang.
func (s *decisionService) ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom timezone.Date) error {
	if s.CareBookings == nil {
		s.Logger.Warn("offering roster resync: enrollment repositories not configured; skipping sourced-template resync")
		return nil
	}
	return s.CareBookings.ResyncOfferingSourcedTemplates(ctx, effectiveFrom)
}

// ReconcileOfferingPickupForStudents refreshes the students' projected pickup
// consumers after their bookings changed.
func (s *decisionService) ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64) error {
	if s.CareBookings == nil {
		return nil
	}
	return s.CareBookings.ReconcileOfferingPickupForStudents(ctx, studentIDs)
}

// materializeEnrollmentsForApproval writes the roster rows of the child's
// bookings during an approval. Wired without Care Plan bookings it skips:
// approvals still create the student record; the admin can attach activity
// groups later via the activity admin UI.
func (s *decisionService) materializeEnrollmentsForApproval(ctx context.Context, requestChildID, studentID int64, phase *capability.Phase) error {
	if s.CareBookings == nil {
		s.Logger.Warn("decision: enrollment repos missing; skipping activity materialization",
			slog.Int64("request_child_id", requestChildID),
			slog.Int64("student_id", studentID))
		return nil
	}
	return s.CareBookings.MaterializeApprovedBookings(ctx, requestChildID, studentID, offeringPhaseOf(phase))
}

// resyncMultiSourceTemplatesForChild re-reconciles the multi-source templates
// fed by the child's offerings once the child is approved.
func (s *decisionService) resyncMultiSourceTemplatesForChild(ctx context.Context, requestChildID int64, phase *capability.Phase) error {
	if s.CareBookings == nil {
		return nil
	}
	return s.CareBookings.ResyncMultiSourceTemplatesForChild(ctx, requestChildID, offeringPhaseOf(phase))
}

// syncOfferingPickupAfterApproval refreshes projected-pickup consumers after
// an approved child has been linked to a student. It writes no weekly rows.
func (s *decisionService) syncOfferingPickupAfterApproval(ctx context.Context, child *RequestChild) error {
	switch {
	case child == nil:
		return nil
	case child.CreatedStudentID != nil && *child.CreatedStudentID > 0:
		return s.ReconcileOfferingPickupForStudents(ctx, []int64{*child.CreatedStudentID})
	case child.MatchedStudentID != nil && *child.MatchedStudentID > 0:
		return s.ReconcileOfferingPickupForStudents(ctx, []int64{*child.MatchedStudentID})
	default:
		return nil
	}
}

func (s *decisionService) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error) {
	result, err := s.adjustChildOfferings(ctx, input, careplan.OfferingAdjustmentSourceDirect)
	if err != nil {
		return nil, err
	}
	return result.Child, nil
}

// applyApprovedChangeRequestOfferings applies an offering proposal whose
// capability was validated and pinned when the parent created the change
// request. The offering-change request service separately enforces the live
// care-offerings setting before using this shared path; generic form
// corrections intentionally preserve their frozen offering snapshot.
func (s *decisionService) applyApprovedChangeRequestOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*RequestChild, error) {
	result, err := s.applyApprovedChangeRequestOfferingsWithResult(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Child, nil
}

func (s *decisionService) applyApprovedChangeRequestOfferingsWithResult(ctx context.Context, input UpdateChildOfferingsInput) (*appliedOfferingAdjustment, error) {
	return s.adjustChildOfferings(ctx, input, careplan.OfferingAdjustmentSourceRequest)
}

// adjustChildOfferings runs the booking switch in Care Plan and reads the
// adjusted child back in the enrollment rows the callers return.
func (s *decisionService) adjustChildOfferings(ctx context.Context, input UpdateChildOfferingsInput, source string) (*appliedOfferingAdjustment, error) {
	if s.CareBookings == nil || s.Children == nil {
		return nil, errors.New("decision: offering adjustment dependencies are not configured")
	}
	result, err := s.CareBookings.AdjustOfferings(ctx, careplan.OfferingAdjustment{
		RequestID: input.RequestID, ChildID: input.ChildID, Offerings: adjustmentChoices(input.Offerings),
		ExcludedAutoAddTargetIDs: input.ExcludedAutoAddTargetIDs, Reason: input.Reason,
		ActorAccountID: input.ActorAccountID, ActorRole: input.ActorRole, EffectiveFrom: input.EffectiveFrom,
		CompleteWithdrawalConfirmed: input.CompleteWithdrawalConfirmed, Source: source,
	})
	if err != nil {
		return nil, err
	}
	child, err := offeringChildByID(ctx, s.Children, result.RequestChildID)
	if err != nil {
		return nil, err
	}
	offerings := make(map[int64]*enrollmentModels.CareOffering, len(result.Offerings))
	for id, value := range result.Offerings {
		offering, err := careOfferingToLegacy(value)
		if err != nil {
			return nil, err
		}
		offerings[id] = offering
	}
	return &appliedOfferingAdjustment{
		Child: child, Before: requestChildOfferingsOf(result.Before), Selections: materializedSelectionsOf(result.Selections),
		Offerings: offerings, Overridden: snapshotOfferingsOf(result.Overridden), CompleteWithdrawal: result.CompleteWithdrawal,
	}, nil
}

// offeringPhaseOf hands an enrollment phase's service window to Care Plan.
func offeringPhaseOf(phase *capability.Phase) careplan.OfferingPhase {
	return careplan.OfferingPhase{
		ID: phase.ID, Name: phase.Name,
		ServiceStart: calendar.Date(phase.ServiceStartDate), ServiceEnd: calendar.Date(phase.ServiceEndDate),
	}
}

func adjustmentChoices(rows []OfferingAdjustmentSelection) []careplan.OfferingAdjustmentChoice {
	choices := make([]careplan.OfferingAdjustmentChoice, 0, len(rows))
	for _, row := range rows {
		choices = append(choices, careplan.OfferingAdjustmentChoice{OfferingID: row.OfferingID, SelectedDays: row.SelectedDays})
	}
	return choices
}

func requestChildOfferingsOf(values []careplan.BookedOffering) []*RequestChildOffering {
	if values == nil {
		return nil
	}
	links := make([]*RequestChildOffering, 0, len(values))
	for _, value := range values {
		links = append(links, &RequestChildOffering{
			RequestChildID: value.RequestChildID, CareOfferingID: value.CareOfferingID,
			SelectedDays: value.SelectedDays, ManualSelectedDays: value.ManualSelectedDays,
			AutomaticSelectedDays: value.AutomaticSelectedDays,
			ValidFrom:             value.ValidFrom, ValidUntil: value.ValidUntil,
		})
	}
	return links
}

func materializedSelectionsOf(values []careplan.OfferingSelection) []materializedOfferingSelection {
	selections := make([]materializedOfferingSelection, 0, len(values))
	for _, value := range values {
		selections = append(selections, materializedOfferingSelection{
			OfferingID: value.OfferingID, SelectedDays: value.SelectedDays,
			ManualSelectedDays: value.ManualSelectedDays, AutomaticSelectedDays: value.AutomaticSelectedDays,
		})
	}
	return selections
}

func snapshotOfferingsOf(values []careplan.OfferingOverride) []enrollmentModels.OfferingChangeSnapshotOffering {
	if values == nil {
		return nil
	}
	offerings := make([]enrollmentModels.OfferingChangeSnapshotOffering, 0, len(values))
	for _, value := range values {
		offerings = append(offerings, enrollmentModels.OfferingChangeSnapshotOffering{OfferingID: value.OfferingID, Name: value.Name})
	}
	return offerings
}
