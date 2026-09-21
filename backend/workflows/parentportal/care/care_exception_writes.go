package care

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	activeModels "github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SubmitCareException sets the guardian-authored pickup and/or arrival override
// for a single day. The two times are the COMPLETE desired override for the
// day, mirroring the parents-portal modal (which always prefills both fields
// from the current state): a non-nil time sets that leg, a nil time clears the
// guardian row for that leg. So emptying the pickup field and saving removes the
// pickup override while keeping the arrival one, instead of silently retaining
func (s *Service) SubmitCareExceptionWithReason(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason string) (*CareException, error) {
	trimmedReason := strings.TrimSpace(reason)
	if pickupTime == nil {
		return nil, ErrNoCareException
	}
	if utf8.RuneCountInString(trimmedReason) > 255 {
		return nil, ErrCareExceptionReasonTooLong
	}
	if trimmedReason == "" {
		// Same one-day Abholzeit change as the request path, so it follows the
		// same per-school reason policy — which needs the child's tenant first
		// (#2267, story 28).
		child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
		if err != nil {
			return nil, err
		}
		if s.guardianReasonRequired(ctx, child.TenantID) {
			return nil, ErrCareExceptionReasonRequired
		}
	}
	return s.submitCareException(ctx, accountID, studentID, date, pickupTime, &trimmedReason)
}

func (s *Service) submitCareException(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason *string) (*CareException, error) {
	if pickupTime == nil {
		return nil, ErrNoCareException
	}

	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.TenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return nil, ErrPickupChangeDisabled
	}

	today := s.todayDate()
	if err := checkCareExceptionDate(date, today); err != nil {
		return nil, err
	}
	change := careExceptionChange{accountID: accountID, studentID: studentID, date: date, today: today, pickupTime: pickupTime, reason: reason}
	var result *CareException
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		merged, err := s.writeCareException(txCtx, child.TenantID, change)
		result = merged
		return err
	})
	if txErr != nil {
		// Two submits for the same child+date can race between the find and the
		// insert (e.g. a double-click or two guardians at once); the unique
		// (student_id, exception_date) index makes the loser fail, and Care Plan
		// reports it as a race. Classify it as its own conflict (409) instead of
		// leaking a 500 — and distinct from the staff-override conflict, since the
		// fix differs (reload and retry vs. nothing the parent can do).
		if errors.Is(txErr, careplan.ErrGuardianPickupExceptionRaced) {
			return nil, ErrCareExceptionRaced
		}
		return nil, fmt.Errorf("parent: submit care exception: %w", txErr)
	}

	s.Logger.Info("parent submitted care exception",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.TenantID),
		slog.Bool("has_pickup", pickupTime != nil),
		slog.Bool("has_arrival", false),
	)
	return result, nil
}

// checkCareExceptionDate refuses a past day and one beyond two calendar
// months, mirroring the parent-portal list window (parseSickDayRange) so a
// created entry can never fall outside the range the UI shows.
func checkCareExceptionDate(date, today timezone.Date) error {
	if date.Before(today) {
		return ErrPastCareDate
	}
	maxDate := timezone.NewDate(today.Year(), today.Month()+2, today.Day())
	if date.After(maxDate) {
		return ErrCareDateTooFar
	}
	return nil
}

// careExceptionChange is one guardian pickup change for a day.
type careExceptionChange struct {
	accountID  int64
	studentID  int64
	date       timezone.Date
	today      timezone.Date
	pickupTime *time.Time
	reason     *string
}

// writeCareException applies the change inside the child's tenant unit of
// work: the student row and the care day are locked before any check, Care
// Plan writes the guardian leg and keeps the derived excusal in step, and the
// merged day is read back for the response.
func (s *Service) writeCareException(ctx context.Context, tenantID int64, change careExceptionChange) (*CareException, error) {
	if err := s.lockCareDay(ctx, change.studentID, change.date); err != nil {
		return nil, err
	}
	if err := s.guardCareExceptionDay(ctx, tenantID, change); err != nil {
		return nil, err
	}
	// Parent-set day pickup times couple with the per-block excusal the same
	// way staff-set ones do (#2360): a pull-forward against the weekly
	// baseline excuses the blocks after the new time; moving it back releases
	// them again. Care Plan does both in the same unit of work.
	err := s.GuardianPickups.ApplyGuardianPickupException(ctx, careplan.GuardianPickupChange{
		TenantID: tenantID, StudentID: change.studentID, GuardianAccountID: change.accountID,
		Date: careplan.Date(change.date), PickupTime: change.pickupTime, Reason: change.reason,
	})
	if errors.Is(err, careplan.ErrPickupExceptionStaffOwned) {
		return nil, ErrCareExceptionConflict
	}
	if err != nil {
		return nil, err
	}
	merged, err := s.loadCareException(ctx, change.studentID, change.date)
	if err != nil {
		return nil, err
	}
	pillBody := CareExceptionEventBody(change.date, change.pickupTime, nil)
	pillRefTable, pillRefID := s.careExceptionRef(ctx, change.studentID, change.date)
	tenant.RegisterAfterCommit(ctx, func() {
		s.SelfService.EmitSelfServicePill(tenantID, change.studentID, change.accountID, "care_exception", pillBody, pillRefTable, pillRefID)
		s.broadcastStudentUpdated(tenantID, change.studentID)
		// Fan out to EVERY guardian so a co-guardian's open tab reflects the
		// new override on the "Heute" tile live (#1725 review).
		s.SelfService.WakeChildGuardians(tenantID, change.studentID)
	})
	return merged, nil
}

// lockCareDay locks the child's row, re-checks the care interval under that
// lock, and takes the care-day lock every day-exception writer shares.
func (s *Service) lockCareDay(ctx context.Context, studentID int64, date timezone.Date) error {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return ErrChildCareEnded
	}
	return s.CareExceptions.LockStudentAndExceptionDay(ctx, studentID, date.String())
}

// guardCareExceptionDay refuses a day the child has already left, a day the
// school owns, and a change the school's pickup policy closes.
func (s *Service) guardCareExceptionDay(ctx context.Context, tenantID int64, change careExceptionChange) error {
	alreadyLeft, err := s.childAlreadyLeftToday(ctx, change.studentID, change.date, change.today)
	if err != nil {
		return err
	}
	if alreadyLeft {
		return ErrCareExceptionAlreadyLeft
	}
	staffOwned, err := s.pickupHasStaffException(ctx, change.studentID, change.date)
	if err != nil {
		return err
	}
	if staffOwned {
		return ErrCareExceptionConflict
	}
	policy, err := s.pickupChangePolicyInTx(ctx, tenantID)
	if err != nil {
		return err
	}
	if !policy.enabled {
		return ErrPickupChangeDisabled
	}
	if policy.cutoff.Closed(change.date) {
		return ErrPickupChangeCutoffPassed
	}
	return nil
}

func (s *Service) childAlreadyLeftToday(ctx context.Context, studentID int64, date, today timezone.Date) (bool, error) {
	if date != today || s.Attendance == nil {
		return false, nil
	}
	rows, err := s.Attendance.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{studentID}, FromDate: date.String(), UntilDate: date.String()})
	if err != nil {
		return false, err
	}
	return AttendanceRowsShowLeft(rows), nil
}

// Only the PICKUP leg of the day can block a parent: since arrival times became
// OGS-only, a staff-set Bringzeit says nothing about who owns the Abholzeit, and
// treating it as a conflict would let one OGS entry silently forbid every parent
// pickup change for that day (TestSubmitCareExceptionWithReasonPreservesExistingArrival).
// An AUTO-derived partial absence is the school's own bookkeeping, not a
// decision, so only a manual one counts (#2360).
func (s *Service) pickupHasStaffException(ctx context.Context, studentID int64, date timezone.Date) (bool, error) {
	pickup, err := s.pickupExceptionForDate(ctx, studentID, careplan.Date(date))
	if err != nil {
		return false, err
	}
	return pickup != nil && (pickup.Source == scheduleModels.ExceptionSourceStaff || pickup.HasManualPartialAbsence()), nil
}

// DeleteCareException removes only the guardian-authored pickup exception for
// the date. Arrival and staff rows are left untouched. Deleting a day with
// nothing guardian-owned
// is a no-op (and skips the broadcast). Not gated by the feature toggle for the
// same reason as ListCareExceptions: clearing one's own override stays available.
func (s *Service) DeleteCareException(ctx context.Context, accountID, studentID int64, date timezone.Date) error {
	child, err := s.ResolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.RequireCareRunning(); err != nil {
		return err
	}

	today := s.todayDate()
	if date.Before(today) {
		return ErrPastCareDate
	}
	change := careExceptionChange{accountID: accountID, studentID: studentID, date: date, today: today}
	txErr := InTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		return s.withdrawCareException(txCtx, child.TenantID, change)
	})
	if txErr != nil {
		return fmt.Errorf("parent: delete care exception: %w", txErr)
	}
	return nil
}

// withdrawCareException removes the guardian's pickup row of the day. Removing
// an approved exception takes effect at once, without a staff decision, so
// after the school's cutoff it is closed for today like every other guardian
// pickup write (#3163).
func (s *Service) withdrawCareException(ctx context.Context, tenantID int64, change careExceptionChange) error {
	if err := s.lockCareDay(ctx, change.studentID, change.date); err != nil {
		return err
	}
	alreadyLeft, err := s.childAlreadyLeftToday(ctx, change.studentID, change.date, change.today)
	if err != nil {
		return err
	}
	if alreadyLeft {
		return ErrCareExceptionAlreadyLeft
	}
	pickup, err := s.pickupExceptionForDate(ctx, change.studentID, careplan.Date(change.date))
	if err != nil || pickup == nil {
		return err
	}
	if pickup.HasManualPartialAbsence() {
		return ErrCareExceptionConflict
	}
	if pickup.Source != scheduleModels.ExceptionSourceGuardian {
		return nil
	}
	cutoff, err := s.pickupChangeCutoffInTx(ctx, tenantID)
	if err != nil {
		return err
	}
	if cutoff.Closed(change.date) {
		return ErrPickupChangeCutoffPassed
	}
	// An auto-derived excusal follows the pickup time: Care Plan releases the
	// excused blocks before the row is removed (#2360).
	if err := s.GuardianPickups.WithdrawGuardianPickupException(ctx, *pickup); err != nil {
		return err
	}
	pillBody := "Korrektur: Abholung " + change.date.Format("02.01.") + " zurückgezogen"
	tenant.RegisterAfterCommit(ctx, func() {
		s.SelfService.EmitSelfServicePill(tenantID, change.studentID, change.accountID, "care_exception_correction", pillBody, "", nil)
		s.broadcastStudentUpdated(tenantID, change.studentID)
		// Fan out to EVERY guardian so a co-guardian's open tab drops the
		// removed override on the "Heute" tile live (#1725 review).
		s.SelfService.WakeChildGuardians(tenantID, change.studentID)
	})
	return nil
}

// broadcastStudentUpdated fires a tenant-scoped student_updated event so
// supervisors' live views refresh after a parent-side change. Mirrors the
// staff handler's broadcast; fire-and-forget.
func (s *Service) broadcastStudentUpdated(tenantID, studentID int64) {
	if s.StudentUpdates == nil || tenantID <= 0 {
		return
	}
	if err := s.StudentUpdates.StudentUpdated(tenantID, activeModels.StudentStatusSourceParent); err != nil {
		s.Logger.Warn("parent: failed to broadcast student update",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
	}
}

// careExceptionRef resolves the pill's backreference to the exception row the
// submission wrote (pickup preferred, else arrival). Best-effort: a lookup
// failure just leaves the pill without a ref.
func (s *Service) careExceptionRef(ctx context.Context, studentID int64, date timezone.Date) (string, *int64) {
	pickup, err := s.pickupExceptionForDate(ctx, studentID, careplan.Date(date))
	if err == nil && pickup != nil {
		id := pickup.ID
		return "schedule.student_pickup_exceptions", &id
	}
	arrival, err := s.arrivalExceptionForDate(ctx, studentID, careplan.Date(date))
	if err == nil && arrival != nil {
		id := arrival.ID
		return "schedule.student_arrival_exceptions", &id
	}
	return "", nil
}
