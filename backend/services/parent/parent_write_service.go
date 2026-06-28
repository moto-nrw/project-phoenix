package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// maxParentNoteLen bounds a single note so a parent can't paste a novel
// the staff card then has to render. Generous for a "kurze Nachricht".
const maxParentNoteLen = 2000

// Sentinel errors the HTTP layer maps to stable status codes. They are
// part of the package contract — handlers switch on them via errors.Is.
var (
	// ErrChildNotLinked means the account is not a guardian of the
	// student. Handlers MUST map this to 403/404 and never leak whether
	// the student exists at another school.
	ErrChildNotLinked = errors.New("parent: child not linked to account")
	// ErrGuardianPermissionDenied means the account is linked to the child but
	// the relationship does not grant the requested parent-portal action.
	ErrGuardianPermissionDenied = errors.New("parent: guardian relationship lacks required permission")
	// ErrSickNoteDisabled means operations.parent_sick_note_enabled is
	// off for the child's tenant.
	ErrSickNoteDisabled = errors.New("parent: sick notes disabled for this school")
	// ErrNotesDisabled means operations.parent_notes_enabled is off for
	// the child's tenant.
	ErrNotesDisabled = errors.New("parent: parent notes disabled for this school")
	// ErrNoDates means the sick-note request carried no dates.
	ErrNoDates = errors.New("parent: at least one date is required")
	// ErrInvalidStatus means the absence status was neither sick nor excused.
	ErrInvalidStatus = errors.New("parent: status must be sick or excused")
	// ErrEmptyNote means the note body was blank after trimming.
	ErrEmptyNote = errors.New("parent: note body must not be empty")
	// ErrNoteTooLong means the note body exceeded maxParentNoteLen.
	ErrNoteTooLong = errors.New("parent: note body too long")
	// ErrPickupChangeDisabled means operations.parent_pickup_change_enabled is
	// off for the child's tenant.
	ErrPickupChangeDisabled = errors.New("parent: pickup-time change disabled for this school")
	// ErrNoCareException means the request carried neither a pickup nor an
	// arrival time.
	ErrNoCareException = errors.New("parent: at least one of pickup or arrival time is required")
	// ErrPastCareDate means the requested date is in the past.
	ErrPastCareDate = errors.New("parent: care exception date must not be in the past")
	// ErrCareDateTooFar means the requested date is beyond the window parents may
	// set (two calendar months ahead, matching the parent-portal list range).
	ErrCareDateTooFar = errors.New("parent: care exception date is too far in the future")
	// ErrCareExceptionConflict means a staff-authored exception already exists
	// for the date, so the parent change is refused rather than overwriting it.
	ErrCareExceptionConflict = errors.New("parent: the school already set a special time for this day")
	// ErrCareExceptionRaced means two submits for the same child+date collided
	// on the unique index (e.g. a double-click); the change was not saved and
	// the caller should reload and retry.
	ErrCareExceptionRaced = errors.New("parent: this day was just changed, please reload and try again")
	// ErrInvalidParentRequestType means the change-request type is not registered.
	ErrInvalidParentRequestType = errors.New("parent: invalid request type")
	// ErrParentRequestNotFound means the request row does not exist in the
	// guardian's conversation for this child.
	ErrParentRequestNotFound = errors.New("parent: request not found")
	// ErrParentRequestNotOpen means the request was already decided/withdrawn and
	// can no longer be withdrawn.
	ErrParentRequestNotOpen = errors.New("parent: request is not open")
)

// resolveOwnedChild validates the account is a guardian of the student
// and returns the child's tenant id. The cross-tenant lookup runs under
// an admin tx; a nil child becomes ErrChildNotLinked so the caller never
// trusts a studentID it can't prove ownership of.
func (s *service) resolveOwnedChild(ctx context.Context, accountID, studentID int64) (*parentChild, error) {
	return s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
}

func (s *service) resolvePermittedChild(ctx context.Context, accountID, studentID int64, requiredPermission string) (*parentChild, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return nil, fmt.Errorf("parent: student_id must be positive")
	}

	var resolved *parentChild
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		child, findErr := s.childRepo.FindForAccount(adminCtx, accountID, studentID)
		if findErr != nil {
			return findErr
		}
		if child == nil {
			return ErrChildNotLinked
		}
		if requiredPermission != "" && !childHasPermission(child, requiredPermission) {
			return ErrGuardianPermissionDenied
		}
		resolved = &parentChild{
			tenantID:            child.TenantID,
			guardianPermissions: child.GuardianPermissions,
			studentName:         strings.TrimSpace(child.FirstName + " " + child.LastName),
			schoolName:          child.SchoolName,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

func childHasPermission(child *parentModels.ChildSummary, permission string) bool {
	if child == nil {
		return false
	}
	return authorize.StudentGuardianHasPermission(&usersModels.StudentGuardian{
		Permissions: child.GuardianPermissions,
	}, permission)
}

// parentChild is the minimal resolved context a per-child write needs.
type parentChild struct {
	tenantID            int64
	guardianPermissions map[string]interface{}
	// studentName / schoolName feed the OGS messaging views (thread counterpart
	// + child label); resolved once here from the cross-tenant child lookup.
	studentName string
	schoolName  string
}

func (c *parentChild) hasPermission(permission string) bool {
	if c == nil {
		return false
	}
	return authorize.StudentGuardianHasPermission(&usersModels.StudentGuardian{
		Permissions: c.guardianPermissions,
	}, permission)
}

// SubmitSickNote reports the child absent for the given dates with the chosen
// status. The status is either StudentStatusDaySick (a "Krankmeldung": flips the
// live sick flag when today is included, exactly as before) or
// StudentStatusDayExcused (a "Termin/Abwesenheit": stored as a planned status day
// with NO live flag, per issue #1735). Krank and entschuldigt stay separate in the
// data so staff see each correctly; only the parent's reason choice picks between
// them. Both share the same dates, reason field and source=parent.
func (s *service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string) ([]*activeModels.StudentStatusDay, error) {
	if len(dates) == 0 {
		return nil, ErrNoDates
	}
	if status != activeModels.StudentStatusDaySick && status != activeModels.StudentStatusDayExcused {
		return nil, ErrInvalidStatus
	}

	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionSickNoteSubmit)
	if err != nil {
		return nil, err
	}

	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if !enabled {
		return nil, ErrSickNoteDisabled
	}

	now := time.Now()
	today := timezone.TodayDate()

	var notePtr *string
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		// Count characters (runes), not UTF-8 bytes, so the limit matches the
		// frontend's maxLength — a German text with umlauts stays under the
		// budget even though it spans more bytes.
		if utf8.RuneCountInString(trimmed) > maxParentNoteLen {
			return nil, ErrNoteTooLong
		}
		notePtr = &trimmed
	}

	var result []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, other := range activeModels.StudentStatusDayStatusesExcept(status) {
			if err := s.statusDayRepo.MarkClearedForDates(txCtx, studentID, other, dates, now, activeModels.StudentStatusSourceParent); err != nil {
				return err
			}
		}
		for _, d := range dates {
			if err := s.statusDayRepo.UpsertReported(txCtx, &activeModels.StudentStatusDay{
				StudentID:  studentID,
				Date:       d,
				Status:     status,
				ReportedAt: now,
				Source:     activeModels.StudentStatusSourceParent,
				Note:       notePtr,
			}); err != nil {
				return err
			}
		}

		if containsDate(dates, today) {
			fresh, err := s.studentRepo.FindByIDForUpdate(txCtx, studentID)
			if err != nil {
				return err
			}
			applyLiveStatusForParentToday(fresh, status, now)
			if err := s.studentRepo.Update(txCtx, fresh); err != nil {
				return err
			}
		}

		rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, minDate(dates), maxDate(dates))
		if err != nil {
			return err
		}
		// Return only the days the parent actually submitted with the chosen
		// status. The range query spans min..max, so for a non-contiguous
		// submission (e.g. Mon + Wed) it can also return an unrelated active row
		// in between (a different status on Tuesday) which must not be surfaced.
		dateSet := make(map[timezone.Date]struct{}, len(dates))
		for _, d := range dates {
			dateSet[d] = struct{}{}
		}
		filtered := make([]*activeModels.StudentStatusDay, 0, len(dates))
		for _, r := range rows {
			if r.Status != status {
				continue
			}
			if _, ok := dateSet[r.Date]; !ok {
				continue
			}
			filtered = append(filtered, r)
		}
		result = filtered

		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastStudentUpdated(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: submit sick note: %w", txErr)
	}

	s.logger.Info("parent submitted absence",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.String("status", status),
		slog.Int("days", len(dates)),
		slog.Bool("has_reason", notePtr != nil),
	)
	return result, nil
}

// ChildFeatures resolves the parent-portal feature toggles for the child's
// tenant after verifying ownership.
func (s *service) ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}
	sick, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	notes, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	pickupChange, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	inviteMode, err := s.settings.ResolveStringForTenant(ctx, child.tenantID, configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	canRemove, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyGuardianParentCanRemove)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	return ChildFeatureFlags{
		SickNoteEnabled:              sick && child.hasPermission(authorize.GuardianPermissionSickNoteSubmit),
		NotesEnabled:                 notes && child.hasPermission(authorize.GuardianPermissionNotesWrite),
		RequestSubmitEnabled:         notes && child.hasPermission(authorize.GuardianPermissionRequestSubmit),
		PickupChangeEnabled:          pickupChange,
		RelatedAccountsInviteEnabled: inviteMode != configModels.ParentInviteModeDisabled,
		RelatedAccountsRemoveEnabled: canRemove && inviteMode != configModels.ParentInviteModeDisabled,
	}, nil
}

// ListSickDays returns the child's active parent-facing absences in [from, to]:
// sick ("Krankmeldung") days plus the parent's own excused ("Termin/Abwesenheit")
// days, so a parent sees every absence they reported. Staff-created excused days
// (source=planned/manual) are an internal scheduled status the parent neither set
// nor manages here, so they are NOT surfaced. Class-trip days stay excluded for
// the same reason.
func (s *service) ListSickDays(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	var out []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		absences := make([]*activeModels.StudentStatusDay, 0, len(rows))
		for _, r := range rows {
			switch {
			case r.Status == activeModels.StudentStatusDaySick:
				absences = append(absences, r)
			case r.Status == activeModels.StudentStatusDayExcused &&
				r.Source == activeModels.StudentStatusSourceParent:
				// Only parent-reported excused days belong in the parents
				// portal; staff-created excused rows (planned/manual) stay
				// internal so we don't leak their note/source to guardians.
				absences = append(absences, r)
			}
		}
		out = absences
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list absences: %w", txErr)
	}
	return out, nil
}

// SubmitCareException sets the guardian-authored pickup and/or arrival override
// for a single day. The two times are the COMPLETE desired override for the
// day, mirroring the parents-portal modal (which always prefills both fields
// from the current state): a non-nil time sets that leg, a nil time clears the
// guardian row for that leg. So emptying the pickup field and saving removes the
// pickup override while keeping the arrival one, instead of silently retaining
// the old value. At least one leg must be non-nil — clearing the whole day goes
// through DeleteCareException. It mirrors the sick-note path otherwise:
// ownership check, per-tenant feature gate, immediate write under a tenant tx,
// SSE broadcast on commit. A staff-authored exception for the date is never
// clobbered.
func (s *service) SubmitCareException(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime, arrivalTime *time.Time) (*CareException, error) {
	if pickupTime == nil && arrivalTime == nil {
		return nil, ErrNoCareException
	}

	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return nil, ErrPickupChangeDisabled
	}

	today := timezone.TodayDate()
	if date.Before(today) {
		return nil, ErrPastCareDate
	}
	// Cap how far ahead a parent may set an exception: two calendar months,
	// mirroring the parent-portal list window (parseSickDayRange) so a created
	// entry can never fall outside the range the UI shows.
	maxDate := timezone.NewDate(today.Year, today.Month+2, today.Day)
	if date.After(maxDate) {
		return nil, ErrCareDateTooFar
	}

	guardianID := accountID
	var result *CareException
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(txCtx, s.db, studentID, date); err != nil {
			return err
		}

		// A staff-authored exception on EITHER leg makes the whole day the
		// team's deliberate override. Refuse up front rather than per-leg: a
		// parent submitting only an arrival while staff own the pickup would
		// otherwise persist silently yet still render as staff-owned.
		staffOwned, err := s.dayHasStaffException(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if staffOwned {
			return ErrCareExceptionConflict
		}

		// Apply BOTH legs unconditionally: the submitted set is authoritative for
		// the day, so a nil leg clears its guardian row rather than leaving a
		// stale value behind.
		if err := s.applyGuardianPickupException(txCtx, studentID, child.tenantID, date, pickupTime, guardianID); err != nil {
			return err
		}
		if err := s.applyGuardianArrivalException(txCtx, studentID, child.tenantID, date, arrivalTime, guardianID); err != nil {
			return err
		}

		merged, err := s.loadCareException(txCtx, studentID, date)
		if err != nil {
			return err
		}
		result = merged

		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastStudentUpdated(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		// Two submits for the same child+date can race between the find and the
		// insert (e.g. a double-click or two guardians at once); the unique
		// (student_id, exception_date) index makes the loser fail with 23505.
		// Classify it as its own conflict (409) instead of leaking a 500 — and
		// distinct from the staff-override conflict, since the fix differs
		// (reload and retry vs. nothing the parent can do).
		if isExceptionUniqueViolation(txErr) {
			return nil, ErrCareExceptionRaced
		}
		return nil, fmt.Errorf("parent: submit care exception: %w", txErr)
	}

	s.logger.Info("parent submitted care exception",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Bool("has_pickup", pickupTime != nil),
		slog.Bool("has_arrival", arrivalTime != nil),
	)
	return result, nil
}

// isExceptionUniqueViolation reports whether err (or a wrapped DatabaseError)
// carries PostgreSQL code 23505 — a raced duplicate on the (student_id,
// exception_date) unique index for a pickup/arrival exception.
func isExceptionUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var dbErr *base.DatabaseError
	if errors.As(err, &dbErr) {
		err = dbErr.Err
	}
	var pgErr pgdriver.Error
	if errors.As(err, &pgErr) {
		return pgErr.IntegrityViolation() && pgErr.Field('C') == "23505"
	}
	return false
}

// dayHasStaffException reports whether the pickup or arrival exception for the
// date was authored by staff. A staff override on a single leg locks the whole
// day against parent edits.
func (s *service) dayHasStaffException(ctx context.Context, studentID int64, date timezone.Date) (bool, error) {
	pickup, err := s.pickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return false, err
	}
	if pickup != nil && pickup.Source == scheduleModels.ExceptionSourceStaff {
		return true, nil
	}
	arrival, err := s.arrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return false, err
	}
	if arrival != nil && arrival.Source == scheduleModels.ExceptionSourceStaff {
		return true, nil
	}
	return false, nil
}

// applyGuardianPickupException reconciles the guardian-owned pickup leg for the
// date with the submitted time: a non-nil time creates or updates the guardian
// row, a nil time removes any existing guardian row (the parent cleared that
// leg). The day-level staff check in SubmitCareException already rejects
// staff-owned days; the per-leg guard here keeps the helper safe on its own and
// protects against a staff row appearing mid-transaction — a staff leg is never
// touched (neither overwritten nor deleted).
func (s *service) applyGuardianPickupException(ctx context.Context, studentID, tenantID int64, date timezone.Date, pickupTime *time.Time, guardianID int64) error {
	existing, err := s.pickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return err
	}
	if existing != nil && existing.Source == scheduleModels.ExceptionSourceStaff {
		return ErrCareExceptionConflict
	}
	if pickupTime == nil {
		if existing != nil {
			return s.pickupExceptionRepo.Delete(ctx, existing.ID)
		}
		return nil
	}
	if existing != nil {
		existing.PickupTime = pickupTime
		existing.Reason = nil
		existing.Source = scheduleModels.ExceptionSourceGuardian
		existing.CreatedBy = 0
		existing.CreatedByGuardian = &guardianID
		return s.pickupExceptionRepo.Update(ctx, existing)
	}
	entity := &scheduleModels.StudentPickupException{
		StudentID:         studentID,
		ExceptionDate:     date,
		PickupTime:        pickupTime,
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	entity.SetTenantID(tenantID)
	return s.pickupExceptionRepo.Create(ctx, entity)
}

// applyGuardianArrivalException mirrors applyGuardianPickupException for the
// arrival leg.
func (s *service) applyGuardianArrivalException(ctx context.Context, studentID, tenantID int64, date timezone.Date, arrivalTime *time.Time, guardianID int64) error {
	existing, err := s.arrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return err
	}
	if existing != nil && existing.Source == scheduleModels.ExceptionSourceStaff {
		return ErrCareExceptionConflict
	}
	if arrivalTime == nil {
		if existing != nil {
			return s.arrivalExceptionRepo.Delete(ctx, existing.ID)
		}
		return nil
	}
	if existing != nil {
		existing.ExpectedArrival = arrivalTime
		existing.Reason = nil
		existing.Source = scheduleModels.ExceptionSourceGuardian
		existing.CreatedBy = 0
		existing.CreatedByGuardian = &guardianID
		return s.arrivalExceptionRepo.Update(ctx, existing)
	}
	entity := &scheduleModels.StudentArrivalException{
		StudentID:         studentID,
		ExceptionDate:     date,
		ExpectedArrival:   arrivalTime,
		Source:            scheduleModels.ExceptionSourceGuardian,
		CreatedByGuardian: &guardianID,
	}
	entity.SetTenantID(tenantID)
	return s.arrivalExceptionRepo.Create(ctx, entity)
}

// loadCareException merges the pickup and arrival exceptions for one date into
// the parent-facing projection. Returns nil if neither leg has a row.
func (s *service) loadCareException(ctx context.Context, studentID int64, date timezone.Date) (*CareException, error) {
	pickup, err := s.pickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	arrival, err := s.arrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	if pickup == nil && arrival == nil {
		return nil, nil
	}
	out := &CareException{Date: date, Source: scheduleModels.ExceptionSourceGuardian}
	// Any staff-authored leg makes the day staff-owned for display purposes —
	// the portal then shows it read-only rather than as a parent change.
	staffOwned := false
	if pickup != nil {
		out.PickupTime = pickup.PickupTime
		out.UpdatedAt = pickup.UpdatedAt
		if pickup.Source == scheduleModels.ExceptionSourceStaff {
			staffOwned = true
		}
	}
	if arrival != nil {
		out.ArrivalTime = arrival.ExpectedArrival
		if arrival.UpdatedAt.After(out.UpdatedAt) {
			out.UpdatedAt = arrival.UpdatedAt
		}
		if arrival.Source == scheduleModels.ExceptionSourceStaff {
			staffOwned = true
		}
	}
	if staffOwned {
		out.Source = scheduleModels.ExceptionSourceStaff
	}
	return out, nil
}

// ListCareExceptions returns the merged pickup/arrival exceptions for the child
// in [from, to], staff- and guardian-authored alike. Unlike SubmitCareException
// this is not gated by parent_pickup_change_enabled: a parent may always see and
// (via DeleteCareException) clear overrides they created, even after the school
// switches the feature off, so existing entries never become stuck.
func (s *service) ListCareExceptions(ctx context.Context, accountID, studentID int64, from, to timezone.Date) ([]*CareException, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*CareException
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		pickups, err := s.pickupExceptionRepo.FindByStudentIDAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		arrivals, err := s.arrivalExceptionRepo.FindByStudentIDAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		out = mergeCareExceptions(pickups, arrivals)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list care exceptions: %w", txErr)
	}
	return out, nil
}

// DeleteCareException removes only the guardian-authored pickup and arrival
// exceptions for the date, reverting the day to the standard weekly plan. Staff
// rows are left untouched. Idempotent: deleting a day with nothing guardian-owned
// is a no-op (and skips the broadcast). Not gated by the feature toggle for the
// same reason as ListCareExceptions: clearing one's own override stays available.
func (s *service) DeleteCareException(ctx context.Context, accountID, studentID int64, date timezone.Date) error {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return err
	}

	if date.Before(timezone.TodayDate()) {
		return ErrPastCareDate
	}

	deleted := false
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(txCtx, s.db, studentID, date); err != nil {
			return err
		}

		pickup, err := s.pickupExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if pickup != nil && pickup.Source == scheduleModels.ExceptionSourceGuardian {
			if err := s.pickupExceptionRepo.Delete(txCtx, pickup.ID); err != nil {
				return err
			}
			deleted = true
		}
		arrival, err := s.arrivalExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if arrival != nil && arrival.Source == scheduleModels.ExceptionSourceGuardian {
			if err := s.arrivalExceptionRepo.Delete(txCtx, arrival.ID); err != nil {
				return err
			}
			deleted = true
		}
		if deleted {
			capturedTenant := child.tenantID
			tenant.RegisterAfterCommit(txCtx, func() {
				s.broadcastStudentUpdated(capturedTenant, studentID)
			})
		}
		return nil
	})
	if txErr != nil {
		return fmt.Errorf("parent: delete care exception: %w", txErr)
	}
	return nil
}

// mergeCareExceptions joins pickup and arrival exception rows by date into the
// parent-facing projection, sorted ascending by date.
func mergeCareExceptions(pickups []*scheduleModels.StudentPickupException, arrivals []*scheduleModels.StudentArrivalException) []*CareException {
	byDate := make(map[timezone.Date]*CareException)
	order := make([]timezone.Date, 0, len(pickups)+len(arrivals))
	get := func(date timezone.Date) *CareException {
		if ce, ok := byDate[date]; ok {
			return ce
		}
		ce := &CareException{Date: date, Source: scheduleModels.ExceptionSourceGuardian}
		byDate[date] = ce
		order = append(order, date)
		return ce
	}
	for _, p := range pickups {
		ce := get(p.ExceptionDate)
		ce.PickupTime = p.PickupTime
		if p.UpdatedAt.After(ce.UpdatedAt) {
			ce.UpdatedAt = p.UpdatedAt
		}
		if p.Source == scheduleModels.ExceptionSourceStaff {
			ce.Source = scheduleModels.ExceptionSourceStaff
		}
	}
	for _, a := range arrivals {
		ce := get(a.ExceptionDate)
		ce.ArrivalTime = a.ExpectedArrival
		if a.UpdatedAt.After(ce.UpdatedAt) {
			ce.UpdatedAt = a.UpdatedAt
		}
		if a.Source == scheduleModels.ExceptionSourceStaff {
			ce.Source = scheduleModels.ExceptionSourceStaff
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].Before(order[j]) })
	out := make([]*CareException, 0, len(order))
	for _, d := range order {
		out = append(out, byDate[d])
	}
	return out
}

// broadcastStudentUpdated fires a tenant-scoped student_updated SSE event
// so supervisors' live views refresh after a parent-side change. Mirrors
// the staff handler's broadcast; fire-and-forget.
func (s *service) broadcastStudentUpdated(tenantID, studentID int64) {
	if s.broadcaster == nil || tenantID <= 0 {
		return
	}
	source := activeModels.StudentStatusSourceParent
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
	if err := s.broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.logger.Warn("parent: failed to broadcast student update",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
	}
}

// applyLiveStatusForParentToday updates the live student flags for a parent
// submission that includes today. A "Krankmeldung" (sick) flips the live sick
// flag on and clears any excused flag, exactly as before. A "Termin/Abwesenheit"
// (excused) sets NO live flag per issue #1735 — it only clears a stale live sick
// flag so the row stays consistent with the now-cleared sick status day, and
// leaves a staff-set excused flag untouched.
func applyLiveStatusForParentToday(student *usersModels.Student, status string, now time.Time) {
	trueVal := true
	falseVal := false
	switch status {
	case activeModels.StudentStatusDaySick:
		student.Sick = &trueVal
		student.SickSince = &now
		student.Excused = &falseVal
		student.ExcusedSince = nil
	case activeModels.StudentStatusDayExcused:
		student.Sick = &falseVal
		student.SickSince = nil
	}
}

func containsDate(dates []timezone.Date, needle timezone.Date) bool {
	for _, d := range dates {
		if d == needle {
			return true
		}
	}
	return false
}

func minDate(dates []timezone.Date) timezone.Date {
	min := dates[0]
	for _, d := range dates[1:] {
		if d.Before(min) {
			min = d
		}
	}
	return min
}

func maxDate(dates []timezone.Date) timezone.Date {
	max := dates[0]
	for _, d := range dates[1:] {
		if d.After(max) {
			max = d
		}
	}
	return max
}
