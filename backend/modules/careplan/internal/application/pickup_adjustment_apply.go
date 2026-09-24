package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Apply applies a previewed pickup adjustment in the tenant transaction. The
// preview is recomputed under the student lock and must match the token the
// caller previewed.
func (s *PickupAdjustments) Apply(ctx context.Context, input careplan.PickupAdjustmentApplyInput) (*careplan.PickupAdjustmentResult, error) {
	today := s.today()
	normalized, _, err := normalizePickupAdjustmentInput(input.PickupAdjustmentPreviewInput, today)
	if err != nil {
		return nil, err
	}
	input.PickupAdjustmentPreviewInput = normalized
	resolution, err := pickupAdjustmentResolution(input, today)
	if err != nil {
		return nil, err
	}
	var result *careplan.PickupAdjustmentResult
	err = s.deps.UnitOfWork.RunInTenantTx(ctx, func(txCtx context.Context) error {
		var applyErr error
		result, applyErr = s.applyPickupAdjustment(txCtx, input, resolution, today)
		return applyErr
	})
	if err != nil {
		s.deps.UnitOfWork.MarkRollback(ctx)
	}
	return result, err
}

func pickupAdjustmentResolution(input careplan.PickupAdjustmentApplyInput, today calendar.Date) (string, error) {
	resolution := strings.TrimSpace(input.Resolution)
	if resolution == "" {
		resolution = careplan.PickupAdjustmentResolutionException
	}
	if resolution != careplan.PickupAdjustmentResolutionException && resolution != careplan.PickupAdjustmentResolutionOffering {
		return "", fmt.Errorf("%w: unknown resolution %q", careplan.ErrPickupAdjustmentInvalid, resolution)
	}
	if resolution == careplan.PickupAdjustmentResolutionException && input.EffectiveFrom.After(today) {
		return "", fmt.Errorf("%w: lasting exceptions take effect immediately", careplan.ErrPickupAdjustmentInvalid)
	}
	return resolution, nil
}

func (s *PickupAdjustments) applyPickupAdjustment(
	ctx context.Context,
	input careplan.PickupAdjustmentApplyInput,
	resolution string,
	today calendar.Date,
) (*careplan.PickupAdjustmentResult, error) {
	if resolution == careplan.PickupAdjustmentResolutionOffering {
		if err := s.deps.Offerings.PrepareDirectOfferingAdjustment(ctx, directOfferingInput(input)); err != nil {
			return nil, err
		}
	}
	if err := s.lockAndAuthorizePickupStudent(ctx, input); err != nil {
		return nil, err
	}
	if resolution == careplan.PickupAdjustmentResolutionOffering {
		if err := s.preflightPickupOffering(ctx, input.StudentID, input.EffectiveFrom, today); err != nil {
			return nil, err
		}
	}
	preview, err := s.preview(ctx, input.PickupAdjustmentPreviewInput, today)
	if err != nil {
		return nil, err
	}
	if !samePickupAdjustmentToken(input.PreviewToken, preview.PreviewToken) {
		return nil, careplan.ErrPickupAdjustmentStale
	}
	if preview.ResolutionRequired && strings.TrimSpace(input.Resolution) == "" {
		return nil, careplan.ErrPickupAdjustmentResolutionRequired
	}
	if resolution == careplan.PickupAdjustmentResolutionOffering {
		err = s.applyPickupOffering(ctx, input, preview)
	} else {
		err = s.applyPickupException(ctx, input, preview)
	}
	if err != nil {
		return nil, err
	}
	if err := s.applyPickupArrivalSchedules(ctx, input, resolution, today); err != nil {
		return nil, err
	}
	return &careplan.PickupAdjustmentResult{Resolution: resolution}, nil
}

// applyPickupArrivalSchedules writes the arrival rows of a lasting exception
// that takes effect today. Arrival rows have no effective date, so an
// offering switch leaves them to the booking projection.
func (s *PickupAdjustments) applyPickupArrivalSchedules(
	ctx context.Context,
	input careplan.PickupAdjustmentApplyInput,
	resolution string,
	today calendar.Date,
) error {
	if input.ArrivalSchedules == nil || !appliesArrivalSchedulesOn(resolution, input.EffectiveFrom, today) {
		return nil
	}
	rows, err := pickupArrivalScheduleRows(input.StudentID, input.CreatedByStaffID, *input.ArrivalSchedules)
	if err != nil {
		return err
	}
	if err := s.deps.ArrivalSchedules.UpsertBulkStudentArrivalSchedules(ctx, input.StudentID, rows); err != nil {
		return fmt.Errorf("pickup adjustment: update arrival schedules: %w", err)
	}
	return nil
}

func appliesArrivalSchedulesOn(resolution string, effectiveFrom, today calendar.Date) bool {
	return resolution == careplan.PickupAdjustmentResolutionException && !effectiveFrom.After(today)
}

func (s *PickupAdjustments) lockAndAuthorizePickupStudent(ctx context.Context, input careplan.PickupAdjustmentApplyInput) error {
	// A student that cannot be read under lock is reported as missing.
	student, err := s.deps.Students.LockStudent(ctx, input.StudentID)
	if err != nil || student == nil {
		return careplan.ErrPickupAdjustmentStudentNotFound
	}
	if input.Authorize == nil {
		return nil
	}
	allowed, err := input.Authorize(ctx, *student)
	if err != nil || !allowed {
		return careplan.ErrPickupAdjustmentUnauthorized
	}
	return nil
}

func directOfferingInput(input careplan.PickupAdjustmentApplyInput) careplan.DirectOfferingAdjustmentInput {
	return careplan.DirectOfferingAdjustmentInput{
		StudentID: input.StudentID, EffectiveFrom: input.EffectiveFrom,
		Selections: input.Selections, ExcludedAutoOfferingIDs: input.ExcludedAutoOfferingIDs,
		Reason: input.Reason, ActorAccountID: input.ActorAccountID, ActorRole: input.ActorRole,
		CompleteWithdrawalConfirmed: input.CompleteWithdrawalConfirmed,
	}
}

func (s *PickupAdjustments) applyPickupException(ctx context.Context, input careplan.PickupAdjustmentApplyInput, preview *careplan.PickupAdjustmentPreview) error {
	rows, err := pickupScheduleRows(input.StudentID, input.CreatedByStaffID, input.Schedules)
	if err != nil {
		return err
	}
	if err := s.deps.PickupSchedules.UpsertBulkStudentPickupSchedulesForDate(ctx, input.StudentID, input.EffectiveFrom, rows); err != nil {
		return err
	}
	if preview.CurrentPlan == preview.ProposedPlan {
		return nil
	}
	return s.deps.Audit.RecordPickupPlanForActor(
		ctx, input.StudentID, preview.CurrentPlan, preview.ProposedPlan,
		"Dauerhafte Ausnahme", input.Reason, input.ActorAccountID,
	)
}

func (s *PickupAdjustments) applyPickupOffering(ctx context.Context, input careplan.PickupAdjustmentApplyInput, preview *careplan.PickupAdjustmentPreview) error {
	if len(input.Selections) == 0 || preview.OfferingConsequences == nil {
		return fmt.Errorf("%w: an offering selection preview is required", careplan.ErrPickupAdjustmentInvalid)
	}
	if !selectsExactPickupOffering(input.Selections, preview.MatchingOfferings) {
		return fmt.Errorf("%w: selected care offering does not match the pickup plan", careplan.ErrPickupAdjustmentInvalid)
	}
	if err := s.deps.Offerings.ApplyDirectOfferingAdjustment(ctx, directOfferingInput(input)); err != nil {
		return err
	}
	if err := s.deleteManualPickupSchedules(ctx, input.StudentID); err != nil {
		return err
	}
	return s.deps.Audit.RecordPickupPlanForActor(
		ctx, input.StudentID, preview.CurrentPlan, preview.ProposedPlan,
		"Angebot geändert", input.Reason, input.ActorAccountID,
	)
}

// deleteManualPickupSchedules removes the manual weekly rows. The student
// row is locked by Apply and every pickup-plan writer takes the same lock,
// so this is exactly the manual plan the preview described.
func (s *PickupAdjustments) deleteManualPickupSchedules(ctx context.Context, studentID int64) error {
	existing, err := s.pickupRows(ctx, studentID)
	if err != nil {
		return err
	}
	for _, row := range existing {
		if row.ID > 0 && row.Source != careplan.ScheduleSourceCareOffering {
			if err := s.deps.PickupSchedules.DeleteStudentPickupSchedule(ctx, row.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

// ApplyBulkExceptions writes a lasting pickup exception for several
// children. While the school reviews deviations from the booked offering,
// it needs an explicit confirmation and records each changed plan.
func (s *PickupAdjustments) ApplyBulkExceptions(ctx context.Context, input careplan.PickupAdjustmentBulkInput) (*careplan.BulkUpsertResult, error) {
	today := s.today()
	reviewEnabled, err := s.deps.Settings.PickupOfferingReviewRequired(ctx)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: resolve offering review setting: %w", err)
	}
	filter := careplan.PickupBulkFilter{StudentIDs: input.StudentIDs, Authorize: input.Authorize}
	if !reviewEnabled {
		result, applyErr := s.deps.PickupSchedules.BulkUpsertPickupSchedules(ctx, filter, input.Schedules, input.CreatedByStaffID)
		if applyErr != nil {
			s.deps.UnitOfWork.MarkRollback(ctx)
		}
		return result, applyErr
	}
	if !input.ConfirmedException {
		return nil, careplan.ErrPickupAdjustmentBulkConfirmation
	}
	var result *careplan.BulkUpsertResult
	err = s.deps.UnitOfWork.RunInTenantTx(ctx, func(txCtx context.Context) error {
		before, err := s.lockAndSnapshotBulkStudents(txCtx, input, today)
		if err != nil {
			return err
		}
		result, err = s.deps.PickupSchedules.BulkUpsertPickupSchedules(txCtx, filter, input.Schedules, input.CreatedByStaffID)
		if err != nil {
			return err
		}
		return s.auditBulkPickupPlans(txCtx, input, result.AffectedStudentIDs, before, today)
	})
	if err != nil {
		s.deps.UnitOfWork.MarkRollback(ctx)
	}
	return result, err
}

func (s *PickupAdjustments) lockAndSnapshotBulkStudents(ctx context.Context, input careplan.PickupAdjustmentBulkInput, today calendar.Date) (map[int64]string, error) {
	studentIDs := slices.Clone(input.StudentIDs)
	slices.Sort(studentIDs)
	students, err := s.deps.Students.LockStudents(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("pickup adjustment: lock students: %w", err)
	}
	for _, studentID := range studentIDs {
		student, ok := students[studentID]
		if !ok {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentNotFound, studentID)
		}
		if input.Authorize == nil {
			continue
		}
		allowed, authorizeErr := input.Authorize(ctx, student)
		if authorizeErr != nil || !allowed {
			return nil, fmt.Errorf("%w: student %d", careplan.ErrBulkStudentUnauthorized, studentID)
		}
	}
	return s.projectPickupPlanLabels(ctx, studentIDs, today)
}

func (s *PickupAdjustments) auditBulkPickupPlans(
	ctx context.Context,
	input careplan.PickupAdjustmentBulkInput,
	studentIDs []int64,
	before map[int64]string,
	today calendar.Date,
) error {
	after, err := s.projectPickupPlanLabels(ctx, studentIDs, today)
	if err != nil {
		return err
	}
	for _, studentID := range studentIDs {
		prior, ok := before[studentID]
		if !ok {
			return errors.New("pickup adjustment: bulk preview is missing")
		}
		if prior == after[studentID] {
			continue
		}
		if err := s.deps.Audit.RecordPickupPlanForActor(
			ctx, studentID, prior, after[studentID], "Dauerhafte Ausnahme", "", input.ActorAccountID,
		); err != nil {
			return err
		}
	}
	return nil
}

func (s *PickupAdjustments) projectPickupPlanLabels(ctx context.Context, studentIDs []int64, today calendar.Date) (map[int64]string, error) {
	weekStart := today.AddDays(pickupWeekdayMonday - isoWeekday(today))
	projection, err := s.deps.Baselines.Project(ctx, studentIDs, weekStart, weekStart.AddDays(4))
	if err != nil {
		return nil, err
	}
	labels := make(map[int64]string, len(studentIDs))
	for _, studentID := range studentIDs {
		labels[studentID] = pickupPlanLabel(projection.WeeklyForDate(studentID, weekStart))
	}
	return labels, nil
}
