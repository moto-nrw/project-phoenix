package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	mealplanModels "github.com/moto-nrw/project-phoenix/models/mealplan"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	mealplanService "github.com/moto-nrw/project-phoenix/services/mealplan"
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
	// ErrMealPlanDisabled means operations.meal_plan_enabled is off for the
	// child's tenant, so the parents portal must hide the meal plan section.
	ErrMealPlanDisabled = errors.New("parent: meal plan disabled for this school")
	// ErrMealPlanWeekOutOfRange means the requested week is outside the window
	// parents may view (the current and next work week). Staff may plan
	// arbitrary future weeks on the staff page, but those are drafts; the
	// parents portal only ever exposes this week and next, so a request for any
	// other week is refused rather than leaking an unpublished menu.
	ErrMealPlanWeekOutOfRange = errors.New("parent: meal plan week is outside the viewable range")
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
	// ErrExcusedRequestNotFound means the excused-absence request id does not
	// belong to a request the caller submitted for this child (#1845).
	ErrExcusedRequestNotFound = errors.New("parent: excused absence request not found")
	// ErrExcusedRequestNotPending means the excused-absence request was already
	// decided or withdrawn, so it can no longer be withdrawn.
	ErrExcusedRequestNotPending = errors.New("parent: excused absence request is not pending")
	// ErrExcusedRequestOverlap means a different pending excused request already
	// covers one of the submitted dates (#1845). An identical resubmit is handled
	// idempotently by the request service and never reaches this error.
	ErrExcusedRequestOverlap = errors.New("parent: excused absence request overlaps an existing pending request")
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
	err := tenant.WithAdminTx(ctx, s.DB, func(adminCtx context.Context, _ bun.Tx) error {
		child, findErr := s.ChildRepo.FindForAccount(adminCtx, accountID, studentID)
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
			guardianProfileID:   child.GuardianProfileID,
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
	guardianProfileID   int64
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

// SickNoteResult is the outcome of a parent absence submission. Exactly one of
// its fields is populated: StatusDays for a direct write (sick, or excused when
// the approval gate is off), PendingRequest when an excused report was turned
// into a pending office-approval request (#1845).
type SickNoteResult struct {
	StatusDays     []*activeModels.StudentStatusDay
	PendingRequest *activeModels.ExcusedAbsenceRequest
}

// SubmitSickNote reports the child absent for the given dates with the chosen
// status. The status is either StudentStatusDaySick (a "Krankmeldung": flips the
// live sick flag when today is included) or StudentStatusDayExcused (an
// "entschuldigte Abmeldung": stored with NO live flag, per issue #1735). A note
// is mandatory for excused, optional for sick.
//
// When operations.parent_excused_requires_approval is on for the child's tenant,
// an excused report does NOT write a status day; it creates a PENDING request
// (#1845) that staff must confirm, and the result carries PendingRequest. Sick
// reports, and excused reports while the gate is off, are written directly and
// the result carries StatusDays.
func (s *service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string) (*SickNoteResult, error) {
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

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if !enabled {
		return nil, ErrSickNoteDisabled
	}

	// Count characters (runes), not UTF-8 bytes, so the limit matches the
	// frontend's maxLength — a German text with umlauts stays under the budget.
	trimmedNote := strings.TrimSpace(reason)
	if utf8.RuneCountInString(trimmedNote) > maxParentNoteLen {
		return nil, ErrNoteTooLong
	}
	// A note is mandatory for an excused absence (#1845): the office needs a
	// reason. Krankmeldungen keep the note optional.
	if status == activeModels.StudentStatusDayExcused && trimmedNote == "" {
		return nil, ErrEmptyNote
	}

	// Optional office-approval gate for excused absences (#1845). When on, the
	// report becomes a pending request instead of a direct status-day write, so
	// the child stays "expected" until staff decide.
	if status == activeModels.StudentStatusDayExcused {
		requiresApproval, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentExcusedRequiresApproval)
		if err != nil {
			return nil, fmt.Errorf("parent: resolve excused-approval setting: %w", err)
		}
		if requiresApproval {
			return s.submitExcusedRequest(ctx, child, accountID, studentID, dates, trimmedNote)
		}
	}

	now := time.Now()
	today := timezone.TodayDate()

	var notePtr *string
	if trimmedNote != "" {
		notePtr = &trimmedNote
	}

	var result []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		for _, other := range activeModels.StudentStatusDayStatusesExcept(status) {
			if err := s.StatusDayRepo.MarkClearedForDates(txCtx, studentID, other, dates, now, activeModels.StudentStatusSourceParent); err != nil {
				return err
			}
		}
		for _, d := range dates {
			if err := s.StatusDayRepo.UpsertReported(txCtx, &activeModels.StudentStatusDay{
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

		if slices.Contains(dates, today) {
			fresh, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
			if err != nil {
				return err
			}
			applyLiveStatusForParentToday(fresh, status, now)
			if err := s.StudentRepo.Update(txCtx, fresh); err != nil {
				return err
			}
		}

		rows, err := s.StatusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, slices.MinFunc(dates, timezone.Date.Compare), slices.MaxFunc(dates, timezone.Date.Compare))
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
		pillBody := sickNoteEventBody(status, dates)
		pillRefID := firstStatusID(filtered)
		tenant.RegisterAfterCommit(txCtx, func() {
			s.emitSelfServicePill(capturedTenant, studentID, accountID, "sick_note", pillBody, "active.student_status_days", pillRefID)
			s.broadcastStudentUpdated(capturedTenant, studentID)
			// broadcastStudentUpdated is staff-only and emitSelfServicePill wakes
			// just the acting guardian's thread; fan out to EVERY guardian so a
			// co-guardian's open tab drops the stale presence too (#1725 review).
			s.wakeChildGuardians(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: submit sick note: %w", txErr)
	}

	s.Logger.Info("parent submitted absence",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.String("status", status),
		slog.Int("days", len(dates)),
		slog.Bool("has_reason", notePtr != nil),
	)
	return &SickNoteResult{StatusDays: result}, nil
}

// submitExcusedRequest turns an excused report into a pending office-approval
// request (#1845) inside the child's tenant transaction, then returns it as the
// submission result. The note was already validated as non-empty and within the
// length bound by the caller. Errors from the request service map onto the
// parent sentinels so the handler renders stable status codes.
func (s *service) submitExcusedRequest(ctx context.Context, child *parentChild, accountID, studentID int64, dates []timezone.Date, note string) (*SickNoteResult, error) {
	if s.ExcusedRequests == nil {
		return nil, fmt.Errorf("parent: excused request service not configured")
	}
	var req *activeModels.ExcusedAbsenceRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		created, err := s.ExcusedRequests.CreateRequest(txCtx, studentID, accountID, dates, note)
		if err != nil {
			return err
		}
		req = created
		return nil
	})
	if txErr != nil {
		switch {
		case errors.Is(txErr, absenceSvc.ErrExcusedRequestNoDates):
			return nil, ErrNoDates
		case errors.Is(txErr, absenceSvc.ErrExcusedRequestEmptyNote):
			return nil, ErrEmptyNote
		case errors.Is(txErr, absenceSvc.ErrExcusedRequestNoteTooLong):
			return nil, ErrNoteTooLong
		case errors.Is(txErr, absenceSvc.ErrExcusedRequestOverlap):
			return nil, ErrExcusedRequestOverlap
		default:
			return nil, fmt.Errorf("parent: submit excused request: %w", txErr)
		}
	}
	s.Logger.Info("parent submitted excused absence request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Int64("request_id", req.ID),
		slog.Int("days", len(dates)),
	)
	return &SickNoteResult{PendingRequest: req}, nil
}

// ListExcusedRequests returns the child's pending excused-absence requests plus
// any decided in the recent window, newest-first (#1845). Read-only: a linked
// guardian with portal access may see them.
func (s *service) ListExcusedRequests(ctx context.Context, accountID, studentID int64) ([]*activeModels.ExcusedAbsenceRequest, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.ExcusedRequests == nil {
		return []*activeModels.ExcusedAbsenceRequest{}, nil
	}
	// Show rejected/withdrawn requests for two weeks so a parent learns the
	// outcome, while pending ones show regardless of age.
	recentSince := time.Now().AddDate(0, 0, -14)
	var out []*activeModels.ExcusedAbsenceRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.ExcusedRequests.ListForStudent(txCtx, studentID, recentSince)
		if err != nil {
			return err
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list excused requests: %w", txErr)
	}
	return out, nil
}

// WithdrawExcusedRequest withdraws the caller's own pending excused-absence
// request (#1845). Gated ONLY on portal access, not parent_portal.sick_note.submit:
// a guardian must be able to wind down their OWN outstanding request even after
// the school revokes their submit permission — ListExcusedRequests still
// surfaces the pending request and the UI still offers withdrawal, so the write
// gate must match that read gate. Ownership (submitted_by) and the
// pending-status check are enforced inside excusedRequests.WithdrawRequest,
// which binds the request to this accountID and studentID. Mirrors
// WithdrawCareScheduleRequest.
func (s *service) WithdrawExcusedRequest(ctx context.Context, accountID, studentID, requestID int64) (*activeModels.ExcusedAbsenceRequest, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.ExcusedRequests == nil {
		return nil, ErrExcusedRequestNotFound
	}
	var out *activeModels.ExcusedAbsenceRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		req, err := s.ExcusedRequests.WithdrawRequest(txCtx, requestID, studentID, accountID)
		if err != nil {
			return err
		}
		out = req
		return nil
	})
	if txErr != nil {
		switch {
		case errors.Is(txErr, activeModels.ErrExcusedRequestNotFound):
			return nil, ErrExcusedRequestNotFound
		case errors.Is(txErr, activeModels.ErrExcusedRequestNotPending):
			return nil, ErrExcusedRequestNotPending
		default:
			return nil, fmt.Errorf("parent: withdraw excused request: %w", txErr)
		}
	}
	return out, nil
}

// ChildFeatures resolves the parent-portal feature toggles for the child's
// tenant after verifying ownership.
func (s *service) ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}
	sick, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	excusedApproval, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentExcusedRequiresApproval)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve excused-approval setting: %w", err)
	}
	notes, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	pickupChange, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	inviteMode, err := s.Settings.ResolveStringForTenant(ctx, child.tenantID, configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	canRemove, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyGuardianParentCanRemove)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	masterEdit, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentMasterDataEditEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve master-data edit setting: %w", err)
	}
	masterRequest, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentMasterDataRequestEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	mealPlan, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyMealPlanEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve meal-plan setting: %w", err)
	}
	news, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNewsEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve parent-news setting: %w", err)
	}
	guardianManagement, err := s.guardianManagementEnabled(ctx, child.tenantID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}
	canEditMasterData := masterEdit && child.hasPermission(authorize.GuardianPermissionMasterDataEdit)
	return ChildFeatureFlags{
		HasOpenChangeRequest:         s.hasOpenChangeRequest(ctx, child.tenantID, studentID),
		SickNoteEnabled:              sick && child.hasPermission(authorize.GuardianPermissionSickNoteSubmit),
		ExcusedRequiresApproval:      excusedApproval,
		NotesEnabled:                 notes && child.hasPermission(authorize.GuardianPermissionNotesWrite),
		RequestSubmitEnabled:         notes && child.hasPermission(authorize.GuardianPermissionRequestSubmit),
		PickupChangeEnabled:          pickupChange,
		RelatedAccountsInviteEnabled: inviteMode != configModels.ParentInviteModeDisabled,
		RelatedAccountsRemoveEnabled: canRemove && inviteMode != configModels.ParentInviteModeDisabled,
		MasterDataEditEnabled:        canEditMasterData,
		MasterDataContactEditEnabled: canEditMasterData && guardianManagement,
		MasterDataRequestEnabled:     masterRequest && child.hasPermission(authorize.GuardianPermissionMasterDataRequest),
		MealPlanEnabled:              mealPlan,
		NewsEnabled:                  news,
	}, nil
}

// hasOpenChangeRequest reports whether the child has a pending change request
// (care schedule OR master data) awaiting an OGS decision, so the parent
// overview can badge the Stammdaten entry. The lookups hit tenant-scoped/RLS
// tables, so they must run inside a tenant transaction — ChildFeatures is only
// parent-authenticated and carries no tenant context otherwise. Best-effort: a
// query error logs and yields false so a transient failure never shows a
// phantom badge.
func (s *service) hasOpenChangeRequest(ctx context.Context, tenantID, studentID int64) bool {
	open := false
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if s.CareRequests != nil {
			if req, _, err := s.CareRequests.GetPendingForStudent(txCtx, studentID); err != nil {
				s.Logger.Warn("parent: pending care-request check failed",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			} else if req != nil {
				open = true
				return nil
			}
		}
		if s.ChangeRequestRepo != nil {
			pending, err := s.ChangeRequestRepo.ListByStudent(txCtx, studentID, []string{usersModels.DataChangeStatusPending}, 0)
			if err != nil {
				s.Logger.Warn("parent: pending master-data check failed",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			} else if len(pending) > 0 {
				open = true
			}
		}
		return nil
	})
	if err != nil {
		s.Logger.Warn("parent: open change-request check failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return false
	}
	return open
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.StatusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, from, to)
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

// MealPlanWeek returns the child's school meal plan for the Monday-Friday week
// containing weekStart. Unlike ListSickDays this is gated by the
// operations.meal_plan_enabled toggle: if the school does not run a meal plan
// the parent must not see one, so a disabled tenant yields ErrMealPlanDisabled.
func (s *service) MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart timezone.Date) ([]*mealplanModels.MealPlanEntry, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyMealPlanEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve meal-plan setting: %w", err)
	}
	if !enabled {
		return nil, ErrMealPlanDisabled
	}

	monday, friday := mealplanService.WeekRange(weekStart)
	// Parents may only read the current and next work week. Staff can plan
	// arbitrary future (and past) weeks on the staff page; those are drafts and
	// must not be reachable through the parent proxy by supplying a crafted
	// week_start. Compare on the normalized Monday so any day within an allowed
	// week resolves the same.
	currentMonday, _ := mealplanService.WeekRange(timezone.TodayDate())
	if monday != currentMonday && monday != currentMonday.AddDays(7) {
		return nil, ErrMealPlanWeekOutOfRange
	}

	var out []*mealplanModels.MealPlanEntry
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, findErr := s.MealPlanRepo.FindByDateRange(txCtx, monday, friday)
		if findErr != nil {
			return findErr
		}
		out = rows
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: meal plan week: %w", txErr)
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

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(txCtx, s.DB, studentID, date); err != nil {
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
		pillBody := careExceptionEventBody(date, pickupTime, arrivalTime)
		pillRefTable, pillRefID := s.careExceptionRef(txCtx, studentID, date)
		tenant.RegisterAfterCommit(txCtx, func() {
			s.emitSelfServicePill(capturedTenant, studentID, accountID, "care_exception", pillBody, pillRefTable, pillRefID)
			s.broadcastStudentUpdated(capturedTenant, studentID)
			// Fan out to EVERY guardian so a co-guardian's open tab reflects the
			// new override on the "Heute" tile live (#1725 review).
			s.wakeChildGuardians(capturedTenant, studentID)
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
		if base.IsUniqueViolation(txErr) {
			return nil, ErrCareExceptionRaced
		}
		return nil, fmt.Errorf("parent: submit care exception: %w", txErr)
	}

	s.Logger.Info("parent submitted care exception",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Bool("has_pickup", pickupTime != nil),
		slog.Bool("has_arrival", arrivalTime != nil),
	)
	return result, nil
}

// dayHasStaffException reports whether the pickup or arrival exception for the
// date was authored by staff. A staff override on a single leg locks the whole
// day against parent edits.
func (s *service) dayHasStaffException(ctx context.Context, studentID int64, date timezone.Date) (bool, error) {
	pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return false, err
	}
	if pickup != nil && pickup.Source == scheduleModels.ExceptionSourceStaff {
		return true, nil
	}
	arrival, err := s.ArrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
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
	return applyGuardianTimeException(ctx, pickupTime,
		func(ctx context.Context) (*scheduleModels.StudentPickupException, error) {
			return s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
		},
		func(e *scheduleModels.StudentPickupException) string { return e.Source },
		func(ctx context.Context, e *scheduleModels.StudentPickupException) error {
			return s.PickupExceptionRepo.Delete(ctx, e.ID)
		},
		func(ctx context.Context, e *scheduleModels.StudentPickupException) error {
			e.PickupTime = pickupTime
			e.Reason = nil
			e.Source = scheduleModels.ExceptionSourceGuardian
			e.CreatedBy = 0
			e.CreatedByGuardian = &guardianID
			return s.PickupExceptionRepo.Update(ctx, e)
		},
		func(ctx context.Context) error {
			entity := &scheduleModels.StudentPickupException{
				StudentID:         studentID,
				ExceptionDate:     date,
				PickupTime:        pickupTime,
				Source:            scheduleModels.ExceptionSourceGuardian,
				CreatedByGuardian: &guardianID,
			}
			entity.SetTenantID(tenantID)
			return s.PickupExceptionRepo.Create(ctx, entity)
		})
}

// applyGuardianArrivalException mirrors applyGuardianPickupException for the
// arrival leg.
func (s *service) applyGuardianArrivalException(ctx context.Context, studentID, tenantID int64, date timezone.Date, arrivalTime *time.Time, guardianID int64) error {
	return applyGuardianTimeException(ctx, arrivalTime,
		func(ctx context.Context) (*scheduleModels.StudentArrivalException, error) {
			return s.ArrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
		},
		func(e *scheduleModels.StudentArrivalException) string { return e.Source },
		func(ctx context.Context, e *scheduleModels.StudentArrivalException) error {
			return s.ArrivalExceptionRepo.Delete(ctx, e.ID)
		},
		func(ctx context.Context, e *scheduleModels.StudentArrivalException) error {
			e.ExpectedArrival = arrivalTime
			e.Reason = nil
			e.Source = scheduleModels.ExceptionSourceGuardian
			e.CreatedBy = 0
			e.CreatedByGuardian = &guardianID
			return s.ArrivalExceptionRepo.Update(ctx, e)
		},
		func(ctx context.Context) error {
			entity := &scheduleModels.StudentArrivalException{
				StudentID:         studentID,
				ExceptionDate:     date,
				ExpectedArrival:   arrivalTime,
				Source:            scheduleModels.ExceptionSourceGuardian,
				CreatedByGuardian: &guardianID,
			}
			entity.SetTenantID(tenantID)
			return s.ArrivalExceptionRepo.Create(ctx, entity)
		})
}

// applyGuardianTimeException is the shared control flow of the guardian
// pickup/arrival leg appliers: a staff-owned row is never touched (neither
// overwritten nor deleted), a nil time removes any existing guardian row
// (the parent cleared that leg), and a non-nil time creates or updates the
// guardian row.
func applyGuardianTimeException[P any, E *P](ctx context.Context, t *time.Time,
	find func(context.Context) (E, error),
	sourceOf func(E) string,
	del func(context.Context, E) error,
	stampAndUpdate func(context.Context, E) error,
	create func(context.Context) error,
) error {
	existing, err := find(ctx)
	if err != nil {
		return err
	}
	if existing != nil && sourceOf(existing) == scheduleModels.ExceptionSourceStaff {
		return ErrCareExceptionConflict
	}
	if t == nil {
		if existing != nil {
			return del(ctx, existing)
		}
		return nil
	}
	if existing != nil {
		return stampAndUpdate(ctx, existing)
	}
	return create(ctx)
}

// loadCareException merges the pickup and arrival exceptions for one date into
// the parent-facing projection. Returns nil if neither leg has a row.
func (s *service) loadCareException(ctx context.Context, studentID int64, date timezone.Date) (*CareException, error) {
	pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return nil, err
	}
	arrival, err := s.ArrivalExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
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
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		pickups, err := s.PickupExceptionRepo.FindByStudentIDAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		arrivals, err := s.ArrivalExceptionRepo.FindByStudentIDAndDateRange(txCtx, studentID, from, to)
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

	pickupDeleted, arrivalDeleted := false, false
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := scheduleService.LockCareExceptionDay(txCtx, s.DB, studentID, date); err != nil {
			return err
		}

		pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if pickup != nil && pickup.Source == scheduleModels.ExceptionSourceGuardian {
			if err := s.PickupExceptionRepo.Delete(txCtx, pickup.ID); err != nil {
				return err
			}
			pickupDeleted = true
		}
		arrival, err := s.ArrivalExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if arrival != nil && arrival.Source == scheduleModels.ExceptionSourceGuardian {
			if err := s.ArrivalExceptionRepo.Delete(txCtx, arrival.ID); err != nil {
				return err
			}
			arrivalDeleted = true
		}
		if pickupDeleted || arrivalDeleted {
			capturedTenant := child.tenantID
			// Name the leg(s) actually removed: an arrival-only deletion must not
			// be recorded as a withdrawn pickup. Both legs collapse to a neutral
			// "Betreuungszeit" label.
			leg := "Betreuungszeit"
			switch {
			case pickupDeleted && !arrivalDeleted:
				leg = "Abholung"
			case arrivalDeleted && !pickupDeleted:
				leg = "Ankunft"
			}
			pillBody := "Korrektur: " + leg + " " + date.Format("02.01.") + " zurückgezogen"
			tenant.RegisterAfterCommit(txCtx, func() {
				s.emitSelfServicePill(capturedTenant, studentID, accountID, "care_exception_correction", pillBody, "", nil)
				s.broadcastStudentUpdated(capturedTenant, studentID)
				// Fan out to EVERY guardian so a co-guardian's open tab drops the
				// removed override on the "Heute" tile live (#1725 review).
				s.wakeChildGuardians(capturedTenant, studentID)
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
		// A pickup row with no time is an absence marker, not "no override". Carry
		// that distinction to the parent UI so a staff-set "not coming today" row
		// resolves to an absence rather than falling through to the base plan.
		ce.PickupAbsent = p.PickupTime == nil
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
		// An arrival row with no expected time is a "not coming today" absence
		// marker (StudentArrivalException.IsAbsent), the arrival-leg twin of a
		// timeless pickup row. It creates no status day either, so carry the
		// distinction to the parent UI: an arrival-only absence must resolve to
		// an absence, not fall through to a regular pickup time (#1725 review).
		ce.ArrivalAbsent = a.IsAbsent()
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
	if s.Broadcaster == nil || tenantID <= 0 {
		return
	}
	source := activeModels.StudentStatusSourceParent
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
	if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.Logger.Warn("parent: failed to broadcast student update",
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
