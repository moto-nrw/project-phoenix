package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/activities"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

var ErrCompleteWithdrawalConfirmationRequired = errors.New("Alle Betreuungstage werden entfernt. Bitte bestätigen Sie die Komplett-Abmeldung.") //nolint:staticcheck // user-facing German message

type offeringAdjustmentSnapshot struct {
	OfferingID            string   `json:"offering_id"`
	OfferingName          string   `json:"offering_name"`
	DaysOfWeekMode        string   `json:"days_of_week_mode"`
	SelectedDays          []string `json:"selected_days,omitempty"`
	ManualSelectedDays    []string `json:"manual_selected_days,omitempty"`
	AutomaticSelectedDays []string `json:"automatic_selected_days,omitempty"`
	AvailableDays         []string `json:"available_days,omitempty"`
}

// appliedOfferingAdjustment carries the exact materialization persisted by the
// shared adjustment path. The change-request decision uses it for its snapshot
// so a second catalog read cannot describe a different booking.
type appliedOfferingAdjustment struct {
	Child      *enrollmentModels.RequestChild
	Before     []*enrollmentModels.RequestChildOffering
	Selections []materializedOfferingSelection
	Offerings  map[int64]*enrollmentModels.CareOffering
	Overridden []enrollmentModels.OfferingChangeSnapshotOffering
}

func (s *decisionService) ListOfferingAdjustments(ctx context.Context, requestID, requestChildID int64) ([]*auditModels.EnrollmentOfferingAdjustment, error) {
	if s.OfferingAdjustmentRepo == nil {
		return nil, fmt.Errorf("decision: offering adjustment repo not configured")
	}
	if requestID <= 0 || requestChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	child, err := s.RequestChildRepo.FindByID(ctx, requestChildID)
	if err != nil || child == nil || child.RequestID != requestID {
		return nil, ErrDecisionChildNotFound
	}
	return s.OfferingAdjustmentRepo.ListByRequestChildID(ctx, requestChildID)
}

func (s *decisionService) UpdateChildOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error) {
	result, err := s.updateChildOfferings(ctx, input, auditModels.OfferingAdjustmentSourceDirect)
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
func (s *decisionService) applyApprovedChangeRequestOfferings(ctx context.Context, input UpdateChildOfferingsInput) (*enrollmentModels.RequestChild, error) {
	result, err := s.applyApprovedChangeRequestOfferingsWithResult(ctx, input)
	if err != nil {
		return nil, err
	}
	return result.Child, nil
}

func (s *decisionService) applyApprovedChangeRequestOfferingsWithResult(
	ctx context.Context,
	input UpdateChildOfferingsInput,
) (*appliedOfferingAdjustment, error) {
	return s.updateChildOfferings(ctx, input, auditModels.OfferingAdjustmentSourceRequest)
}

// updateChildOfferings performs the booking switch and records it. source
// tells the two entry points apart: a direct correction by the office is
// subject to the live care-offerings setting and shows up in the central
// history as its own row kind (#2436), while a request-applied change carries
// the capability frozen when the parent submitted and is already visible there
// as the decided request.
func (s *decisionService) updateChildOfferings(
	ctx context.Context,
	input UpdateChildOfferingsInput,
	source string,
) (*appliedOfferingAdjustment, error) {
	work, err := s.prepareOfferingAdjustment(ctx, input, source)
	if err != nil {
		return nil, err
	}
	return s.applyPreparedOfferingAdjustment(ctx, work)
}

func (s *decisionService) prepareOfferingAdjustment(
	ctx context.Context, input UpdateChildOfferingsInput, source string,
) (*offeringAdjustmentWork, error) {
	reason, err := s.validateOfferingAdjustmentInput(ctx, input, source)
	if err != nil {
		return nil, err
	}
	work := &offeringAdjustmentWork{input: input, source: source, reason: reason}
	if err := s.loadOfferingAdjustmentSubject(ctx, work); err != nil {
		return nil, err
	}
	if err := s.loadOfferingAdjustmentCatalog(ctx, work); err != nil {
		return nil, err
	}
	if err := s.materializeOfferingAdjustment(ctx, work); err != nil {
		return nil, err
	}
	return work, nil
}

func (s *decisionService) applyPreparedOfferingAdjustment(
	ctx context.Context, work *offeringAdjustmentWork,
) (*appliedOfferingAdjustment, error) {
	if err := s.persistOfferingAdjustment(ctx, work); err != nil {
		return nil, err
	}
	entry, err := s.recordOfferingAdjustment(ctx, work)
	if err != nil {
		return nil, err
	}
	if err := s.reconcileOfferingAdjustmentWithdrawal(ctx, work, entry); err != nil {
		return nil, err
	}
	return s.finishOfferingAdjustment(ctx, work)
}

type offeringAdjustmentWork struct {
	input                                  UpdateChildOfferingsInput
	source, reason                         string
	request                                *enrollmentModels.Request
	child                                  *enrollmentModels.RequestChild
	student                                *users.Student
	phase                                  *enrollmentModels.Phase
	effectiveFrom                          *timezone.Date
	selectionDate                          timezone.Date
	offeringByID                           map[int64]*enrollmentModels.CareOffering
	activeOfferingByID, beforeOfferingByID map[int64]*enrollmentModels.CareOffering
	beforeLinks                            []*enrollmentModels.RequestChildOffering
	beforeJSON, afterJSON                  []byte
	selections                             []materializedOfferingSelection
	replacement                            []*enrollmentModels.RequestChildOffering
	overridden                             []enrollmentModels.OfferingChangeSnapshotOffering
	authoritative, afterHasCareDays        bool
	isCompleteWithdrawal                   bool
}

func (s *decisionService) validateOfferingAdjustmentInput(
	ctx context.Context, input UpdateChildOfferingsInput, source string,
) (string, error) {
	if source == auditModels.OfferingAdjustmentSourceDirect {
		enabled, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentCareOfferingsEnabled, true)
		if err != nil {
			return "", fmt.Errorf("offering adjustment: resolve care offerings setting: %w", err)
		}
		if !enabled {
			return "", ErrCareOfferingsDisabled
		}
	}
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return "", fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	if input.ActorAccountID <= 0 {
		return "", fmt.Errorf("%w: actor account id is required", ErrOfferingAdjustmentInvalid)
	}
	reason := strings.TrimSpace(input.Reason)
	if reason == "" {
		return "", fmt.Errorf("%w: reason is required", ErrOfferingAdjustmentInvalid)
	}
	if s.RequestRepo == nil || s.RequestChildRepo == nil || s.RequestChildOfferingRepo == nil ||
		s.CareOfferingRepo == nil || s.PhaseRepo == nil || s.OfferingAdjustmentRepo == nil || s.StudentRepo == nil {
		return "", errors.New("decision: offering adjustment dependencies are not configured")
	}
	return reason, nil
}

func (s *decisionService) loadOfferingAdjustmentSubject(ctx context.Context, work *offeringAdjustmentWork) error {
	req, err := s.RequestRepo.FindByID(ctx, work.input.RequestID)
	if err != nil {
		return ErrDecisionRequestNotFound
	}
	child, err := s.RequestChildRepo.FindByID(ctx, work.input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return fmt.Errorf("%w: only approved children with a linked student can be adjusted", ErrOfferingAdjustmentInvalid)
	}
	work.request, work.child = req, child
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, *child.CreatedStudentID)
	if err != nil {
		return fmt.Errorf("decision: lock adjustment student: %w", err)
	}
	if student == nil {
		return fmt.Errorf("%w: linked student was not found", ErrOfferingAdjustmentInvalid)
	}
	phase, err := s.PhaseRepo.FindByID(ctx, req.PhaseID)
	if err != nil || phase == nil {
		return fmt.Errorf("decision: load adjustment phase: %w", err)
	}
	effectiveFrom, err := validateAdjustmentEffectiveFrom(work.input.EffectiveFrom, phase)
	if err != nil {
		return err
	}
	work.student, work.phase = student, phase
	work.effectiveFrom = effectiveFrom
	work.selectionDate = adjustmentSelectionDate(phase, effectiveFrom)
	return nil
}

func adjustmentSelectionDate(phase *enrollmentModels.Phase, effectiveFrom *timezone.Date) timezone.Date {
	selectionDate := currentOfferingSelectionDate(phase)
	if effectiveFrom != nil && effectiveFrom.After(selectionDate) {
		selectionDate = *effectiveFrom
	}
	return selectionDate
}

func (s *decisionService) loadOfferingAdjustmentCatalog(ctx context.Context, work *offeringAdjustmentWork) error {
	offerings, err := s.CareOfferingRepo.ListByPhase(ctx, work.request.PhaseID)
	if err != nil {
		return fmt.Errorf("decision: list phase offerings for adjustment: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	addCareOfferingsByID(offeringByID, offerings)
	activeOfferings, err := s.CareOfferingRepo.ListActiveByPhase(ctx, work.request.PhaseID)
	if err != nil {
		return fmt.Errorf("decision: list active phase offerings for adjustment: %w", err)
	}
	activeOfferingByID := make(map[int64]*enrollmentModels.CareOffering, len(activeOfferings))
	addCareOfferingsByID(activeOfferingByID, activeOfferings)
	addCareOfferingsByID(offeringByID, activeOfferings)
	beforeLinks, err := s.RequestChildOfferingRepo.ListByRequestChildIDAtDate(ctx, work.child.ID, work.selectionDate)
	if err != nil {
		return fmt.Errorf("decision: list current child offerings: %w", err)
	}
	beforeOfferingByID := map[int64]*enrollmentModels.CareOffering{}
	if ids := offeringIDsFromLinks(beforeLinks); len(ids) > 0 {
		beforeOfferings, listErr := s.CareOfferingRepo.ListByIDs(ctx, ids)
		if listErr != nil {
			return fmt.Errorf("decision: list existing child offerings for adjustment: %w", listErr)
		}
		addCareOfferingsByID(beforeOfferingByID, beforeOfferings)
		addCareOfferingsByID(offeringByID, beforeOfferings)
	}
	beforeJSON, err := adjustmentSnapshotJSON(beforeLinks, offeringByID)
	if err != nil {
		return err
	}
	work.offeringByID, work.activeOfferingByID = offeringByID, activeOfferingByID
	work.beforeOfferingByID, work.beforeLinks, work.beforeJSON = beforeOfferingByID, beforeLinks, beforeJSON
	return nil
}

func addCareOfferingsByID(target map[int64]*enrollmentModels.CareOffering, offerings []*enrollmentModels.CareOffering) {
	for _, offering := range offerings {
		if offering != nil {
			target[offering.ID] = offering
		}
	}
}

func buildAdjustmentSubmitChild(work *offeringAdjustmentWork) (SubmitChild, error) {
	input, child := work.input, work.child
	submitChild := SubmitChild{
		FirstName:                child.FirstName,
		LastName:                 child.LastName,
		DateOfBirth:              child.DateOfBirth,
		TargetGradeLevel:         child.TargetGradeLevel,
		CustomData:               child.CustomData,
		OfferingIDs:              make([]int64, 0, len(input.Offerings)),
		OfferingDays:             make([]SubmitOfferingDays, 0, len(input.Offerings)),
		ExcludedAutoAddTargetIDs: input.ExcludedAutoAddTargetIDs,
	}
	seen := make(map[int64]bool, len(input.Offerings))
	for _, row := range input.Offerings {
		if row.OfferingID <= 0 {
			return SubmitChild{}, fmt.Errorf("%w: offering_id is required", ErrOfferingAdjustmentInvalid)
		}
		if work.activeOfferingByID[row.OfferingID] == nil && work.beforeOfferingByID[row.OfferingID] == nil {
			return SubmitChild{}, fmt.Errorf("%w: care offering %d is not available for this child adjustment", ErrCareOfferingMissing, row.OfferingID)
		}
		if seen[row.OfferingID] {
			continue
		}
		seen[row.OfferingID] = true
		submitChild.OfferingIDs = append(submitChild.OfferingIDs, row.OfferingID)
		if len(row.SelectedDays) > 0 {
			submitChild.OfferingDays = append(submitChild.OfferingDays, SubmitOfferingDays{
				OfferingID:   row.OfferingID,
				SelectedDays: copyDays(row.SelectedDays),
			})
		}
	}
	sortAdjustmentOfferingIDs(submitChild.OfferingIDs, work.offeringByID)
	return submitChild, nil
}

func sortAdjustmentOfferingIDs(ids []int64, offerings map[int64]*enrollmentModels.CareOffering) {
	sort.SliceStable(ids, func(i, j int) bool {
		left, right := offerings[ids[i]], offerings[ids[j]]
		if left == nil || right == nil || left.SortOrder == right.SortOrder {
			return ids[i] < ids[j]
		}
		return left.SortOrder < right.SortOrder
	})
}

func (s *decisionService) resolveAdjustmentAuthority(ctx context.Context, work *offeringAdjustmentWork) error {
	authoritative, err := s.resolveDecisionBool(ctx, configModel.KeyEnrollmentBookingsAuthoritative, false)
	if err != nil {
		return fmt.Errorf("offering adjustment: resolve authoritative bookings setting: %w", err)
	}
	if authoritative && s.CareWithdrawal == nil {
		return errors.New("offering adjustment: authoritative booking lifecycle is not configured")
	}
	work.authoritative = authoritative
	return nil
}

func (s *decisionService) materializeOfferingAdjustment(ctx context.Context, work *offeringAdjustmentWork) error {
	submitChild, err := buildAdjustmentSubmitChild(work)
	if err != nil {
		return err
	}
	if err := s.resolveAdjustmentAuthority(ctx, work); err != nil {
		return err
	}
	allowed := make(map[int64]*enrollmentModels.CareOffering, len(work.activeOfferingByID)+len(work.beforeOfferingByID))
	addCareOfferingMap(allowed, work.activeOfferingByID)
	addCareOfferingMap(allowed, work.beforeOfferingByID)
	grandfathered := grandfatheredOfferingsFromLinks(work.beforeLinks)
	allowWithdrawal := work.authoritative && requestChildOfferingLinksHaveCareDays(work.beforeLinks, work.offeringByID)
	overridden, err := validateAppliedOfferingOverrides(
		work.input.ExcludedAutoAddTargetIDs, submitChild, allowed,
		work.phase.CareOfferingSelectionMode, grandfathered,
	)
	if err != nil {
		return err
	}
	work.reason = adjustmentReasonWithOverrides(work.reason, overridden)
	materialized, err := materializeAndValidateChildrenOfferingSelectionsForAdjustment(
		[]SubmitChild{submitChild}, allowed, work.phase.CareOfferingSelectionMode, grandfathered, allowWithdrawal,
	)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrOfferingAdjustmentInvalid, err)
	}
	work.selections, work.overridden = materialized[0], overridden
	work.afterHasCareDays = materializedSelectionsHaveCareDays(work.selections, work.offeringByID)
	work.isCompleteWithdrawal = allowWithdrawal && !work.afterHasCareDays
	if work.source == auditModels.OfferingAdjustmentSourceDirect && work.isCompleteWithdrawal && !work.input.CompleteWithdrawalConfirmed {
		return ErrCompleteWithdrawalConfirmationRequired
	}
	return s.buildOfferingAdjustmentReplacement(work)
}

func addCareOfferingMap(target, source map[int64]*enrollmentModels.CareOffering) {
	for id, offering := range source {
		target[id] = offering
	}
}

func (s *decisionService) buildOfferingAdjustmentReplacement(work *offeringAdjustmentWork) error {
	replacement := make([]*enrollmentModels.RequestChildOffering, 0, len(work.selections))
	for _, selection := range work.selections {
		replacement = append(replacement, &enrollmentModels.RequestChildOffering{
			RequestChildID:        work.child.ID,
			CareOfferingID:        selection.OfferingID,
			SelectedDays:          selection.SelectedDays,
			ManualSelectedDays:    selection.ManualSelectedDays,
			AutomaticSelectedDays: selection.AutomaticSelectedDays,
		})
	}
	replacement = capOfferingReplacementAtCareEnd(replacement, work.student.EnrolledUntil, work.phase, work.effectiveFrom)
	afterJSON, err := adjustmentSnapshotJSON(replacement, work.offeringByID)
	if err != nil {
		return err
	}
	work.replacement, work.afterJSON = replacement, afterJSON
	return nil
}

func (s *decisionService) persistOfferingAdjustment(ctx context.Context, work *offeringAdjustmentWork) error {
	scheduled, err := s.scheduledOfferingReplacements(ctx, work.child.ID, work.selectionDate, work.effectiveFrom)
	if err != nil {
		return err
	}
	if err := s.persistOfferingSourceRows(ctx, work, scheduled); err != nil {
		return err
	}
	materializeFrom := work.effectiveFrom
	if materializeFrom == nil && len(scheduled) > 0 {
		materializeFrom = &work.selectionDate
	}
	studentID := *work.child.CreatedStudentID
	if err := s.rematerializeAdjustedEnrollments(ctx, work.child.ID, studentID, work.beforeLinks, work.replacement, work.phase, materializeFrom); err != nil {
		return err
	}
	for _, future := range scheduled {
		if err := s.splitAdjustedEnrollments(ctx, work.child.ID, studentID, future.Rows, work.phase, future.EffectiveFrom); err != nil {
			return err
		}
	}
	return nil
}

func (s *decisionService) persistOfferingSourceRows(
	ctx context.Context, work *offeringAdjustmentWork, scheduled []scheduledOfferingReplacement,
) error {
	if work.effectiveFrom != nil || len(scheduled) > 0 {
		if err := s.RequestChildOfferingRepo.ScheduleReplacementForRequestChild(ctx, work.child.ID, work.selectionDate, work.replacement); err != nil {
			return fmt.Errorf("decision: schedule child offerings: %w", err)
		}
		for _, future := range scheduled {
			if err := s.RequestChildOfferingRepo.ScheduleReplacementForRequestChild(ctx, work.child.ID, future.EffectiveFrom, future.Rows); err != nil {
				return fmt.Errorf("decision: restore scheduled child offerings: %w", err)
			}
		}
		return nil
	}
	if err := s.RequestChildOfferingRepo.ReplaceForRequestChild(ctx, work.child.ID, work.replacement); err != nil {
		return fmt.Errorf("decision: replace child offerings: %w", err)
	}
	return nil
}

func (s *decisionService) recordOfferingAdjustment(
	ctx context.Context, work *offeringAdjustmentWork,
) (*auditModels.EnrollmentOfferingAdjustment, error) {
	actorName, actorEmail := s.actorSnapshot(ctx, work.input.ActorAccountID)
	entry := &auditModels.EnrollmentOfferingAdjustment{
		RequestID:                   work.request.ID,
		RequestChildID:              work.child.ID,
		StudentID:                   *work.child.CreatedStudentID,
		ActorAccountID:              work.input.ActorAccountID,
		ActorRole:                   strings.TrimSpace(work.input.ActorRole),
		ActorNameSnapshot:           actorName,
		ActorEmailSnapshot:          actorEmail,
		Reason:                      work.reason,
		Source:                      work.source,
		Before:                      work.beforeJSON,
		After:                       work.afterJSON,
		CompleteWithdrawalConfirmed: work.isCompleteWithdrawal,
	}
	if entry.ActorRole == "" {
		entry.ActorRole = "admin"
	}
	if err := s.OfferingAdjustmentRepo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("decision: create offering adjustment audit: %w", err)
	}
	return entry, nil
}

func (s *decisionService) reconcileOfferingAdjustmentWithdrawal(
	ctx context.Context, work *offeringAdjustmentWork, entry *auditModels.EnrollmentOfferingAdjustment,
) error {
	if s.CareWithdrawal == nil || (!work.authoritative && !work.afterHasCareDays) {
		return nil
	}
	err := s.CareWithdrawal.ReconcileAuthoritativeBookingChange(ctx, users.CareWithdrawalBookingChange{
		StudentID: *work.child.CreatedStudentID, FirstBookinglessDay: work.selectionDate,
		HasCareDays: work.afterHasCareDays, WasCompleteWithdrawal: work.isCompleteWithdrawal,
		SourceAdjustmentID: entry.ID, ConfirmedBy: work.input.ActorAccountID,
		ConfirmedRole: entry.ActorRole, SourceOfferings: careExitSourceOfferingsFromLinks(work.beforeLinks, work.offeringByID),
	})
	if err != nil {
		return fmt.Errorf("offering adjustment: reconcile complete withdrawal: %w", err)
	}
	return nil
}

func (s *decisionService) finishOfferingAdjustment(
	ctx context.Context, work *offeringAdjustmentWork,
) (*appliedOfferingAdjustment, error) {
	studentID := *work.child.CreatedStudentID
	if err := s.ReconcileOfferingPickupForStudents(ctx, []int64{studentID}); err != nil {
		return nil, fmt.Errorf("decision: reconcile offering pickup times: %w", err)
	}
	updated, err := s.RequestChildRepo.FindByID(ctx, work.child.ID)
	if err != nil {
		return nil, err
	}
	return &appliedOfferingAdjustment{
		Child: updated, Before: work.beforeLinks, Selections: work.selections,
		Offerings: work.offeringByID, Overridden: work.overridden,
	}, nil
}

func capOfferingReplacementAtCareEnd(
	rows []*enrollmentModels.RequestChildOffering,
	lastCareDay *timezone.Date,
	phase *enrollmentModels.Phase,
	effectiveFrom *timezone.Date,
) []*enrollmentModels.RequestChildOffering {
	if lastCareDay == nil {
		return rows
	}
	validFrom := phase.ServiceStartDate
	if effectiveFrom != nil {
		validFrom = *effectiveFrom
	}
	validUntil := lastCareDay.AddDays(1)
	if !validFrom.Before(validUntil) {
		return nil
	}
	for _, row := range rows {
		from, until := validFrom, validUntil
		row.ValidFrom, row.ValidUntil = &from, &until
	}
	return rows
}

func careExitSourceOfferingsFromLinks(
	links []*enrollmentModels.RequestChildOffering,
	offerings map[int64]*enrollmentModels.CareOffering,
) []users.CareExitSourceOffering {
	result := make([]users.CareExitSourceOffering, 0, len(links))
	for _, link := range links {
		if link == nil || len(link.SelectedDays) == 0 {
			continue
		}
		offering := offerings[link.CareOfferingID]
		if offering == nil {
			continue
		}
		result = append(result, users.CareExitSourceOffering{
			Name: offering.Name, Days: copyDays(link.SelectedDays),
		})
	}
	return result
}

func requestChildOfferingLinksHaveCareDays(
	links []*enrollmentModels.RequestChildOffering,
	offerings map[int64]*enrollmentModels.CareOffering,
) bool {
	for _, link := range links {
		if link == nil {
			continue
		}
		offering := offerings[link.CareOfferingID]
		if offering == nil || !offering.CountsAsCare {
			continue
		}
		hasCareDays := len(link.SelectedDays) > 0
		if offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			hasCareDays = len(offering.AvailableDays) > 0
		}
		if hasCareDays {
			return true
		}
	}
	return false
}

func validateAppliedOfferingOverrides(
	excluded map[int64]bool,
	child SubmitChild,
	allowed map[int64]*enrollmentModels.CareOffering,
	selectionMode string,
	grandfathered GrandfatheredOfferings,
) ([]enrollmentModels.OfferingChangeSnapshotOffering, error) {
	if len(excluded) == 0 {
		return nil, nil
	}
	child.ExcludedAutoAddTargetIDs = nil
	base, err := materializeAndValidateChildrenOfferingSelectionsGrandfathering(
		[]SubmitChild{child}, allowed, selectionMode, grandfathered,
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrOfferingAdjustmentInvalid, err)
	}
	excludedIDs := make([]int64, 0, len(excluded))
	for offeringID := range excluded {
		excludedIDs = append(excludedIDs, offeringID)
	}
	sort.Slice(excludedIDs, func(i, j int) bool { return excludedIDs[i] < excludedIDs[j] })
	return validateExcludedAutoTargets(excludedIDs, base[0], allowed)
}

func adjustmentReasonWithOverrides(
	reason string,
	overridden []enrollmentModels.OfferingChangeSnapshotOffering,
) string {
	if len(overridden) == 0 {
		return reason
	}
	names := make([]string, 0, len(overridden))
	for _, offering := range overridden {
		names = append(names, offering.Name)
	}
	return reason + " · Mitbuchung nicht angewendet: " + strings.Join(names, ", ")
}

// grandfatheredOfferingsFromLinks classifies the bookings a child already
// holds for the Bestandsschutz exemption (#2186): an availability rule
// tightened after the fact does not revoke what is already on file for THIS
// child, even when the grade rule now excludes it. Newly added blocked
// offerings are not on file and are still rejected with
// ErrCareOfferingUnavailable.
//
// The split follows the DAYS on each link, not the link as a whole: manual
// days are what the admin ticked and can untick, automatic days are derived
// from a trigger and never appear in a payload at all. A link can carry both,
// so the two buckets deliberately OVERLAP (#2186 review) — unticking such a
// booking withdraws only its manual half, and its automatic half must still be
// re-derived while the trigger stays selected. Classifying a mixed link as
// manual only dropped it out of the auto-materialization catalog on that very
// save and deleted its automatic days.
func grandfatheredOfferingsFromLinks(links []*enrollmentModels.RequestChildOffering) GrandfatheredOfferings {
	grandfathered := GrandfatheredOfferings{
		Manual:    make(map[int64]bool, len(links)),
		Automatic: make(map[int64]bool, len(links)),
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		if len(link.AutomaticSelectedDays) > 0 {
			grandfathered.Automatic[link.CareOfferingID] = true
		}
		// Legacy links carry neither breakdown, only SelectedDays. Nothing
		// derived those, so they are manual by construction.
		if len(link.ManualSelectedDays) > 0 || len(link.AutomaticSelectedDays) == 0 {
			grandfathered.Manual[link.CareOfferingID] = true
		}
	}
	return grandfathered
}

// validateAdjustmentEffectiveFrom keeps a dated switch inside the window it
// can actually describe. A date in the past would silently rewrite days that
// were already attended (the whole reason the dated path exists), and a date
// after the phase ends would produce a row whose exclusive end is not after
// its start, which the DB check would reject with a far less useful message.
func validateAdjustmentEffectiveFrom(
	effectiveFrom *timezone.Date,
	phase *enrollmentModels.Phase,
) (*timezone.Date, error) {
	if effectiveFrom == nil {
		return nil, nil
	}
	if effectiveFrom.Before(timezone.TodayDate()) {
		return nil, fmt.Errorf("%w: effective_from must not be in the past", ErrOfferingAdjustmentInvalid)
	}
	if effectiveFrom.After(phase.ServiceEndDate) {
		return nil, fmt.Errorf("%w: effective_from must not be after the care period ends", ErrOfferingAdjustmentInvalid)
	}
	return effectiveFrom, nil
}

// currentOfferingSelectionDate returns the day a child's persisted offering
// links have to be read at to yield the booking that is in force right now.
// Since a dated change splits those links into intervals, reading them without
// a date returns the whole history and reading them at the service start
// returns the superseded booking - both of which a caller asking for "the
// current selection" would misread as a live one. The window clamp keeps the
// answer meaningful outside the service period too: before it starts that is
// the initial selection, after it ended the last one.
func currentOfferingSelectionDate(phase *enrollmentModels.Phase) timezone.Date {
	today := timezone.TodayDate()
	if phase == nil {
		return today
	}
	if today.Before(phase.ServiceStartDate) {
		return phase.ServiceStartDate
	}
	if today.After(phase.ServiceEndDate) {
		return phase.ServiceEndDate
	}
	return today
}

type scheduledOfferingReplacement struct {
	EffectiveFrom timezone.Date
	Rows          []*enrollmentModels.RequestChildOffering
}

// scheduledOfferingReplacements captures future changes before an undated
// staff correction rewrites the current selection. A correction applies now;
// it must not silently cancel a separately approved future request.
func (s *decisionService) scheduledOfferingReplacements(
	ctx context.Context,
	requestChildID int64,
	selectionDate timezone.Date,
	effectiveFrom *timezone.Date,
) ([]scheduledOfferingReplacement, error) {
	if effectiveFrom != nil {
		return nil, nil
	}
	history, err := s.RequestChildOfferingRepo.ListHistoryByRequestChildID(ctx, requestChildID)
	if err != nil {
		return nil, fmt.Errorf("decision: list scheduled child offerings: %w", err)
	}
	byDate := make(map[timezone.Date][]*enrollmentModels.RequestChildOffering)
	for _, row := range history {
		if row == nil || row.ValidFrom == nil || !row.ValidFrom.After(selectionDate) {
			continue
		}
		date := *row.ValidFrom
		byDate[date] = append(byDate[date], row)
	}
	dates := make([]timezone.Date, 0, len(byDate))
	for date := range byDate {
		dates = append(dates, date)
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i].Before(dates[j]) })
	scheduled := make([]scheduledOfferingReplacement, 0, len(dates))
	for _, date := range dates {
		scheduled = append(scheduled, scheduledOfferingReplacement{EffectiveFrom: date, Rows: byDate[date]})
	}
	return scheduled, nil
}

func (s *decisionService) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*enrollmentModels.RequestChild, error) {
	if input.RequestID <= 0 || input.ChildID <= 0 {
		return nil, fmt.Errorf("%w: request_id and child_id are required", ErrOfferingAdjustmentInvalid)
	}
	if s.RequestRepo == nil || s.RequestChildRepo == nil || s.StudentRepo == nil || s.PersonRepo == nil {
		return nil, fmt.Errorf("decision: approved child sync dependencies are not configured")
	}

	req, err := s.RequestRepo.FindByID(ctx, input.RequestID)
	if err != nil {
		return nil, ErrDecisionRequestNotFound
	}
	child, err := s.RequestChildRepo.FindByID(ctx, input.ChildID)
	if err != nil || child == nil || child.RequestID != req.ID {
		return nil, ErrDecisionChildNotFound
	}
	if child.Status != enrollmentModels.ChildStatusApproved || child.CreatedStudentID == nil || *child.CreatedStudentID <= 0 {
		return nil, fmt.Errorf("%w: only approved children with a linked student can be synced", ErrOfferingAdjustmentInvalid)
	}

	// Tenant-wide gates BEFORE the first student row lock, in the project-wide
	// class-change order (shared class-writes gate first, recurrence gate
	// second, row locks last) — the same order the direct school_class PUT
	// takes in acquirePreRowLockGates. Locking the row first and the
	// recurrence gate only later (in the class-change branch below) let this
	// sync and a concurrent direct PUT on the same child wait on each other
	// cyclically until PostgreSQL aborted one (#2147 review round 15). The
	// gates are taken unconditionally because whether the confirmed edit
	// actually changes the class is only known once the row is locked.
	if err := s.LockOfferingDerivedWrites(ctx); err != nil {
		return nil, err
	}

	student, err := s.StudentRepo.FindByIDForUpdate(ctx, *child.CreatedStudentID)
	if err != nil || student == nil {
		return nil, ErrDecisionStudentNotFound
	}
	person, err := s.PersonRepo.FindByID(ctx, student.PersonID)
	if err != nil || person == nil {
		return nil, fmt.Errorf("decision: load approved child person: %w", err)
	}

	person.FirstName = child.FirstName
	person.LastName = child.LastName
	dob := child.DateOfBirth
	person.Birthday = &dob
	if err := s.PersonRepo.Update(ctx, person); err != nil {
		return nil, fmt.Errorf("decision: sync approved child person: %w", err)
	}

	// Re-derive the school_class exactly like the rollover approval path:
	// the concrete class carried by the edit wins; otherwise a bare grade
	// placeholder tracks a grade change ("1" -> "2"), a concrete class whose
	// grade still matches is kept ("3b" at grade 3), and a concrete class
	// stranded on a now-stale grade ("2a" while the edit bumps to grade 3)
	// falls back to the new bare grade rather than leaving the student in a
	// mismatched class. Issue #1833.
	previousSchoolClass := student.SchoolClass
	student.SchoolClass = s.resolveRolloverSchoolClass(child, student.SchoolClass)
	guardianEmail := strings.TrimSpace(strings.ToLower(req.GuardianEmail))
	if guardianEmail != "" {
		student.GuardianEmail = &guardianEmail
	}
	student.GuardianPhone = req.GuardianPhone
	if err := s.StudentRepo.Update(ctx, student); err != nil {
		return nil, fmt.Errorf("decision: sync approved child student: %w", err)
	}

	// A confirmed edit can move the child into another Jahrgang, exactly like
	// a direct school_class edit or a grade transition, so the Jahrgang-
	// filtered offering-sourced Regeltermine and their materialized future
	// occurrences must follow in the same transaction (#2147 review round 13).
	// The recurrence gate is already held — taken with the shared class-writes
	// gate before the first row lock above.
	if student.SchoolClass != previousSchoolClass {
		if err := s.ResyncOfferingSourcedTemplates(ctx, timezone.TodayDate()); err != nil {
			return nil, fmt.Errorf("decision: resync sourced templates after class change: %w", err)
		}
	}

	guardianRequest := req
	if s.GuardianProfileRepo != nil {
		guardianRequest, err = s.guardianIdentityRequest(ctx, req)
		if err != nil {
			return nil, err
		}
	}

	var guardian *users.GuardianProfile
	if input.ReplaceTargetedData && s.GuardianProfileRepo != nil {
		guardian, err = s.reconcilePrimaryGuardianLink(ctx, guardianRequest, student.ID, true)
		if err != nil {
			return nil, err
		}
	}

	keepGuardianProfileIDs := map[int64]bool{}
	if input.ReplaceTargetedData {
		var relinkErr error
		keepGuardianProfileIDs, relinkErr = s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians)
		if relinkErr != nil {
			return nil, relinkErr
		}
	}

	if s.GuardianProfileRepo != nil {
		if guardian == nil {
			resolved, _, gerr := s.resolveGuardianProfile(ctx, guardianRequest)
			if gerr == nil {
				guardian = resolved
			}
		}
		if guardian != nil {
			beforeTargetedSync := *student
			planSynced, terr := s.applyTargetedFields(ctx, req, child, student, guardian, input.ActorAccountID, targetedFieldSyncOptions{
				Replace:                input.ReplaceTargetedData,
				PreviousSnapshot:       input.PreviousSnapshot,
				KeepGuardianProfileIDs: keepGuardianProfileIDs,
			})
			if terr != nil {
				s.Logger.Warn("decision: approved child targeted-field sync had errors",
					slog.Int64("request_id", req.ID),
					slog.Int64("child_id", child.ID),
					slog.String("error", terr.Error()),
				)
				// The two companion sentinels are NOT best-effort field noise:
				// they mean the departure-plan sync was REFUSED, either because
				// it would strand a linked child without an allowed Heimweg or
				// because another editor holds the linked child's row. Swallowing
				// them outside the replacement path (as every other field error is
				// swallowed) would report success for a correction that never
				// landed. Propagate so the tenant transaction rolls back and the
				// handler answers with the actionable 400/409 the student PUT
				// gives (#1694).
				if input.ReplaceTargetedData {
					return nil, fmt.Errorf("decision: approved child targeted-field replacement sync: %w", terr)
				}
				if errors.Is(terr, users.ErrCompanionWouldLoseDeparture) ||
					errors.Is(terr, users.ErrCompanionLockBusy) {
					return nil, fmt.Errorf("decision: approved child departure sync refused: %w", terr)
				}
			}
			if s.StudentAudit != nil {
				afterTargetedSync := student
				if terr != nil {
					persistedStudent, reloadErr := s.StudentRepo.FindByID(ctx, student.ID)
					if reloadErr != nil {
						return nil, fmt.Errorf("decision: reload approved child after partial targeted-field sync: %w", reloadErr)
					}
					afterTargetedSync = persistedStudent
				}
				if auditErr := s.StudentAudit.RecordChangesForActor(
					ctx,
					&beforeTargetedSync,
					afterTargetedSync,
					input.ActorAccountID,
				); auditErr != nil {
					return nil, fmt.Errorf("decision: audit approved child targeted-field sync: %w", auditErr)
				}
			}
			// A departure-plan sync that actually TRIMMED a link changed rows on
			// ANOTHER child's card too. Announce it exactly like the student PUT,
			// care-request and master-data-review writers do, or open detail cards
			// and the Laufgemeinschaft search keep showing the pre-sync links.
			// After the refusal returns above, so a rolled-back sync stays silent.
			if planSynced {
				s.deferStudentPlanBroadcasts(ctx, student.ID)
			}
		}
	}
	if input.ReplaceTargetedData {
		if _, relinkErr := s.reconcileApprovedChildGuardians(ctx, req, student.ID, input.PreviousRequestGuardians); relinkErr != nil {
			return nil, relinkErr
		}
	}

	return s.RequestChildRepo.FindByID(ctx, child.ID)
}

// deferStudentPlanBroadcasts announces, after the surrounding tenant
// transaction commits, that an enrollment sync replaced a child's departure
// plan. Two events, because they invalidate different caches — exactly like
// the master-data-review and care-request emitters:
//
//   - student_updated: the plan itself lives on the student record, so the
//     student detail and database caches that feed an OPEN staff editor must
//     refetch. Without it the editor keeps the pre-sync plan and resubmits it
//     whole on the next unrelated save (an address edit), reverting the
//     approved change.
//   - student_companions_changed: a narrowed plan makes the repository trim
//     "läuft mit" links, which are rows on ANOTHER child's card too — the
//     signal every mounted companion view refetches on.
//
// Fire-and-forget: a lost event costs a stale card, never data.
func (s *decisionService) deferStudentPlanBroadcasts(ctx context.Context, studentID int64) {
	if s.Broadcaster == nil {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		source := "enrollment_sync"
		studentEvent := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
		if err := s.Broadcaster.BroadcastToTenant(tenantID, studentEvent); err != nil {
			s.Logger.Warn("decision: failed to broadcast student update",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
		companionEvent := realtime.NewEvent(realtime.EventStudentCompanionsChanged, "", realtime.EventData{Source: &source})
		if err := s.Broadcaster.BroadcastToTenant(tenantID, companionEvent); err != nil {
			s.Logger.Warn("decision: failed to broadcast student companions change",
				slog.Int64("tenant_id", tenantID),
				slog.Int64("student_id", studentID),
				slog.String("error", err.Error()),
			)
		}
	})
}

func (s *decisionService) actorSnapshot(ctx context.Context, accountID int64) (*string, *string) {
	var name *string
	if s.PersonRepo != nil {
		if person, err := s.PersonRepo.FindByAccountID(ctx, accountID); err == nil && person != nil {
			fullName := strings.TrimSpace(person.GetFullName())
			if fullName != "" {
				name = &fullName
			}
		}
	}
	var email *string
	if s.AccountRepo != nil {
		if account, err := s.AccountRepo.FindByID(ctx, accountID); err == nil && account != nil && strings.TrimSpace(account.Email) != "" {
			value := account.Email
			email = &value
		}
	}
	return name, email
}

func adjustmentSnapshotJSON(links []*enrollmentModels.RequestChildOffering, offeringByID map[int64]*enrollmentModels.CareOffering) ([]byte, error) {
	rows := make([]offeringAdjustmentSnapshot, 0, len(links))
	for _, link := range links {
		if link == nil {
			continue
		}
		row := offeringAdjustmentSnapshot{
			OfferingID:            strconv.FormatInt(link.CareOfferingID, 10),
			SelectedDays:          copyDays(link.SelectedDays),
			ManualSelectedDays:    copyDays(link.ManualSelectedDays),
			AutomaticSelectedDays: copyDays(link.AutomaticSelectedDays),
		}
		if offering := offeringByID[link.CareOfferingID]; offering != nil {
			row.OfferingName = offering.Name
			row.DaysOfWeekMode = offering.DaysOfWeekMode
			row.AvailableDays = copyDays(offering.AvailableDays)
		}
		rows = append(rows, row)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		return lessNumericString(rows[i].OfferingID, rows[j].OfferingID)
	})
	raw, err := json.Marshal(rows)
	if err != nil {
		return nil, fmt.Errorf("decision: marshal offering adjustment snapshot: %w", err)
	}
	return raw, nil
}

func lessNumericString(left, right string) bool {
	leftID, leftErr := strconv.ParseInt(left, 10, 64)
	rightID, rightErr := strconv.ParseInt(right, 10, 64)
	if leftErr == nil && rightErr == nil {
		return leftID < rightID
	}
	return left < right
}

func offeringIDsFromLinks(links []*enrollmentModels.RequestChildOffering) []int64 {
	ids := make([]int64, 0, len(links))
	seen := make(map[int64]bool, len(links))
	for _, link := range links {
		if link == nil || link.CareOfferingID <= 0 || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	return ids
}

func (s *decisionService) rematerializeAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	beforeLinks []*enrollmentModels.RequestChildOffering,
	replacement []*enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
	effectiveFrom *timezone.Date,
) error {
	if s.StudentEnrollmentRepo == nil {
		return nil
	}
	if err := s.lockTemplateRecurrence(ctx); err != nil {
		return err
	}
	if len(beforeLinks) > 0 {
		if err := s.backfillLegacyAdjustedEnrollments(ctx, requestChildID, studentID, beforeLinks); err != nil {
			return err
		}
	}
	if effectiveFrom != nil {
		return s.splitAdjustedEnrollments(ctx, requestChildID, studentID, replacement, phase, *effectiveFrom)
	}
	previousGroups, err := s.enrollmentGroupIDsForRequestChild(ctx, studentID, requestChildID)
	if err != nil {
		return err
	}
	if _, err := s.StudentEnrollmentRepo.DeleteByEnrollmentRequestChild(ctx, studentID, requestChildID); err != nil {
		return fmt.Errorf("decision: delete sourced adjusted enrollments: %w", err)
	}
	if err := s.materializeEnrollments(ctx, requestChildID, studentID, phase); err != nil {
		return err
	}
	// materializeEnrollments reconciled the templates the NEW selection plans;
	// templates only the OLD selection planned still carry the child on
	// already-materialized future occurrences (#2147 review).
	return s.reconcileEnrollmentInstanceRosters(ctx, studentID, previousGroups, enrollmentRewriteBoundary(nil))
}

// enrollmentGroupIDsForRequestChild returns the activity groups the child's
// tagged enrollment rows currently reference, so a full rematerialization can
// reconcile the occurrences of templates the new selection no longer plans.
func (s *decisionService) enrollmentGroupIDsForRequestChild(
	ctx context.Context,
	studentID, requestChildID int64,
) (map[int64]bool, error) {
	rows, err := s.StudentEnrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("decision: list enrollments for adjustment reconcile: %w", err)
	}
	groupIDs := make(map[int64]bool, len(rows))
	for _, row := range rows {
		if row == nil || row.EnrollmentRequestChildID == nil || *row.EnrollmentRequestChildID != requestChildID {
			continue
		}
		groupIDs[row.ActivityGroupID] = true
	}
	return groupIDs, nil
}

// splitAdjustedEnrollments applies the new offering selection from
// effectiveFrom onward without rewriting the past. Deleting and re-creating
// the whole phase window (what an admin correction does) would erase the
// record of which groups the child actually attended before the switch, and
// the attendance taken there would no longer have a roster row behind it.
//
// Per existing row materialized from this request child:
//   - already ended before the switch: untouched, it is history
//   - identical to a row the new selection wants: kept, so an unchanged
//     offering does not become two adjacent rows
//   - starts on/after the switch: deleted, it never took effect
//   - started earlier: capped at effectiveFrom (valid_until is exclusive, so
//     the last attended day is the day before)
//
// Rows the new selection still wants are then materialized starting at the
// switch date. Phase-window edits are deliberately not reconciled here: a
// kept row keeps its original valid_until, and correcting phase dates is what
// the undated (correction) path is for.
func (s *decisionService) splitAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	replacement []*enrollmentModels.RequestChildOffering,
	phase *enrollmentModels.Phase,
	effectiveFrom timezone.Date,
) error {
	drafts, multiSource, err := s.careEnrollmentDraftsForLinks(ctx, requestChildID, studentID, replacement, phase)
	if err != nil {
		return err
	}
	existing, err := s.StudentEnrollmentRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return fmt.Errorf("decision: list enrollments for dated adjustment: %w", err)
	}
	// Every template the switch touches — rows it caps or deletes as well as
	// rows it creates — must afterwards be reconciled onto its already-
	// materialized future occurrences (#2147 review).
	affectedGroups := draftGroupIDSet(drafts)
	for _, row := range existing {
		if row != nil && row.EnrollmentRequestChildID != nil && *row.EnrollmentRequestChildID == requestChildID {
			affectedGroups[row.ActivityGroupID] = true
		}
		if err := s.reconcileAdjustedEnrollment(ctx, row, requestChildID, phase, effectiveFrom, drafts); err != nil {
			return err
		}
	}
	if err := s.persistCareEnrollmentDrafts(ctx, requestChildID, studentID, phase, drafts, &effectiveFrom); err != nil {
		return err
	}
	if err := s.reconcileEnrollmentInstanceRosters(ctx, studentID, affectedGroups, enrollmentRewriteBoundary(&effectiveFrom)); err != nil {
		return err
	}
	// Multi-source templates were deliberately not drafted (see
	// addSourcedTemplateDrafts); the resync re-establishes the child's union
	// coverage from the switch date onward — including templates the child
	// keeps through ANOTHER of the template's source offerings after leaving
	// this one. Scoped to this child: a dated switch must not reconcile other
	// children's rows as a side effect.
	return s.resyncMultiSourceTemplates(ctx, multiSource, enrollmentRewriteBoundary(&effectiveFrom), []int64{requestChildID})
}

func (s *decisionService) reconcileAdjustedEnrollment(
	ctx context.Context,
	row *activities.StudentEnrollment,
	requestChildID int64,
	phase *enrollmentModels.Phase,
	effectiveFrom timezone.Date,
	drafts map[int64]*careEnrollmentDraft,
) error {
	if row == nil || row.EnrollmentRequestChildID == nil || *row.EnrollmentRequestChildID != requestChildID {
		return nil
	}
	if row.ValidUntil != nil && !row.ValidUntil.After(effectiveFrom) {
		return nil
	}
	if draft := drafts[row.ActivityGroupID]; draft != nil &&
		!row.ValidFrom.After(effectiveFrom) &&
		(row.ValidUntil == nil || row.ValidUntil.After(effectiveFrom)) &&
		careDraftMatchesEnrollment(draft, row) {
		// The retained row is only ever extended to the end the draft itself
		// may reach — the phase end, clamped by the sourced segment's envelope
		// and the offering-link window (#2147 review). Extending a capped
		// split predecessor back to the phase end would restore coverage past
		// the split and overlap its successor.
		draftEndExclusive := careDraftValidUntil(draft, phase)
		if row.ValidUntil != nil && row.ValidUntil.Before(draftEndExclusive) {
			if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, draftEndExclusive); err != nil {
				return fmt.Errorf("decision: extend retained adjusted enrollment: %w", err)
			}
		}
		delete(drafts, row.ActivityGroupID)
		return nil
	}
	if !row.ValidFrom.Before(effectiveFrom) {
		if err := s.StudentEnrollmentRepo.Delete(ctx, row.ID); err != nil {
			return fmt.Errorf("decision: delete not-yet-effective adjusted enrollment: %w", err)
		}
		return nil
	}
	if err := s.StudentEnrollmentRepo.SetValidUntilByID(ctx, row.ID, effectiveFrom); err != nil {
		return fmt.Errorf("decision: cap adjusted enrollment: %w", err)
	}
	return nil
}

// careDraftMatchesEnrollment reports whether an existing row already is what
// the new selection asks for, so the switch can leave it alone. Weekday sets
// are compared as sets; an empty stored set means every weekday, which is how
// materialization writes an all-days draft.
func careDraftMatchesEnrollment(draft *careEnrollmentDraft, row *activities.StudentEnrollment) bool {
	if !sameOptionalInt64(draft.calendarPeriodID, row.CalendarPeriodID) {
		return false
	}
	if draft.allWeekdays || len(draft.selectedWeekday) == 0 {
		return len(row.SelectedWeekdays) == 0
	}
	if len(row.SelectedWeekdays) != len(draft.selectedWeekday) {
		return false
	}
	for _, weekday := range row.SelectedWeekdays {
		if !draft.selectedWeekday[weekday] {
			return false
		}
	}
	return true
}

func (s *decisionService) backfillLegacyAdjustedEnrollments(
	ctx context.Context,
	requestChildID, studentID int64,
	beforeLinks []*enrollmentModels.RequestChildOffering,
) error {
	if s.CareOfferingRepo == nil {
		return nil
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDsFromLinks(beforeLinks))
	if err != nil {
		return fmt.Errorf("decision: list existing child offerings for legacy enrollment cleanup: %w", err)
	}
	groupIDs := make([]int64, 0, len(offerings))
	seen := make(map[int64]bool, len(offerings))
	for _, offering := range offerings {
		if offering == nil || offering.ActivityGroupID == nil || *offering.ActivityGroupID <= 0 || seen[*offering.ActivityGroupID] {
			continue
		}
		seen[*offering.ActivityGroupID] = true
		groupIDs = append(groupIDs, *offering.ActivityGroupID)
	}
	if len(groupIDs) == 0 {
		return nil
	}
	if _, err := s.StudentEnrollmentRepo.BackfillEnrollmentRequestChildSource(ctx, studentID, requestChildID, groupIDs); err != nil {
		return fmt.Errorf("decision: backfill legacy adjusted enrollments: %w", err)
	}
	return nil
}
