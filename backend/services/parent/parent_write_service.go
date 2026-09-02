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
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	notificationsSvc "github.com/moto-nrw/project-phoenix/modules/delivery/application/notifications"
	mealplanModule "github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/realtime"
	absenceSvc "github.com/moto-nrw/project-phoenix/services/absence"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	scheduleService "github.com/moto-nrw/project-phoenix/services/schedule"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
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
	ErrMealPlanWeekOutOfRange      = errors.New("parent: meal plan week is outside the viewable range")
	ErrMealRegistrationDisabled    = errors.New("parent: meal registration disabled for this school")
	ErrMealParticipationOutOfRange = errors.New("parent: meal participation date is outside the changeable range")
	ErrMealParticipationCutoff     = errors.New("parent: meal participation cutoff has passed")
	ErrInvalidMealParticipation    = errors.New("parent: invalid meal participation")
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
	// ErrCareExceptionReasonRequired means the parent API request omitted the
	// explanation staff need to understand a changed pickup time.
	ErrCareExceptionReasonRequired = errors.New("parent: care exception reason is required")
	ErrCareExceptionReasonTooLong  = errors.New("parent: care exception reason exceeds 255 characters")
	ErrCareExceptionAlreadyLeft    = errors.New("parent: child has already left care today")
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
	// ErrExcusedRequestNotFound is the legacy-named error for an absence request
	// the caller did not submit for this child.
	ErrExcusedRequestNotFound = errors.New("parent: excused absence request not found")
	// ErrExcusedRequestNotPending means the legacy-named absence request was
	// already decided or withdrawn, so it can no longer be withdrawn.
	ErrExcusedRequestNotPending = errors.New("parent: excused absence request is not pending")
	// ErrExcusedRequestOverlap means a different pending absence request already
	// covers one of the submitted dates. An identical resubmit is idempotent.
	ErrExcusedRequestOverlap = errors.New("parent: excused absence request overlaps an existing pending request")
	// ErrChildCareEnded means the child's care at this school has ended (#2487).
	// Every parent WRITE for that child is refused from the day after the last
	// care day; reading what happened before stays open, which is why this is
	// checked per write path and not in resolvePermittedChild itself.
	ErrChildCareEnded = errors.New("parent: care for this child has ended")
)

// resolveOwnedChild validates the account is a guardian of the student
// and returns the child's tenant id. The cross-tenant lookup runs under
// an admin tx; a nil child becomes ErrChildNotLinked so the caller never
// trusts a studentID it can't prove ownership of.
func (s *service) resolveOwnedChild(ctx context.Context, accountID, studentID int64) (*parentChild, error) {
	return s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
}

// requireCareRunningForUpdate locks the child before a parent write so a
// concurrent care exit cannot turn an already-authorized operation into a
// post-exit write.
func (s *service) requireCareRunningForUpdate(ctx context.Context, studentID int64) error {
	student, err := s.StudentRepo.FindByIDForUpdate(ctx, studentID)
	if err != nil {
		return err
	}
	if student.CareEndedOn(s.todayDate()) {
		return ErrChildCareEnded
	}
	return nil
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
			careEnded:           child.CareEnded(s.todayDate()),
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
	// careEnded mirrors the child's enrollment interval as of today (#2487).
	careEnded bool
}

// requireCareRunning refuses a write for a child whose care has ended. Reads
// deliberately do not call it: a family keeps access to what happened while
// their child was here.
func (c *parentChild) requireCareRunning() error {
	if c == nil {
		return ErrChildNotLinked
	}
	if c.careEnded {
		return ErrChildCareEnded
	}
	return nil
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
// its fields is populated: StatusDays for a direct write, PendingRequest when
// the selected absence type requires office approval (#1845, #2447, #2449).
type SickNoteResult struct {
	StatusDays     []*activeModels.StudentStatusDay
	PendingRequest *activeModels.ExcusedAbsenceRequest
}

// SubmitSickNote reports the child absent for the given dates with the chosen
// status. The status is either StudentStatusDaySick (a "Krankmeldung": flips the
// live sick flag when today is included) or StudentStatusDayExcused (an
// "entschuldigte Abmeldung": stored with NO live flag, per issue #1735). A note
// is mandatory for both absence types.
//
// Each absence type has an independent approval setting. When its gate is on,
// the report creates a PENDING request and writes no status day until staff
// approve it. With the gate off, the report is applied directly.
func (s *service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []timezone.Date, reason, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
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
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
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
	// The note is mandatory only while the school's reason policy asks the
	// family for one (#2267, story 28). Every other school keeps the previous
	// behaviour, including one that never configured the setting.
	if trimmedNote == "" && s.guardianReasonRequired(ctx, child.tenantID) {
		return nil, ErrEmptyNote
	}

	approvalKey := configModels.KeyParentSickRequiresApproval
	if status == activeModels.StudentStatusDayExcused {
		approvalKey = configModels.KeyParentExcusedRequiresApproval
	}
	requiresApproval, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, approvalKey)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve absence-approval setting %s: %w", approvalKey, err)
	}
	if requiresApproval {
		return s.submitAbsenceRequest(ctx, child, accountID, studentID, dates, trimmedNote, status, recipientGuardianProfileIDs)
	}

	now := time.Now()
	today := s.todayDate()

	var notePtr *string
	if trimmedNote != "" {
		notePtr = &trimmedNote
	}

	var result []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// Serialize every parent status write with staff writes on the same
		// student, including future-only ranges. Staff conflict detection relies
		// on this lock to make its read and write one atomic decision.
		fresh, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		// resolvePermittedChild ran before this transaction. Re-check the
		// interval after acquiring the same row lock as care exits so a care exit
		// cannot commit between authorization and this write.
		if fresh.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if err := s.ensureNoPartialAbsenceForStatusWrite(txCtx, studentID, dates); err != nil {
			return err
		}
		notifyAbsence := slices.Contains(dates, today) &&
			isNewParentReportableAbsence(fresh, status)

		for _, other := range activeModels.StudentStatusDayStatusesExcept(status) {
			if err := s.StatusDayRepo.MarkClearedForDates(txCtx, studentID, other, dates, now, activeModels.StudentStatusSourceParent); err != nil {
				return err
			}
		}
		for _, d := range dates {
			guardianAccountID := accountID
			if err := s.StatusDayRepo.UpsertReported(txCtx, &activeModels.StudentStatusDay{
				StudentID:         studentID,
				Date:              d,
				Status:            status,
				ReportedAt:        now,
				Source:            activeModels.StudentStatusSourceParent,
				GuardianAccountID: &guardianAccountID,
				Note:              notePtr,
			}); err != nil {
				return err
			}
		}

		if slices.Contains(dates, today) {
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
		if notifyAbsence && s.AbsenceNotifier != nil {
			if err := s.AbsenceNotifier.NotifyAbsenceReported(txCtx, notificationsSvc.AbsenceReport{
				TenantID: capturedTenant, StudentIDs: []int64{studentID}, Status: status, Dates: dates,
				FromParent: true, ActorAccountID: accountID,
			}); err != nil {
				return err
			}
		}
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

// ensureNoPartialAbsenceForStatusWrite serializes parent and staff writes for
// every requested day, then refuses to replace a time-specific excusal with a
// broad sick/excused status. The check must run before clearing any status rows.
func (s *service) ensureNoPartialAbsenceForStatusWrite(
	ctx context.Context, studentID int64, dates []timezone.Date,
) error {
	if s.DB == nil {
		return errors.New("parent: database is not configured")
	}
	if s.PickupExceptionRepo == nil {
		return errors.New("parent: pickup exception repository is not configured")
	}

	sortedDates := append([]timezone.Date(nil), dates...)
	slices.SortFunc(sortedDates, timezone.Date.Compare)
	for _, date := range sortedDates {
		if err := scheduleService.LockCareExceptionDay(ctx, s.DB, studentID, date); err != nil {
			return err
		}
	}

	rows, err := s.PickupExceptionRepo.FindByStudentIDAndDateRange(
		ctx, studentID, sortedDates[0], sortedDates[len(sortedDates)-1],
	)
	if err != nil {
		return err
	}
	requested := make(map[timezone.Date]struct{}, len(dates))
	for _, date := range dates {
		requested[date] = struct{}{}
	}
	for _, row := range rows {
		// Only manual partial absences conflict; auto-derived excusals
		// (pulled-forward pickup time, #2360) coexist with a full-day status.
		if _, ok := requested[row.ExceptionDate]; ok && row.HasManualPartialAbsence() {
			return ErrCareExceptionConflict
		}
	}
	return nil
}

func isNewParentReportableAbsence(student *usersModels.Student, status string) bool {
	switch status {
	case activeModels.StudentStatusDaySick:
		return student.Sick == nil || !*student.Sick
	case activeModels.StudentStatusDayExcused:
		return student.Excused == nil || !*student.Excused
	default:
		return false
	}
}

// submitAbsenceRequest turns a sick or excused report into a pending office
// request inside the child's tenant transaction.
func (s *service) submitAbsenceRequest(ctx context.Context, child *parentChild, accountID, studentID int64, dates []timezone.Date, note, status string, recipientGuardianProfileIDs []int64) (*SickNoteResult, error) {
	if s.ExcusedRequests == nil {
		return nil, fmt.Errorf("parent: absence request service not configured")
	}
	var req *activeModels.ExcusedAbsenceRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// The initial authorization snapshot may predate a concurrent care exit.
		// Lock and re-read the child before creating a pending request so both
		// absence paths obey the same read-only boundary.
		fresh, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if fresh.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		// The note is mandatory only while the school's reason policy asks the
		// family for one (#2267, story 28).
		created, err := s.ExcusedRequests.Create(txCtx, absenceSvc.ExcusedRequestCreateInput{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Dates:             dates,
			Note:              note,
			AbsenceStatus:     status,
			NoteRequired:      s.guardianReasonRequired(ctx, child.tenantID),
		})
		if err != nil {
			return err
		}
		// The recipient choice is written in the SAME transaction as the
		// request: a refused share rolls the request back with it, so a family
		// never ends up with a request nobody they picked can see (#2267).
		if err := s.ShareRequestInTx(
			txCtx, accountID, studentID, RequestShareExcused, created.ID, recipientGuardianProfileIDs,
		); err != nil {
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
		// A planned partial-day excusal already owns one of the requested dates
		// (same refusal as a direct parent status write that hits
		// ensureNoPartialAbsenceForStatusWrite). Surface as the existing care
		// conflict so the handler returns HTTP 409, not 500.
		case errors.Is(txErr, absenceSvc.ErrExcusedRequestStatusConflict):
			return nil, ErrCareExceptionConflict
		default:
			return nil, fmt.Errorf("parent: submit absence request: %w", txErr)
		}
	}
	s.Logger.Info("parent submitted absence request",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Int64("request_id", req.ID),
		slog.String("status", status),
		slog.Int("days", len(dates)),
	)
	return &SickNoteResult{PendingRequest: req}, nil
}

// ListExcusedRequests is the legacy-named read path for the child's pending and
// recently decided absence requests submitted by the calling guardian. The
// child's effective absence state is shared separately; request notes and
// decision reasons remain private to their submitter.
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
		visibility, visibilityErr := s.loadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		out = visibleExcusedRequests(rows, accountID, visibility)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list excused requests: %w", txErr)
	}
	return out, nil
}

func visibleExcusedRequests(
	rows []*activeModels.ExcusedAbsenceRequest, accountID int64, visibility *requestShareVisibility,
) []*activeModels.ExcusedAbsenceRequest {
	out := make([]*activeModels.ExcusedAbsenceRequest, 0, len(rows))
	for _, row := range rows {
		if row != nil && visibility.allows(RequestShareExcused, row.ID, accountID, row.SubmittedBy) {
			out = append(out, row)
		}
	}
	return out
}

// guardianReasonRequired resolves operations.parent_request_reason_policy for
// the child's school and answers whether the SUBMITTING family must state a
// reason (#2267, story 28). A read failure falls back to the strictest
// reading: asking for a reason that was not required is a nuisance, dropping
// one that was required loses information nobody can recover later.
func (s *service) guardianReasonRequired(ctx context.Context, tenantID int64) bool {
	if s.Settings == nil {
		return true
	}
	policy, err := s.Settings.ResolveStringForTenant(ctx, tenantID, configModels.KeyParentRequestReasonPolicy)
	if err != nil {
		s.Logger.Warn("parent: resolve reason policy failed, requiring a reason",
			slog.Int64("tenant_id", tenantID),
			slog.String("error", err.Error()),
		)
		return true
	}
	return usersSvc.ReasonRequiredFor(policy, false)
}

// mapExcusedRequestError translates the absence domain's sentinels into the
// parent package's, so handlers keep switching on one set. Everything else is
// wrapped, which keeps errors.Is working for the shared request sentinels
// (stale, reason required).
func mapExcusedRequestError(err error, op string) error {
	switch {
	case errors.Is(err, activeModels.ErrExcusedRequestNotFound):
		return ErrExcusedRequestNotFound
	case errors.Is(err, activeModels.ErrExcusedRequestNotPending):
		return ErrExcusedRequestNotPending
	default:
		return fmt.Errorf("parent: %s: %w", op, err)
	}
}

// EditExcusedRequest rewrites the caller's own pending absence request
// (#2267, story 37). It replaces withdrawal: a guardian who picked the wrong
// day corrects it, the request keeps its id and the co-guardians it was shared
// with stay recipients. Same gate as the withdraw it replaces — available
// while the child's care is running, ownership enforced in the absence
// service.
func (s *service) EditExcusedRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	dates []timezone.Date, note, expectedVersion string,
) (*activeModels.ExcusedAbsenceRequest, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if s.ExcusedRequests == nil {
		return nil, ErrExcusedRequestNotFound
	}
	var out *activeModels.ExcusedAbsenceRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		req, editErr := s.ExcusedRequests.EditRequest(txCtx, absenceSvc.ExcusedRequestEditInput{
			RequestID:         requestID,
			StudentID:         studentID,
			GuardianAccountID: accountID,
			ExpectedVersion:   expectedVersion,
			Dates:             dates,
			Note:              note,
			NoteRequired:      s.guardianReasonRequired(ctx, child.tenantID),
		})
		if editErr != nil {
			return editErr
		}
		out = req
		return nil
	})
	if txErr != nil {
		return nil, mapExcusedRequestError(txErr, "edit excused request")
	}
	return out, nil
}

// EditPickupChangeRequest rewrites the caller's own pending one-day pickup
// change (#2267, story 37).
func (s *service) EditPickupChangeRequest(
	ctx context.Context, accountID, studentID, requestID int64,
	date timezone.Date, pickupTime time.Time, reason, expectedVersion string,
) (*scheduleModels.CareScheduleChangeRequest, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	var out *scheduleModels.CareScheduleChangeRequest
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		req, editErr := s.CareRequests.EditRequest(txCtx, scheduleService.CareRequestEditInput{
			RequestID:         requestID,
			StudentID:         studentID,
			GuardianAccountID: accountID,
			ExpectedVersion:   expectedVersion,
			Date:              date,
			PickupTime:        pickupTime,
			Reason:            reason,
			// The reason is mandatory only while the school asks the family
			// for one (#2267, story 28).
			ReasonRequired: s.guardianReasonRequired(ctx, child.tenantID),
		})
		if editErr != nil {
			return editErr
		}
		out = req
		return nil
	})
	if txErr != nil {
		return nil, mapCareRequestError(txErr, "edit pickup change request")
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
	keys := []string{
		configModels.KeyParentSickNoteEnabled,
		configModels.KeyParentSickRequiresApproval,
		configModels.KeyParentExcusedRequiresApproval,
		configModels.KeyParentNotesEnabled,
		configModels.KeyParentPickupChangeEnabled,
		configModels.KeyGuardianParentInviteMode,
		configModels.KeyGuardianParentCanRemove,
		configModels.KeyParentMasterDataEditEnabled,
		configModels.KeyParentMasterDataRequestEnabled,
		configModels.KeyParentNewsEnabled,
		configModels.KeyParentGuardianManagementEnabled,
		configModels.KeyParentRequestReasonPolicy,
	}
	var snapshot *configService.SettingsSnapshot
	if batch, ok := s.Settings.(interface {
		ResolveManyForTenant(context.Context, int64, []string) (*configService.SettingsSnapshot, error)
	}); ok {
		snapshot, err = batch.ResolveManyForTenant(ctx, child.tenantID, keys)
		if err != nil {
			return ChildFeatureFlags{}, fmt.Errorf("parent: resolve child feature settings: %w", err)
		}
	}
	resolveBool := func(key string) (bool, error) {
		if snapshot != nil {
			return snapshot.Bool(key)
		}
		return s.Settings.ResolveBoolForTenant(ctx, child.tenantID, key)
	}
	resolveString := func(key string) (string, error) {
		if snapshot != nil {
			return snapshot.String(key)
		}
		return s.Settings.ResolveStringForTenant(ctx, child.tenantID, key)
	}

	sick, err := resolveBool(configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	sickApproval, err := resolveBool(configModels.KeyParentSickRequiresApproval)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve sick-approval setting: %w", err)
	}
	excusedApproval, err := resolveBool(configModels.KeyParentExcusedRequiresApproval)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve excused-approval setting: %w", err)
	}
	notes, err := resolveBool(configModels.KeyParentNotesEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	pickupChange, err := resolveBool(configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	inviteMode, err := resolveString(configModels.KeyGuardianParentInviteMode)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve invite mode: %w", err)
	}
	canRemove, err := resolveBool(configModels.KeyGuardianParentCanRemove)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve remove setting: %w", err)
	}
	masterEdit, err := resolveBool(configModels.KeyParentMasterDataEditEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve master-data edit setting: %w", err)
	}
	masterRequest, err := resolveBool(configModels.KeyParentMasterDataRequestEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve master-data request setting: %w", err)
	}
	mealPlan, err := s.mealPlanAvailableForTenant(ctx, child.tenantID)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve meal-plan setting: %w", err)
	}
	mealRegistration, err := s.mealRegistrationAvailableForTenant(ctx, child.tenantID)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve meal-registration setting: %w", err)
	}
	news, err := resolveBool(configModels.KeyParentNewsEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve parent-news setting: %w", err)
	}
	guardianManagement, err := resolveBool(configModels.KeyParentGuardianManagementEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve guardian-management setting: %w", err)
	}
	reasonPolicy, err := resolveString(configModels.KeyParentRequestReasonPolicy)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve reason policy: %w", err)
	}
	canEditMasterData := masterEdit && child.hasPermission(authorize.GuardianPermissionMasterDataEdit)
	canManagePickup := child.hasPermission(authorize.GuardianPermissionPickupManage)
	canManageGuardianContacts := child.hasPermission(authorize.GuardianPermissionGuardianEdit)

	// The child has left the OGS (#2487). Every WRITE capability goes off in
	// this one place rather than in each screen: the portal builds its buttons
	// from these flags, so a family sees a read-only profile instead of
	// affordances that would all end in the same 403. CareEnded travels
	// alongside so the portal can say why, and the read flags (meal plan, news)
	// stay untouched — what happened stays readable.
	if child.careEnded {
		return ChildFeatureFlags{
			CareEnded:               true,
			SickRequiresApproval:    sickApproval,
			ExcusedRequiresApproval: excusedApproval,
			MealPlanEnabled:         mealPlan,
			MealRegistrationEnabled: false,
			NewsEnabled:             news,
		}, nil
	}

	return ChildFeatureFlags{
		HasOpenChangeRequest:         s.hasOpenChangeRequest(ctx, child.tenantID, accountID, studentID),
		SickNoteEnabled:              sick && child.hasPermission(authorize.GuardianPermissionSickNoteSubmit),
		SickRequiresApproval:         sickApproval,
		ExcusedRequiresApproval:      excusedApproval,
		NotesEnabled:                 notes && child.hasPermission(authorize.GuardianPermissionNotesWrite),
		RequestSubmitEnabled:         notes && child.hasPermission(authorize.GuardianPermissionRequestSubmit),
		PickupChangeEnabled:          pickupChange && canManagePickup,
		PickupManageAllowed:          guardianManagement && canManagePickup,
		GuardianContactManageAllowed: guardianManagement && canManageGuardianContacts,
		RelatedAccountsInviteEnabled: inviteMode != configModels.ParentInviteModeDisabled,
		RelatedAccountsRemoveEnabled: canRemove && inviteMode != configModels.ParentInviteModeDisabled,
		MasterDataEditEnabled:        canEditMasterData,
		MasterDataContactEditEnabled: canEditMasterData && guardianManagement,
		MasterDataRequestEnabled:     masterRequest && child.hasPermission(authorize.GuardianPermissionMasterDataRequest),
		MealPlanEnabled:              mealPlan,
		MealRegistrationEnabled:      mealRegistration,
		NewsEnabled:                  news,
		ReasonRequired:               usersSvc.ReasonRequiredFor(reasonPolicy, false),
	}, nil
}

// hasOpenChangeRequest reports whether the child has a pending change request
// (care schedule OR master data) awaiting an OGS decision, so the parent
// overview can badge the Stammdaten entry. The lookups hit tenant-scoped/RLS
// tables, so they must run inside a tenant transaction — ChildFeatures is only
// parent-authenticated and carries no tenant context otherwise. Best-effort: a
// query error logs and yields false so a transient failure never shows a
// phantom badge.
func (s *service) hasOpenChangeRequest(ctx context.Context, tenantID, accountID, studentID int64) bool {
	open := false
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		visibility, err := s.loadRequestShareVisibility(txCtx, studentID)
		if err != nil {
			return err
		}
		if s.CareRequests != nil {
			if req, _, err := s.CareRequests.GetPendingForStudent(txCtx, studentID); err != nil {
				s.Logger.Warn("parent: pending care-request check failed",
					slog.Int64("student_id", studentID),
					slog.String("error", err.Error()),
				)
			} else if req != nil && visibility.allows(RequestShareCareSchedule, req.ID, accountID, req.SubmittedBy) {
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
			} else {
				for _, req := range pending {
					if req != nil && visibility.allows(RequestShareMasterData, req.ID, accountID, req.SubmittedBy) {
						open = true
						break
					}
				}
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
				absences = append(absences, parentVisibleStatusDay(r, accountID))
			case r.Status == activeModels.StudentStatusDayExcused &&
				r.Source == activeModels.StudentStatusSourceParent:
				// Only parent-reported excused days belong in the parents
				// portal; staff-created excused rows (planned/manual) stay
				// internal so we don't leak their note/source to guardians.
				absences = append(absences, parentVisibleStatusDay(r, accountID))
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

func parentVisibleStatusDay(row *activeModels.StudentStatusDay, accountID int64) *activeModels.StudentStatusDay {
	visible := *row
	if row.GuardianAccountID == nil || *row.GuardianAccountID != accountID {
		visible.Note = nil
	}
	return &visible
}

// MealPlanWeek returns the child's school meal plan for the Monday-Friday week
// containing weekStart. Unlike ListSickDays this is gated by the
// operations.meal_plan_enabled toggle: if the school does not run a meal plan
// the parent must not see one, so a disabled tenant yields ErrMealPlanDisabled.
func (s *service) MealPlanWeek(ctx context.Context, accountID, studentID int64, weekStart timezone.Date) ([]MealPlanEntry, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return nil, err
	}

	monday := weekStart.StartOfISOWeek()
	// Parents may only read the current and next work week. Staff can plan
	// arbitrary future (and past) weeks on the staff page; those are drafts and
	// must not be reachable through the parent proxy by supplying a crafted
	// week_start. Compare on the normalized Monday so any day within an allowed
	// week resolves the same.
	currentMonday := s.todayDate().StartOfISOWeek()
	if monday != currentMonday && monday != currentMonday.AddDays(7) {
		return nil, ErrMealPlanWeekOutOfRange
	}

	if s.MealPlan == nil {
		return nil, errors.New("parent: meal plan capability is required")
	}
	var out []MealPlanEntry
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, findErr := s.MealPlan.Week(txCtx, mealplanModule.Date(monday.String()))
		if findErr != nil {
			return findErr
		}
		out = make([]MealPlanEntry, 0, len(rows))
		for _, row := range rows {
			out = append(out, MealPlanEntry{Date: string(row.Date), Position: row.Position, Dish: row.Dish, Note: row.Note})
		}
		return nil
	})
	if txErr != nil {
		if errors.Is(txErr, mealplanModule.ErrDisabled) {
			return nil, ErrMealPlanDisabled
		}
		return nil, fmt.Errorf("parent: meal plan week: %w", txErr)
	}
	return out, nil
}

func (s *service) mealPlanAvailableForTenant(ctx context.Context, tenantID int64) (bool, error) {
	if s.MealPlan == nil {
		return false, errors.New("parent: meal plan capability is required")
	}
	var available bool
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var resolveErr error
		available, resolveErr = s.MealPlan.Available(txCtx)
		return resolveErr
	})
	return available, err
}

func (s *service) mealRegistrationAvailableForTenant(ctx context.Context, tenantID int64) (bool, error) {
	if s.MealPlan == nil {
		return false, errors.New("parent: meal plan capability is required")
	}
	var available bool
	err := tenant.WithTenantTx(ctx, s.DB, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var resolveErr error
		available, resolveErr = s.MealPlan.RegistrationAvailable(txCtx)
		return resolveErr
	})
	if errors.Is(err, mealplanModule.ErrDisabled) {
		return false, nil
	}
	return available, err
}

func (s *service) MealParticipation(ctx context.Context, accountID, studentID int64, from, to timezone.Date) (MealParticipationPlan, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return MealParticipationPlan{}, err
	}
	currentMonday := s.todayDate().StartOfISOWeek()
	if from.Before(currentMonday) || to.After(currentMonday.AddDays(11)) || to.Before(from) {
		return MealParticipationPlan{}, ErrMealParticipationOutOfRange
	}
	var plan mealplanModule.ParticipationPlan
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var readErr error
		plan, readErr = s.MealPlan.Participation(txCtx, studentID, mealplanModule.Date(from.String()), mealplanModule.Date(to.String()))
		return readErr
	})
	if err != nil {
		return MealParticipationPlan{}, mapMealParticipationError(err)
	}
	out := MealParticipationPlan{
		EffectiveFrom: string(plan.EffectiveFrom),
		CutoffTime:    plan.CutoffTime,
		Weekdays:      make([]MealWeekday, 0, len(plan.Weekdays)),
		Days:          make([]MealParticipationDay, 0, len(plan.Days)),
	}
	for _, weekday := range plan.Weekdays {
		out.Weekdays = append(out.Weekdays, MealWeekday(weekday))
	}
	for _, day := range plan.Days {
		out.Days = append(out.Days, MealParticipationDay{Date: string(day.Date), Participating: day.Participating, Source: string(day.Source), Changeable: day.Changeable})
	}
	return out, nil
}

func (s *service) ReplaceMealParticipationSchedule(ctx context.Context, accountID, studentID int64, weekdays []MealWeekday) (string, error) {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return "", err
	}
	if err := child.requireCareRunning(); err != nil {
		return "", err
	}
	moduleWeekdays := make([]mealplanModule.Weekday, 0, len(weekdays))
	for _, weekday := range weekdays {
		moduleWeekdays = append(moduleWeekdays, mealplanModule.Weekday(weekday))
	}
	var effectiveFrom mealplanModule.Date
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.requireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		var writeErr error
		effectiveFrom, writeErr = s.MealPlan.ReplaceParticipationSchedule(txCtx, mealplanModule.ReplaceParticipationSchedule{StudentID: studentID, GuardianAccountID: accountID, Weekdays: moduleWeekdays})
		return writeErr
	})
	if err != nil {
		return "", mapMealParticipationError(err)
	}
	return string(effectiveFrom), nil
}

func (s *service) SetMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date, participating bool) error {
	return s.ChangeMealParticipationDays(ctx, accountID, studentID, []MealParticipationDayChange{{Date: date, Participating: &participating}})
}

func (s *service) ClearMealParticipationDay(ctx context.Context, accountID, studentID int64, date timezone.Date) error {
	return s.ChangeMealParticipationDays(ctx, accountID, studentID, []MealParticipationDayChange{{Date: date}})
}

func (s *service) ChangeMealParticipationDays(ctx context.Context, accountID, studentID int64, changes []MealParticipationDayChange) error {
	if len(changes) == 0 || len(changes) > 10 {
		return ErrInvalidMealParticipation
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPortalAccess)
	if err != nil {
		return err
	}
	if err := child.requireCareRunning(); err != nil {
		return err
	}
	today := s.todayDate()
	seen := make(map[timezone.Date]struct{}, len(changes))
	for _, change := range changes {
		if change.Date.Before(today) || change.Date.After(today.StartOfISOWeek().AddDays(11)) {
			return ErrMealParticipationOutOfRange
		}
		if change.Date.Weekday() == time.Saturday || change.Date.Weekday() == time.Sunday {
			return ErrInvalidMealParticipation
		}
		if _, duplicate := seen[change.Date]; duplicate {
			return ErrInvalidMealParticipation
		}
		seen[change.Date] = struct{}{}
	}
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.requireCareRunningForUpdate(txCtx, studentID); err != nil {
			return err
		}
		for _, change := range changes {
			command := mealplanModule.SetParticipationDay{
				StudentID:         studentID,
				GuardianAccountID: accountID,
				Date:              mealplanModule.Date(change.Date.String()),
			}
			if change.Participating == nil {
				if err := s.MealPlan.ClearParticipationForDay(txCtx, command); err != nil {
					return err
				}
				continue
			}
			command.Participating = *change.Participating
			if err := s.MealPlan.SetParticipationForDay(txCtx, command); err != nil {
				return err
			}
		}
		return nil
	})
	return mapMealParticipationError(err)
}

func mapMealParticipationError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, mealplanModule.ErrDisabled), errors.Is(err, mealplanModule.ErrRegistrationDisabled):
		return ErrMealRegistrationDisabled
	case errors.Is(err, mealplanModule.ErrParticipationCutoff):
		return ErrMealParticipationCutoff
	case errors.Is(err, mealplanModule.ErrInvalidParticipation):
		return ErrInvalidMealParticipation
	default:
		return err
	}
}

// SubmitCareException sets the guardian-authored pickup and/or arrival override
// for a single day. The two times are the COMPLETE desired override for the
// day, mirroring the parents-portal modal (which always prefills both fields
// from the current state): a non-nil time sets that leg, a nil time clears the
// guardian row for that leg. So emptying the pickup field and saving removes the
// pickup override while keeping the arrival one, instead of silently retaining
func (s *service) SubmitCareExceptionWithReason(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason string) (*CareException, error) {
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
		child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
		if err != nil {
			return nil, err
		}
		if s.guardianReasonRequired(ctx, child.tenantID) {
			return nil, ErrCareExceptionReasonRequired
		}
	}
	return s.submitCareException(ctx, accountID, studentID, date, pickupTime, &trimmedReason)
}

func (s *service) SubmitPickupChangeRequest(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime time.Time, reason string, recipientGuardianProfileIDs []int64) (*scheduleModels.CareScheduleChangeRequest, error) {
	reason = strings.TrimSpace(reason)
	if pickupTime.IsZero() {
		return nil, ErrNoCareException
	}
	if utf8.RuneCountInString(reason) > 255 {
		return nil, ErrCareExceptionReasonTooLong
	}
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	// Whether the reason is mandatory is a per-school setting, so it can only
	// be decided once the child — and with it the tenant — is known (#2267,
	// story 28). The cheap shape checks above still run first.
	reasonRequired := s.guardianReasonRequired(ctx, child.tenantID)
	if reason == "" && reasonRequired {
		return nil, ErrCareExceptionReasonRequired
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}
	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return nil, ErrPickupChangeDisabled
	}
	today := s.todayDate()
	if date.Before(today) {
		return nil, ErrPastCareDate
	}
	if date.After(timezone.NewDate(today.Year(), today.Month()+2, today.Day())) {
		return nil, ErrCareDateTooFar
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}

	var result *scheduleModels.CareScheduleChangeRequest
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if err := scheduleService.LockCareExceptionDay(txCtx, s.DB, studentID, date); err != nil {
			return err
		}
		staffOwned, checkErr := s.pickupHasStaffException(txCtx, studentID, date)
		if checkErr != nil {
			return checkErr
		}
		if staffOwned {
			return ErrCareExceptionConflict
		}
		alreadyLeft, checkErr := s.childAlreadyLeftToday(txCtx, studentID, date, today)
		if checkErr != nil {
			return checkErr
		}
		if alreadyLeft {
			return ErrCareExceptionAlreadyLeft
		}
		created, createErr := s.CareRequests.CreatePickupChange(txCtx, scheduleService.PickupChangeCreateInput{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Date:              date,
			PickupTime:        pickupTime,
			Reason:            reason,
			// The reason is mandatory only while the school asks the family
			// for one (#2267, story 28).
			ReasonRequired: reasonRequired,
		})
		if createErr != nil {
			return createErr
		}
		// Same transaction as the request row, so a refused share never leaves
		// a request the family cannot see (#2267).
		if shareErr := s.ShareRequestInTx(
			txCtx, accountID, studentID, RequestSharePickupChange, created.ID, recipientGuardianProfileIDs,
		); shareErr != nil {
			return shareErr
		}
		result = created
		return nil
	})
	if err != nil {
		return nil, mapCareRequestError(err, "submit pickup change request")
	}
	return result, nil
}

func (s *service) ListPickupChangeRequests(ctx context.Context, accountID, studentID int64) ([]*scheduleModels.CareScheduleChangeRequest, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.CareRequests == nil {
		return nil, errors.New("parent: pickup change request service not configured")
	}
	var rows []*scheduleModels.CareScheduleChangeRequest
	err = tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		var listErr error
		rows, listErr = s.CareRequests.ListPickupChangeRequests(txCtx, studentID, time.Now().AddDate(0, -2, 0))
		if listErr != nil {
			return listErr
		}
		visibility, visibilityErr := s.loadRequestShareVisibility(txCtx, studentID)
		if visibilityErr != nil {
			return visibilityErr
		}
		visible := rows[:0]
		for _, row := range rows {
			if row != nil && visibility.allows(RequestSharePickupChange, row.ID, accountID, row.SubmittedBy) {
				visible = append(visible, row)
			}
		}
		rows = visible
		s.enrichLegacyPickupChangeRequests(txCtx, studentID, rows)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("parent: list pickup change requests: %w", err)
	}
	return rows, nil
}

func (s *service) enrichLegacyPickupChangeRequests(ctx context.Context, studentID int64, rows []*scheduleModels.CareScheduleChangeRequest) {
	if s.PickupSchedules == nil {
		return
	}
	for _, row := range rows {
		if row == nil || row.Status != scheduleModels.CareRequestStatusPending {
			continue
		}
		if previous, _ := row.Payload["previous_pickup_time"].(string); previous != "" {
			continue
		}
		dateRaw, _ := row.Payload["date"].(string)
		date, err := timezone.ParseDate(dateRaw)
		if err != nil {
			s.Logger.Warn("parent: legacy pickup request has invalid date",
				"request_id", row.ID,
				"student_id", studentID,
			)
			continue
		}
		effective, err := s.PickupSchedules.GetEffectivePickupTimeForDate(ctx, studentID, date)
		if err != nil {
			s.Logger.Warn("parent: resolve legacy pickup request baseline failed",
				"request_id", row.ID,
				"student_id", studentID,
				"error", err,
			)
			continue
		}
		if effective == nil || effective.PickupTime == nil {
			continue
		}
		payload := make(map[string]any, len(row.Payload)+1)
		for key, value := range row.Payload {
			payload[key] = value
		}
		payload["previous_pickup_time"] = effective.PickupTime.Format("15:04")
		row.Payload = payload
	}
}

func (s *service) submitCareException(ctx context.Context, accountID, studentID int64, date timezone.Date, pickupTime *time.Time, reason *string) (*CareException, error) {
	if pickupTime == nil {
		return nil, ErrNoCareException
	}

	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return nil, err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return nil, err
	}

	enabled, err := s.Settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentPickupChangeEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve pickup-change setting: %w", err)
	}
	if !enabled {
		return nil, ErrPickupChangeDisabled
	}

	today := s.todayDate()
	if date.Before(today) {
		return nil, ErrPastCareDate
	}
	// Cap how far ahead a parent may set an exception: two calendar months,
	// mirroring the parent-portal list window (parseSickDayRange) so a created
	// entry can never fall outside the range the UI shows.
	maxDate := timezone.NewDate(today.Year(), today.Month()+2, today.Day())
	if date.After(maxDate) {
		return nil, ErrCareDateTooFar
	}

	guardianID := accountID
	var result *CareException
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if err := scheduleService.LockCareExceptionDay(txCtx, s.DB, studentID, date); err != nil {
			return err
		}

		alreadyLeft, err := s.childAlreadyLeftToday(txCtx, studentID, date, today)
		if err != nil {
			return err
		}
		if alreadyLeft {
			return ErrCareExceptionAlreadyLeft
		}

		staffOwned, err := s.pickupHasStaffException(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if staffOwned {
			return ErrCareExceptionConflict
		}

		if err := s.applyGuardianPickupException(txCtx, studentID, child.tenantID, date, pickupTime, reason, guardianID); err != nil {
			return err
		}

		// Parent-set day pickup times couple with the per-block excusal the
		// same way staff-set ones do (#2360): a pull-forward against the
		// weekly baseline excuses the blocks after the new time; moving it
		// back (or clearing it) releases them again.
		if s.PickupAutoExcusal != nil {
			if row, findErr := s.PickupExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date); findErr != nil {
				return findErr
			} else if row != nil {
				if _, err := s.PickupAutoExcusal.Sync(txCtx, row.ID); err != nil {
					return err
				}
			}
		}

		merged, err := s.loadCareException(txCtx, studentID, date)
		if err != nil {
			return err
		}
		result = merged

		capturedTenant := child.tenantID
		pillBody := careExceptionEventBody(date, pickupTime, nil)
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
		slog.Bool("has_arrival", false),
	)
	return result, nil
}

func (s *service) childAlreadyLeftToday(ctx context.Context, studentID int64, date, today timezone.Date) (bool, error) {
	if date != today || s.AttendanceRepo == nil {
		return false, nil
	}
	rows, err := s.AttendanceRepo.FindByStudentAndDate(ctx, studentID, date)
	if err != nil {
		return false, err
	}
	facts := todayStatusFacts{}
	applyAttendanceRows(&facts, rows)
	return facts.HasAttendanceToday && facts.CheckOut != "", nil
}

// Only the PICKUP leg of the day can block a parent: since arrival times became
// OGS-only, a staff-set Bringzeit says nothing about who owns the Abholzeit, and
// treating it as a conflict would let one OGS entry silently forbid every parent
// pickup change for that day (TestSubmitCareExceptionWithReasonPreservesExistingArrival).
// An AUTO-derived partial absence is the school's own bookkeeping, not a
// decision, so only a manual one counts (#2360).
func (s *service) pickupHasStaffException(ctx context.Context, studentID int64, date timezone.Date) (bool, error) {
	pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
	if err != nil {
		return false, err
	}
	return pickup != nil && (pickup.Source == scheduleModels.ExceptionSourceStaff || pickup.HasManualPartialAbsence()), nil
}

// applyGuardianPickupException reconciles the guardian-owned pickup leg for the
// date with the submitted time: a non-nil time creates or updates the guardian
// row, a nil time removes any existing guardian row (the parent cleared that
// leg). The day-level staff check in SubmitCareException already rejects
// staff-owned days; the per-leg guard here keeps the helper safe on its own and
// protects against a staff row appearing mid-transaction — a staff leg is never
// touched (neither overwritten nor deleted).
func (s *service) applyGuardianPickupException(ctx context.Context, studentID, tenantID int64, date timezone.Date, pickupTime *time.Time, reason *string, guardianID int64) error {
	return applyGuardianTimeException(ctx, pickupTime,
		func(ctx context.Context) (*scheduleModels.StudentPickupException, error) {
			return s.PickupExceptionRepo.FindByStudentIDAndDate(ctx, studentID, date)
		},
		func(e *scheduleModels.StudentPickupException) string { return e.Source },
		func(ctx context.Context, e *scheduleModels.StudentPickupException) error {
			// Release the auto excusal's block absences BEFORE the row goes
			// away — the FK's ON DELETE SET NULL would otherwise strand them
			// as absent with no provenance to restore from (#2360).
			if s.PickupAutoExcusal != nil {
				if err := s.PickupAutoExcusal.ReleaseBeforeDelete(ctx, e); err != nil {
					return err
				}
			}
			return s.PickupExceptionRepo.Delete(ctx, e.ID)
		},
		func(ctx context.Context, e *scheduleModels.StudentPickupException) error {
			e.PickupTime = pickupTime
			e.Reason = reason
			e.Source = scheduleModels.ExceptionSourceGuardian
			e.CreatedBy = 0
			e.CreatedByGuardian = &guardianID
			// Re-anchor scanned TIME values (e.g. a carried-over excused_from)
			// before the full-row update — the driver scans them onto year 0.
			e.NormalizeWallClockTimes()
			return s.PickupExceptionRepo.Update(ctx, e)
		},
		func(ctx context.Context) error {
			entity := &scheduleModels.StudentPickupException{
				StudentID:         studentID,
				ExceptionDate:     date,
				PickupTime:        pickupTime,
				Reason:            reason,
				Source:            scheduleModels.ExceptionSourceGuardian,
				CreatedByGuardian: &guardianID,
			}
			entity.SetTenantID(tenantID)
			return s.PickupExceptionRepo.Create(ctx, entity)
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
		out.PickupSource = pickup.Source
		if pickup.Source == scheduleModels.ExceptionSourceGuardian {
			out.Reason = pickup.Reason
		}
		out.UpdatedAt = pickup.UpdatedAt
		if pickup.Source == scheduleModels.ExceptionSourceStaff {
			staffOwned = true
		}
	}
	if arrival != nil {
		out.ArrivalTime = arrival.ExpectedArrival
		if out.Reason == nil && arrival.Source == scheduleModels.ExceptionSourceGuardian {
			out.Reason = arrival.Reason
		}
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
		out = mergeCareExceptions(pickups, arrivals, accountID)
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list care exceptions: %w", txErr)
	}
	return out, nil
}

// DeleteCareException removes only the guardian-authored pickup exception for
// the date. Arrival and staff rows are left untouched. Deleting a day with
// nothing guardian-owned
// is a no-op (and skips the broadcast). Not gated by the feature toggle for the
// same reason as ListCareExceptions: clearing one's own override stays available.
func (s *service) DeleteCareException(ctx context.Context, accountID, studentID int64, date timezone.Date) error {
	child, err := s.resolvePermittedChild(ctx, accountID, studentID, authorize.GuardianPermissionPickupManage)
	if err != nil {
		return err
	}
	// A child whose care at this school has ended keeps read access to
	// what happened, but nothing new can be submitted for them (#2487).
	if err := child.requireCareRunning(); err != nil {
		return err
	}

	today := s.todayDate()
	if date.Before(today) {
		return ErrPastCareDate
	}

	pickupDeleted := false
	txErr := tenant.WithTenantTx(ctx, s.DB, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		student, err := s.StudentRepo.FindByIDForUpdate(txCtx, studentID)
		if err != nil {
			return err
		}
		if student.CareEndedOn(s.todayDate()) {
			return ErrChildCareEnded
		}
		if err := scheduleService.LockCareExceptionDay(txCtx, s.DB, studentID, date); err != nil {
			return err
		}
		alreadyLeft, err := s.childAlreadyLeftToday(txCtx, studentID, date, today)
		if err != nil {
			return err
		}
		if alreadyLeft {
			return ErrCareExceptionAlreadyLeft
		}

		pickup, err := s.PickupExceptionRepo.FindByStudentIDAndDate(txCtx, studentID, date)
		if err != nil {
			return err
		}
		if pickup != nil && pickup.HasManualPartialAbsence() {
			return ErrCareExceptionConflict
		}
		if pickup != nil && pickup.Source == scheduleModels.ExceptionSourceGuardian {
			// An auto-derived excusal follows the pickup time: withdrawing the
			// override releases the excused blocks before the row is removed
			// (#2360).
			if s.PickupAutoExcusal != nil {
				if err := s.PickupAutoExcusal.ReleaseBeforeDelete(txCtx, pickup); err != nil {
					return err
				}
			}
			if err := s.PickupExceptionRepo.Delete(txCtx, pickup.ID); err != nil {
				return err
			}
			pickupDeleted = true
		}
		if pickupDeleted {
			capturedTenant := child.tenantID
			pillBody := "Korrektur: Abholung " + date.Format("02.01.") + " zurückgezogen"
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
func mergeCareExceptions(pickups []*scheduleModels.StudentPickupException, arrivals []*scheduleModels.StudentArrivalException, accountID int64) []*CareException {
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
		ce.PickupSource = p.Source
		if p.Source == scheduleModels.ExceptionSourceGuardian && guardianAuthoredBy(p.CreatedByGuardian, accountID) {
			ce.Reason = p.Reason
		}
		// A pickup row with no time is an absence marker, not "no override". Carry
		// that distinction to the parent UI so a staff-set "not coming today" row
		// resolves to an absence rather than falling through to the base plan.
		ce.PickupAbsent = p.PickupTime == nil
		if p.UpdatedAt.After(ce.UpdatedAt) {
			ce.UpdatedAt = p.UpdatedAt
		}
		if p.Source == scheduleModels.ExceptionSourceStaff || p.HasManualPartialAbsence() {
			ce.Source = scheduleModels.ExceptionSourceStaff
		}
	}
	for _, a := range arrivals {
		ce := get(a.ExceptionDate)
		ce.ArrivalTime = a.ExpectedArrival
		if ce.Reason == nil && a.Source == scheduleModels.ExceptionSourceGuardian && guardianAuthoredBy(a.CreatedByGuardian, accountID) {
			ce.Reason = a.Reason
		}
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

func guardianAuthoredBy(author *int64, accountID int64) bool {
	return author != nil && *author == accountID
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
