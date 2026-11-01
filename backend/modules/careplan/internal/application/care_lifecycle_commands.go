package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *CareLifecycle) Preview(ctx context.Context, input careplan.CareExitInput) (value0 *careplan.CareExitPreview, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitPreview(txCtx, input)
		return err
	})
	return
}

func (s *CareLifecycle) Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (value0 *careplan.CareExitResult, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitConfirm(txCtx, token, input, actorAccountID)
		return err
	})
	return
}

// RecordedExitStudentIDs reduces the exit rows to their bare existence. The
// reason and its note stay here: they are readable with users:delete only,
// while the fact that an exit was recorded travels on the ordinary student
// payload so the list can label exactly those children (#2487).
func (s *CareLifecycle) RecordedExitStudentIDs(ctx context.Context, studentIDs []int64) (value0 map[int64]bool, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitRecordedExitStudentIDs(txCtx, studentIDs)
		return err
	})
	return
}

// ListEnded is the archive. It reads the CHILDREN whose enrolment interval
// has run out rather than the reason rows: the archive holds every regularly
// ended care, including one that ended with an enrolment phase and never got
// a manually recorded reason.
func (s *CareLifecycle) ListEnded(ctx context.Context, filter careplan.EndedCareFilter) (value0 []careplan.EndedCare, value1 int, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.inUnitListEnded(txCtx, filter)
		return err
	})
	return
}

// ParticipatingStudentIDs applies the same durable first-bookingless-day
// boundary to every operational reader. Actual presence always wins: safety
// information must remain visible even when the expected care has ended.
func (s *CareLifecycle) ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on calendar.Date, actuallyPresent map[int64]bool) (value0 map[int64]bool, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitParticipatingStudentIDs(txCtx, studentIDs, on, actuallyPresent)
		return err
	})
	return
}

// ResolveListParticipation owns the candidate, live-presence and dated care
// decision for lists and reports. The caller supplies a frozen today so a
// request crossing midnight cannot combine two calendar days.
func (s *CareLifecycle) ResolveListParticipation(ctx context.Context, studentIDs []int64, on, today calendar.Date, includePending bool) (value0 *careplan.CareParticipationResolution, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitResolveListParticipation(txCtx, studentIDs, on, today, includePending)
		return err
	})
	return
}

func (s *CareLifecycle) ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (value0 map[calendar.Date]map[int64]bool, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitParticipatingStudentIDsByDate(txCtx, studentIDs, from, to)
		return err
	})
	return
}

func (s *CareLifecycle) PreviewBookingAuthorityImpact(ctx context.Context, on calendar.Date) (value0 *careplan.BookingAuthorityImpact, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitPreviewBookingAuthorityImpact(txCtx, on)
		return err
	})
	return
}

// ApplyBookingAuthoritySetting validates and reconciles a mode switch while
// the caller's tenant transaction is still open. The booking-write lock is
// the one booking mutations hold, so a preview made before this call cannot
// bypass the binding recheck.
func (s *CareLifecycle) ApplyBookingAuthoritySetting(ctx context.Context, on calendar.Date, enabled bool) (value0 *careplan.BookingAuthorityImpact, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitApplyBookingAuthoritySetting(txCtx, on, enabled)
		return err
	})
	return
}

func (s *CareLifecycle) ListPendingWithdrawals(ctx context.Context, filter careplan.CareWithdrawalFilter) (value0 []careplan.WithdrawalCompletion, value1 int, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.inUnitListPendingWithdrawals(txCtx, filter)
		return err
	})
	return
}

// ListResolvedWithdrawals keeps deleted completions visible without the
// deleted child's personal data.
func (s *CareLifecycle) ListResolvedWithdrawals(ctx context.Context, filter careplan.CareWithdrawalFilter) (value0 []careplan.WithdrawalCompletion, value1 int, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, value1, err = s.inUnitListResolvedWithdrawals(txCtx, filter)
		return err
	})
	return
}

func (s *CareLifecycle) GetPendingWithdrawal(ctx context.Context, id int64) (value0 careplan.WithdrawalCompletion, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitGetPendingWithdrawal(txCtx, id)
		return err
	})
	return
}

func (s *CareLifecycle) PreviewWithdrawalCareEnd(ctx context.Context, completionID int64, input careplan.CareExitInput) (value0 *careplan.CareExitPreview, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitPreviewWithdrawalCareEnd(txCtx, completionID, input)
		return err
	})
	return
}

func (s *CareLifecycle) ConfirmWithdrawalCareEnd(
	ctx context.Context, completionID int64, token string, input careplan.CareExitInput, actorAccountID int64,
) (value0 *careplan.CareExitResult, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitConfirmWithdrawalCareEnd(txCtx, completionID, token, input, actorAccountID)
		return err
	})
	return
}

// ReconcileAuthoritativeBookingChange keeps the withdrawal tasks in step with
// one booking mutation. In weekly-plan mode only a rebooking can make a task
// obsolete; in booking-led mode the child's bookings are evaluated again.
func (s *CareLifecycle) ReconcileAuthoritativeBookingChange(ctx context.Context, change careplan.CareWithdrawalBookingChange) (err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		err = s.inUnitReconcileAuthoritativeBookingChange(txCtx, change)
		return err
	})
	return
}

func (s *CareLifecycle) HistoricalParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to calendar.Date) (value0 map[calendar.Date]map[int64]bool, err error) {
	err = s.unit(ctx, func(txCtx context.Context) error {
		value0, err = s.inUnitHistoricalParticipatingStudentIDsByDate(txCtx, studentIDs, from, to)
		return err
	})
	return
}
