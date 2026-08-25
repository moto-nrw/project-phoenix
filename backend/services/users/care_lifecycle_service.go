package users

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// MaxCareExitBatchSize bounds one "Betreuung beenden" action. It matches the
// selection cap the child management already enforces, so a selection the UI
// allows can always be confirmed.
const MaxCareExitBatchSize = 500

// Errors the care lifecycle raises. All messages are German because they reach
// the user unchanged.
var (
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitNoStudents = errors.New("Bitte wählen Sie mindestens ein Kind aus.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitTooManyStudents = errors.New("Es können höchstens 500 Kinder auf einmal beendet werden.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitDayInPast = errors.New("Der letzte Betreuungstag darf nicht in der Vergangenheit liegen.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitPreviewChanged = errors.New("Die Betreuung wurde nicht beendet. Die Daten haben sich seit der Vorschau geändert. Bitte prüfen Sie die Vorschau erneut.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitBlocked = errors.New("Die Betreuung wurde nicht beendet.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitNotPlanned = errors.New("Für dieses Kind ist kein Ende der Betreuung geplant.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareExitAlreadyEffective = errors.New("Die Betreuung ist bereits beendet und kann nicht mehr storniert werden. Nutzen Sie „Betreuung wieder aufnehmen“.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareResumeNotEnded = errors.New("Die Betreuung dieses Kindes läuft noch.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareResumeMissing = errors.New(careBlockerResumeMissing)
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareResumeStartInPast = errors.New("Der neue Beginn darf nicht in der Vergangenheit liegen.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareResumeNotChecked = errors.New("Bitte bestätigen Sie zuerst die Prüfung. Gruppe, Angebote, Wochenplan und Zeiten bleiben sonst ungeprüft.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareWithdrawalNotFound = errors.New("Diese Abmeldung gibt es nicht oder nicht mehr.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareWithdrawalAfterGap = errors.New("Der letzte Betreuungstag muss vor dem ersten Tag ohne Buchung liegen.")
)

// CareWithdrawalDateError is a client-correctable retroactive-date conflict
// whose message explains the concrete boundary.
type CareWithdrawalDateError struct{ Message string }

func (e *CareWithdrawalDateError) Error() string { return e.Message }

// Per-child blocker sentences. They name the child's own situation, never a
// technical condition, because they are listed one per child under the
// headline "Die Betreuung wurde nicht beendet".
const (
	careBlockerUnknown       = "Dieses Kind gibt es in Ihrer Schule nicht (mehr)."
	careBlockerAlumnus       = "Dieses Kind wurde beim Jahrgangswechsel abgemeldet. Sie finden es unter Jahrgangswechsel, Bereich Abgänge."
	careBlockerAlreadyEnded  = "Die Betreuung dieses Kindes ist bereits beendet."
	careBlockerBeforeStart   = "Die Betreuung dieses Kindes beginnt erst am %s. Bitte wählen Sie einen späteren letzten Betreuungstag."
	careBlockerResumeMissing = "Für dieses Kind ist keine beendete Betreuung hinterlegt."
)

// CareExitInput is the whole action: one set of children, one last care day,
// one reason. Every acceptance criterion that says "alle Kinder einer Aktion"
// holds because there is nowhere to put a per-child value.
type CareExitInput struct {
	StudentIDs  []int64
	LastCareDay timezone.Date
	Reason      string
	ReasonNote  string
}

// CareExitImpact is what ending the care will do to ONE child, named.
type CareExitImpact struct {
	StudentID          int64
	FirstName          string
	LastName           string
	SchoolClass        string
	PlannedRosterRows  int
	ActivityBookings   int
	OpenParentRequests int
	HasRFIDTag         bool
	CurrentlyPresent   bool
	SourceOfferings    []userModels.CareExitSourceOffering
	WeeklyPlans        []string
	// PlannedEndsOn is the exit already recorded for this child, if any. A
	// second run over the same child is a CHANGE, not a blocker.
	PlannedEndsOn *timezone.Date
	// Blocker is empty when the child can be ended, otherwise the German
	// sentence explaining why not.
	Blocker string
}

// CanEnd reports whether this child is free of blockers.
func (i CareExitImpact) CanEnd() bool { return i.Blocker == "" }

// CareExitPreview is the immutable state a confirmation has to quote back.
type CareExitPreview struct {
	Token       string
	LastCareDay timezone.Date
	Reason      string
	ReasonNote  string
	Students    []CareExitImpact
	Blocked     bool
}

// CareExitResult reports what the confirmation actually changed.
type CareExitResult struct {
	StudentsEnded     int
	RosterRowsRemoved int
	BookingsEnded     int
}

// CareResumeInput reopens one child's care.
type CareResumeInput struct {
	StudentID      int64
	NewStart       timezone.Date
	ActorAccountID int64
	// Checked is the explicit confirmation that the previous group, offerings,
	// weekly plan and arrival/pickup times were reviewed. The acceptance
	// criteria require it, and nothing is re-enabled automatically, so the
	// flag is the only thing standing between "resumed" and "resumed with a
	// year-old plan nobody looked at".
	Checked bool
}

// --- collaborator contracts ------------------------------------------------

// CareExitTagReleaser frees the physical bracelets. Implemented by the grade
// transition repository, which already owns exactly this operation — a second
// copy would be one more place that forgets a column.
type CareExitTagReleaser interface {
	ReleaseStudentTagsByIDs(ctx context.Context, studentIDs []int64) (map[int64]string, error)
}

// CareLifecycleService is the ONE care-lifecycle contract. Manual single and
// batch actions use it, and the guided close-out after a full withdrawal
// (#2424) is meant to use it too — there must not be a second way for a child
// to leave the OGS.
type CareLifecycleService interface {
	// Preview describes what ending the care would do, per child.
	Preview(ctx context.Context, input CareExitInput) (*CareExitPreview, error)
	// Confirm ends the care for exactly the previewed state, or changes
	// nothing at all.
	Confirm(ctx context.Context, token string, input CareExitInput, actorAccountID int64) (*CareExitResult, error)
	// Cancel withdraws exits that have not taken effect yet.
	Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error)
	// Resume reopens the care of one child from a new start day.
	Resume(ctx context.Context, input CareResumeInput) error
	// ListEnded is the archive view.
	ListEnded(ctx context.Context, filter userModels.CareExitListFilter) ([]*userModels.EndedCare, int, error)
	// RecordedExitStudentIDs reports which of the given children carry a
	// recorded exit. The child management needs to tell a manual "Betreuung
	// beenden" apart from the ordinary end of an enrolment phase — both write
	// the same interval, only the first can be changed or cancelled here.
	RecordedExitStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error)
	// ApplyDueEffects performs the effect-day housekeeping for every child
	// whose care ended before asOf: closing what is still open, freeing the
	// bracelet, closing the open parent requests. Idempotent.
	ApplyDueEffects(ctx context.Context, asOf timezone.Date) (int, error)
	ReconcileAuthoritativeBookingChange(ctx context.Context, change userModels.CareWithdrawalBookingChange) error
	PreviewBookingAuthorityImpact(ctx context.Context, on timezone.Date) (*BookingAuthorityImpact, error)
	ApplyBookingAuthoritySetting(ctx context.Context, on timezone.Date, enabled bool) (*BookingAuthorityImpact, error)
	ResolveListParticipation(ctx context.Context, studentIDs []int64, on, today timezone.Date, includePending bool) (*CareParticipationResolution, error)
	ParticipatingStudentIDs(ctx context.Context, studentIDs []int64, on timezone.Date, actuallyPresent map[int64]bool) (map[int64]bool, error)
	AdministrativelyVisibleStudentIDs(ctx context.Context, studentIDs []int64, on timezone.Date, actuallyPresent map[int64]bool) (map[int64]bool, error)
	ParticipatingStudentIDsByDate(ctx context.Context, studentIDs []int64, from, to timezone.Date) (map[timezone.Date]map[int64]bool, error)
	ListPendingWithdrawals(ctx context.Context, filter userModels.CareWithdrawalCompletionFilter) ([]*userModels.CareWithdrawalCompletion, int, error)
	ListResolvedWithdrawals(ctx context.Context, filter userModels.CareWithdrawalCompletionFilter) ([]*userModels.CareWithdrawalCompletion, int, error)
	GetPendingWithdrawal(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error)
	PreviewWithdrawalCareEnd(ctx context.Context, completionID int64, input CareExitInput) (*CareExitPreview, error)
	ConfirmWithdrawalCareEnd(ctx context.Context, completionID int64, token string, input CareExitInput, actorAccountID int64) (*CareExitResult, error)
	PreviewWithdrawalDeletion(ctx context.Context, completionID int64) (*StudentDeletionPreview, error)
	DeleteWithdrawal(ctx context.Context, completionID int64, input StudentDeletionInput) (*StudentDeletionResult, error)
}

// CareParticipationResolution is the service-owned dated visibility decision
// consumed by list/report adapters. CandidateIDs contains the resolved school
// population when a caller supplied no narrower candidate set.
type CareParticipationResolution struct {
	CandidateIDs       []int64
	ParticipatingIDs   map[int64]bool
	ActuallyPresentIDs map[int64]bool
}

type careLifecycleService struct {
	studentRepo           userModels.StudentRepository
	personRepo            userModels.PersonRepository
	careExitRepo          userModels.CareExitRepository
	cleanupRepo           userModels.CareExitCleanupRepository
	withdrawalRepo        userModels.CareWithdrawalCompletionRepository
	bookingsAuthoritative func(context.Context) (bool, error)
	tagReleaser           CareExitTagReleaser
	auditService          StudentAuditService
	lockCareBookingWrites func(context.Context) error
	studentDeletion       StudentDeletionService
	txHandler             *modelBase.TxHandler
	logger                *slog.Logger
}

// CareLifecycleDependencies wires the service. Every field is required except
// the logger; a nil collaborator would silently skip a documented effect.
type CareLifecycleDependencies struct {
	StudentRepo     userModels.StudentRepository
	PersonRepo      userModels.PersonRepository
	CareExitRepo    userModels.CareExitRepository
	CleanupRepo     userModels.CareExitCleanupRepository
	WithdrawalRepo  userModels.CareWithdrawalCompletionRepository
	TagReleaser     CareExitTagReleaser
	AuditService    StudentAuditService
	StudentDeletion StudentDeletionService
	// LockCareBookingWrites is the same transaction-scoped gate used by
	// authoritative offering adjustments. Taking it before any plan lock makes
	// rebooking and care-end confirmation a total order instead of allowing a
	// stale exit to commit beside newly booked care.
	LockCareBookingWrites func(context.Context) error
	BookingsAuthoritative func(context.Context) (bool, error)
	DB                    *bun.DB
	Logger                *slog.Logger
}

// NewCareLifecycleService builds the service.
func NewCareLifecycleService(deps CareLifecycleDependencies) CareLifecycleService {
	return &careLifecycleService{
		studentRepo:           deps.StudentRepo,
		personRepo:            deps.PersonRepo,
		careExitRepo:          deps.CareExitRepo,
		cleanupRepo:           deps.CleanupRepo,
		withdrawalRepo:        deps.WithdrawalRepo,
		bookingsAuthoritative: deps.BookingsAuthoritative,
		tagReleaser:           deps.TagReleaser,
		auditService:          deps.AuditService,
		studentDeletion:       deps.StudentDeletion,
		lockCareBookingWrites: deps.LockCareBookingWrites,
		txHandler:             modelBase.NewTxHandler(deps.DB),
		logger:                deps.Logger,
	}
}

// WireCareWithdrawalDeletion attaches the deletion service after both
// services have been constructed by the factory.
func WireCareWithdrawalDeletion(lifecycle CareLifecycleService, deletion StudentDeletionService) {
	setter, ok := lifecycle.(interface{ SetStudentDeletionService(StudentDeletionService) })
	if !ok {
		panic("care lifecycle service does not support student-deletion wiring")
	}
	setter.SetStudentDeletionService(deletion)
}

func (s *careLifecycleService) SetStudentDeletionService(deletion StudentDeletionService) {
	s.studentDeletion = deletion
}

func (s *careLifecycleService) PreviewWithdrawalDeletion(ctx context.Context, completionID int64) (*StudentDeletionPreview, error) {
	completion, err := s.GetPendingWithdrawal(ctx, completionID)
	if err != nil {
		return nil, err
	}
	if s.studentDeletion == nil {
		return nil, errors.New("care lifecycle: student deletion service is not configured")
	}
	return s.studentDeletion.Preview(ctx, *completion.StudentID)
}

func (s *careLifecycleService) DeleteWithdrawal(ctx context.Context, completionID int64, input StudentDeletionInput) (*StudentDeletionResult, error) {
	if s.studentDeletion == nil {
		return nil, errors.New("care lifecycle: student deletion service is not configured")
	}
	var result *StudentDeletionResult
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if s.lockCareBookingWrites != nil {
			if err := s.lockCareBookingWrites(txCtx); err != nil {
				return fmt.Errorf("care lifecycle: lock care booking writes for deletion: %w", err)
			}
		}
		completion, err := s.withdrawalRepo.FindByIDForUpdate(txCtx, completionID)
		if err != nil {
			return err
		}
		if completion == nil || completion.State != userModels.CareWithdrawalStatePending || completion.StudentID == nil {
			return userModels.ErrCareWithdrawalAlreadyResolved
		}
		input.StudentID = *completion.StudentID
		resolved, err := s.withdrawalRepo.MarkDeleted(txCtx, completionID, input.ActorAccountID, time.Now())
		if err != nil {
			return err
		}
		if !resolved {
			return userModels.ErrCareWithdrawalAlreadyResolved
		}
		result, err = s.studentDeletion.Delete(txCtx, input)
		return err
	})
	return result, err
}

func (s *careLifecycleService) getLogger() *slog.Logger {
	if s.logger == nil {
		return slog.Default()
	}
	return s.logger
}

// ---------------------------------------------------------------------------

func (s *careLifecycleService) Preview(ctx context.Context, input CareExitInput) (*CareExitPreview, error) {
	normalized, err := normalizeCareExitInput(input)
	if err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, normalized, false, nil)
}

func (s *careLifecycleService) Confirm(
	ctx context.Context,
	token string,
	input CareExitInput,
	actorAccountID int64,
) (*CareExitResult, error) {
	return s.confirm(ctx, nil, token, input, actorAccountID, false)
}

func (s *careLifecycleService) ConfirmWithdrawalCareEnd(
	ctx context.Context,
	completionID int64,
	token string,
	input CareExitInput,
	actorAccountID int64,
) (*CareExitResult, error) {
	completion, err := s.GetPendingWithdrawal(ctx, completionID)
	if err != nil {
		return nil, err
	}
	input.StudentIDs = []int64{*completion.StudentID}
	return s.confirm(ctx, completion, token, input, actorAccountID, true)
}

func (s *careLifecycleService) confirm(
	ctx context.Context,
	completionSnapshot *userModels.CareWithdrawalCompletion,
	token string,
	input CareExitInput,
	actorAccountID int64,
	allowPast bool,
) (*CareExitResult, error) {
	state, err := newCareExitConfirmation(completionSnapshot, token, input, actorAccountID, allowPast)
	if err != nil {
		return nil, err
	}
	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		return s.applyCareExitConfirmation(txCtx, state)
	})
	if err != nil {
		return nil, err
	}
	s.logCareExitConfirmation(state)
	return &state.result, nil
}

type careExitConfirmation struct {
	completion     *userModels.CareWithdrawalCompletion
	token          string
	input          CareExitInput
	actorAccountID int64
	studentIDs     []int64
	before         map[int64]*userModels.Student
	exits          map[int64]*userModels.CareExit
	result         CareExitResult
}

func newCareExitConfirmation(
	completion *userModels.CareWithdrawalCompletion,
	token string,
	input CareExitInput,
	actorAccountID int64,
	allowPast bool,
) (*careExitConfirmation, error) {
	normalized, err := normalizeCareExitInputMode(input, allowPast)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrCareExitPreviewChanged
	}
	return &careExitConfirmation{
		completion: completion, token: token, input: normalized,
		actorAccountID: actorAccountID,
	}, nil
}

func (s *careLifecycleService) logCareExitConfirmation(state *careExitConfirmation) {
	s.getLogger().Info("care ended",
		slog.Int("students", state.result.StudentsEnded),
		slog.String("last_care_day", state.input.LastCareDay.String()),
		slog.String("reason", state.input.Reason),
		slog.Int64("actor_account_id", state.actorAccountID),
	)
}

func (s *careLifecycleService) applyCareExitConfirmation(ctx context.Context, state *careExitConfirmation) error {
	if err := s.prepareLockedCareExitPreview(ctx, state); err != nil {
		return err
	}
	if err := s.loadCareExitBaseline(ctx, state); err != nil {
		return err
	}
	if _, err := s.cleanupRepo.RestoreRemovals(ctx, state.studentIDs); err != nil {
		return err
	}
	if _, err := s.studentRepo.SetEnrolledUntilByIDs(ctx, state.studentIDs, &state.input.LastCareDay); err != nil {
		return err
	}
	if err := s.upsertCareExitRecords(ctx, state); err != nil {
		return err
	}
	if err := s.cleanupConfirmedCareExit(ctx, state); err != nil {
		return err
	}
	if err := s.finishCareExitConfirmation(ctx, state); err != nil {
		return err
	}
	state.result.StudentsEnded = len(state.studentIDs)
	return nil
}

func (s *careLifecycleService) prepareLockedCareExitPreview(ctx context.Context, state *careExitConfirmation) error {
	if s.lockCareBookingWrites != nil {
		if err := s.lockCareBookingWrites(ctx); err != nil {
			return fmt.Errorf("care lifecycle: lock care booking writes: %w", err)
		}
	}
	preview, err := s.buildPreview(ctx, state.input, true, careWithdrawalOfferings(state.completion))
	if err != nil {
		return err
	}
	if state.completion != nil {
		preview, err = s.refreshLockedWithdrawalPreview(ctx, state)
		if err != nil {
			return err
		}
	}
	if !equalCareToken(preview.Token, state.token) {
		return ErrCareExitPreviewChanged
	}
	if preview.Blocked {
		return ErrCareExitBlocked
	}
	state.studentIDs = careExitPreviewStudentIDs(preview)
	return nil
}

func careWithdrawalOfferings(
	completion *userModels.CareWithdrawalCompletion,
) map[int64][]userModels.CareExitSourceOffering {
	if completion == nil || completion.StudentID == nil {
		return nil
	}
	return map[int64][]userModels.CareExitSourceOffering{
		*completion.StudentID: completion.SourceOfferings,
	}
}

func (s *careLifecycleService) refreshLockedWithdrawalPreview(
	ctx context.Context, state *careExitConfirmation,
) (*CareExitPreview, error) {
	completion, err := s.withdrawalRepo.FindByIDForUpdate(ctx, state.completion.ID)
	if err != nil {
		return nil, err
	}
	if completion == nil {
		return nil, userModels.ErrCareWithdrawalAlreadyResolved
	}
	if !withdrawalMatchesCareExit(completion, state.input.StudentIDs) {
		return nil, userModels.ErrCareWithdrawalAlreadyResolved
	}
	if err := s.validateWithdrawalCareEnd(ctx, completion, state.input); err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, state.input, false, careWithdrawalOfferings(completion))
}

func withdrawalMatchesCareExit(completion *userModels.CareWithdrawalCompletion, studentIDs []int64) bool {
	return completion != nil && completion.State == userModels.CareWithdrawalStatePending &&
		completion.StudentID != nil && len(studentIDs) == 1 && studentIDs[0] == *completion.StudentID
}

func careExitPreviewStudentIDs(preview *CareExitPreview) []int64 {
	ids := make([]int64, 0, len(preview.Students))
	for _, impact := range preview.Students {
		ids = append(ids, impact.StudentID)
	}
	return ids
}

func (s *careLifecycleService) loadCareExitBaseline(ctx context.Context, state *careExitConfirmation) error {
	before, err := s.studentRepo.FindByIDs(ctx, state.studentIDs)
	if err != nil {
		return err
	}
	for id, student := range before {
		before[id] = cloneCareFields(student)
	}
	exits, err := s.careExitRepo.FindByStudentIDs(ctx, state.studentIDs)
	if err != nil {
		return err
	}
	state.before, state.exits = before, exits
	return nil
}

func (s *careLifecycleService) upsertCareExitRecords(ctx context.Context, state *careExitConfirmation) error {
	for _, id := range state.studentIDs {
		exit := &userModels.CareExit{
			StudentID: id, Reason: state.input.Reason, RecordedBy: &state.actorAccountID,
		}
		if state.completion != nil {
			completionID := state.completion.ID
			exit.WithdrawalCompletionID = &completionID
		}
		if state.exits[id] == nil {
			exit.PreviousEnrolledUntil = state.before[id].EnrolledUntil
		}
		if state.input.ReasonNote != "" {
			note := state.input.ReasonNote
			exit.ReasonNote = &note
		}
		if err := s.careExitRepo.Upsert(ctx, exit); err != nil {
			return err
		}
	}
	return nil
}

func (s *careLifecycleService) cleanupConfirmedCareExit(ctx context.Context, state *careExitConfirmation) error {
	removed, err := s.cleanupRepo.DeletePlannedByStudentIDsAfter(ctx, state.studentIDs, state.input.LastCareDay)
	if err != nil {
		return err
	}
	state.result.RosterRowsRemoved = removed
	validUntil := state.input.LastCareDay.AddDays(1)
	capped, err := s.cleanupRepo.CapByStudentIDs(ctx, state.studentIDs, validUntil)
	if err != nil {
		return err
	}
	// Recurring arrival and pickup plans have no date range. Keep them through
	// a future last care day, but end them immediately once that day has passed.
	endSourceBookings := s.cleanupRepo.EndSourceBookings
	if state.input.LastCareDay.Before(timezone.TodayDate()) {
		endSourceBookings = s.cleanupRepo.EndSourceBookingsAndSchedules
	}
	// A completed care exit ends every remaining source booking, including
	// non-care offers and bookings created by another enrollment request. The
	// completion's source child is audit context, not a cleanup boundary.
	sourceEnded, err := endSourceBookings(ctx, state.studentIDs, validUntil, nil)
	if err != nil {
		return err
	}
	state.result.BookingsEnded = int(capped + sourceEnded)
	return nil
}

func (s *careLifecycleService) finishCareExitConfirmation(ctx context.Context, state *careExitConfirmation) error {
	if err := s.recordCareEndAudit(ctx, state.before, state.studentIDs, &state.input.LastCareDay, state.actorAccountID); err != nil {
		return err
	}
	if state.completion == nil {
		return nil
	}
	resolved, err := s.withdrawalRepo.MarkResolved(ctx, state.completion.ID, state.actorAccountID, time.Now())
	if err != nil {
		return err
	}
	if !resolved {
		return userModels.ErrCareWithdrawalAlreadyResolved
	}
	return nil
}

// Cancel withdraws a planned exit. Only exits that have NOT taken effect can
// be cancelled — once the child is out, reopening the care is a decision with
// a new start day, not an undo (Resume).
func (s *careLifecycleService) Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error) {
	ids := dedupeSortedIDs(studentIDs)
	if len(ids) == 0 {
		return 0, ErrCareExitNoStudents
	}
	if len(ids) > MaxCareExitBatchSize {
		return 0, ErrCareExitTooManyStudents
	}

	cancelled := 0
	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if s.lockCareBookingWrites != nil {
			if err := s.lockCareBookingWrites(txCtx); err != nil {
				return fmt.Errorf("care lifecycle: lock care booking writes for cancellation: %w", err)
			}
		}
		today := timezone.TodayDate()
		locked, err := s.studentRepo.FindByIDsForUpdate(txCtx, ids)
		if err != nil {
			return err
		}
		exits, err := s.careExitRepo.FindByStudentIDs(txCtx, ids)
		if err != nil {
			return err
		}
		before := make(map[int64]*userModels.Student, len(ids))
		for _, id := range ids {
			student := locked[id]
			if student == nil {
				return ErrCareExitNotPlanned
			}
			if student.EnrolledUntil == nil {
				return ErrCareExitNotPlanned
			}
			if exits[id] == nil {
				return ErrCareExitNotPlanned
			}
			if student.CareEndedOn(today) {
				return ErrCareExitAlreadyEffective
			}
			before[id] = cloneCareFields(student)
		}
		// Cancelling gives the children their plan back. Without this the
		// cancellation would only clear the date and leave every child active
		// with the emptied timetable and the ended offerings of an exit that
		// was called off (#2487).
		if _, err := s.cleanupRepo.RestoreRemovals(txCtx, ids); err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.studentRepo.SetEnrolledUntilByIDs(txCtx, []int64{id}, exits[id].PreviousEnrolledUntil); err != nil {
				return err
			}
		}
		if err := s.careExitRepo.DeleteByStudentIDs(txCtx, ids); err != nil {
			return err
		}
		for _, id := range ids {
			if completionID := exits[id].WithdrawalCompletionID; s.withdrawalRepo != nil && completionID != nil {
				if _, err := s.withdrawalRepo.ReopenAfterCancelledExit(txCtx, *completionID, id, time.Now()); err != nil {
					return err
				}
			}
			if err := s.recordCareEndAudit(txCtx, before, []int64{id}, exits[id].PreviousEnrolledUntil, actorAccountID); err != nil {
				return err
			}
		}
		cancelled = len(ids)
		return nil
	})
	if err != nil {
		return 0, err
	}
	s.getLogger().Info("planned care end cancelled",
		slog.Int("students", cancelled),
		slog.Int64("actor_account_id", actorAccountID),
	)
	return cancelled, nil
}

// Resume reopens one child's care from a new start day. Master data survives;
// group, offerings, weekly plan and arrival/pickup times are NOT switched back
// on — the caller confirms having reviewed them, and sets them up again in the
// ordinary screens.
func (s *careLifecycleService) Resume(ctx context.Context, input CareResumeInput) error {
	if input.StudentID <= 0 {
		return ErrCareExitNoStudents
	}
	if !input.Checked {
		return ErrCareResumeNotChecked
	}
	today := timezone.TodayDate()
	if input.NewStart.Before(today) {
		return ErrCareResumeStartInPast
	}

	err := s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		locked, err := s.studentRepo.FindByIDsForUpdate(txCtx, []int64{input.StudentID})
		if err != nil {
			return err
		}
		student := locked[input.StudentID]
		if student == nil {
			return errors.New(careBlockerUnknown) //nolint:staticcheck // ST1005: user-facing German message
		}
		if student.IsAlumnus() {
			return errors.New(careBlockerAlumnus) //nolint:staticcheck // ST1005: user-facing German message
		}
		if !student.CareEndedOn(today) {
			return ErrCareResumeNotEnded
		}
		exits, err := s.careExitRepo.FindByStudentIDs(txCtx, []int64{input.StudentID})
		if err != nil {
			return err
		}
		if exits[input.StudentID] == nil {
			return ErrCareResumeMissing
		}
		before := cloneCareFields(student)

		// The lifecycle status is only today's view of the interval: a child
		// resumed for today is active right away, one resumed for next month
		// waits for the activate-students tick like any other future start.
		status := userModels.StudentStatusPending
		if !input.NewStart.After(today) {
			status = userModels.StudentStatusActive
		}
		if err := s.studentRepo.SetEnrollmentWindowByID(txCtx, input.StudentID, input.NewStart, status); err != nil {
			return err
		}
		if err := s.careExitRepo.DeleteByStudentIDs(txCtx, []int64{input.StudentID}); err != nil {
			return err
		}
		// Nothing is switched back on automatically: the criteria have the
		// school check group, offerings, weekly plan and times themselves, so
		// the ledger of the old exit is dropped unreplayed (#2487).
		if err := s.cleanupRepo.DiscardRemovals(txCtx, []int64{input.StudentID}); err != nil {
			return err
		}
		return s.recordCareEndAudit(txCtx,
			map[int64]*userModels.Student{input.StudentID: before},
			[]int64{input.StudentID}, nil, input.ActorAccountID)
	})
	if err != nil {
		return err
	}
	s.getLogger().Info("care resumed",
		slog.Int64("student_id", input.StudentID),
		slog.String("new_start", input.NewStart.String()),
		slog.Int64("actor_account_id", input.ActorAccountID),
	)
	return nil
}

// RecordedExitStudentIDs reduces the exit rows to their bare existence. The
// reason and its note stay in the service: they are readable with users:delete
// only, while the fact that an exit was recorded travels on the ordinary
// student payload so the list can label and the header can offer "Ende ändern"
// / "Ende stornieren" for exactly those children (#2487).
func (s *careLifecycleService) RecordedExitStudentIDs(
	ctx context.Context,
	studentIDs []int64,
) (map[int64]bool, error) {
	ids := dedupeSortedIDs(studentIDs)
	recorded := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return recorded, nil
	}
	exits, err := s.careExitRepo.FindByStudentIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for id, exit := range exits {
		if exit != nil {
			recorded[id] = true
		}
	}
	return recorded, nil
}

func (s *careLifecycleService) ListEnded(
	ctx context.Context,
	filter userModels.CareExitListFilter,
) ([]*userModels.EndedCare, int, error) {
	return s.careExitRepo.ListEnded(ctx, timezone.TodayDate(), filter)
}

func (s *careLifecycleService) ReconcileAuthoritativeBookingChange(
	ctx context.Context,
	change userModels.CareWithdrawalBookingChange,
) error {
	if s.withdrawalRepo == nil {
		return errors.New("care lifecycle: withdrawal repository is not configured")
	}
	today := timezone.TodayDate()
	if s.bookingsAuthoritative == nil {
		return errors.New("care lifecycle: bookings-authoritative resolver is not configured")
	}
	authoritative, err := s.bookingsAuthoritative(ctx)
	if err != nil {
		return err
	}
	if !authoritative {
		return s.reconcileWeeklyPlanRebooking(ctx, change)
	}
	facts, err := s.cleanupRepo.ListCareBookingFacts(ctx, today, []int64{change.StudentID})
	if err != nil {
		return err
	}
	if len(facts) != 1 {
		return fmt.Errorf("care lifecycle: booking facts for student %d not found", change.StudentID)
	}
	if change.WasCompleteWithdrawal {
		facts[0].ConfirmedBookinglessDay = &change.FirstBookinglessDay
	}
	evaluation := EvaluateCareBookingStates(facts, today)[0]
	pending, err := s.withdrawalRepo.ListPendingByStudentIDs(ctx, []int64{change.StudentID})
	if err != nil {
		return err
	}
	return s.reconcileBookingEvaluation(ctx, evaluation, change, today, pending[change.StudentID])
}

func (s *careLifecycleService) reconcileWeeklyPlanRebooking(
	ctx context.Context, change userModels.CareWithdrawalBookingChange,
) error {
	if change.WasCompleteWithdrawal {
		return nil
	}
	pending, err := s.withdrawalRepo.ListPendingByStudentIDs(ctx, []int64{change.StudentID})
	completion := pending[change.StudentID]
	if err != nil || completion == nil {
		return err
	}
	facts, err := s.cleanupRepo.ListCareBookingFacts(ctx, completion.FirstBookinglessDay, []int64{change.StudentID})
	if err != nil {
		return err
	}
	if len(facts) != 1 {
		return fmt.Errorf("care lifecycle: booking facts for student %d not found", change.StudentID)
	}
	evaluation := EvaluateCareBookingStates(facts, completion.FirstBookinglessDay)[0]
	if !evaluation.HasCareDays {
		return nil
	}
	_, err = s.withdrawalRepo.MarkObsoleteForRebooking(ctx, change.StudentID, completion.FirstBookinglessDay, time.Now())
	return err
}

func (s *careLifecycleService) reconcileBookingEvaluation(
	ctx context.Context,
	evaluation userModels.CareBookingEvaluation,
	change userModels.CareWithdrawalBookingChange,
	obsoleteFrom timezone.Date,
	pending *userModels.CareWithdrawalCompletion,
) error {
	if evaluation.FirstBookinglessDay == nil {
		if pending == nil || !evaluation.HasCareDays {
			return nil
		}
		_, err := s.withdrawalRepo.MarkObsoleteForRebooking(ctx, evaluation.StudentID, obsoleteFrom, time.Now())
		return err
	}
	if pending != nil {
		if !pending.FirstBookinglessDay.After(obsoleteFrom) {
			return nil
		}
		if *evaluation.FirstBookinglessDay != pending.FirstBookinglessDay {
			changed, obsoleteErr := s.withdrawalRepo.MarkObsoleteForRebooking(ctx, evaluation.StudentID, obsoleteFrom, time.Now())
			if obsoleteErr != nil {
				return obsoleteErr
			}
			if !changed {
				return errors.New("care lifecycle: pending booking completion changed during reconciliation")
			}
		} else if !change.WasCompleteWithdrawal {
			return nil
		}
	}
	return s.upsertBookingCompletion(ctx, evaluation, change)
}

func (s *careLifecycleService) upsertBookingCompletion(
	ctx context.Context,
	evaluation userModels.CareBookingEvaluation,
	change userModels.CareWithdrawalBookingChange,
) error {
	now := time.Now()
	studentID := change.StudentID
	var actorID *int64
	if change.ConfirmedBy > 0 {
		actorID = &change.ConfirmedBy
	}
	trigger := userModels.CareWithdrawalTriggerBookingExpired
	if change.WasCompleteWithdrawal {
		trigger = userModels.CareWithdrawalTriggerDirectSchool
	}
	role := strings.TrimSpace(change.ConfirmedRole)
	if role == "" {
		role = "system"
	}
	sourceChildID := evaluation.SourceRequestChildID
	if sourceChildID == 0 {
		sourceChildID = change.SourceRequestChildID
	}
	sourceOfferings := evaluation.SourceOfferings
	if len(sourceOfferings) == 0 {
		sourceOfferings = change.SourceOfferings
	}
	return s.withdrawalRepo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID:               &studentID,
		FirstBookinglessDay:     *evaluation.FirstBookinglessDay,
		Trigger:                 trigger,
		SourceAdjustmentID:      optionalPositiveID(change.SourceAdjustmentID),
		SourceRequestChildID:    optionalPositiveID(sourceChildID),
		WithdrawalConfirmedBy:   actorID,
		WithdrawalConfirmedRole: role,
		WithdrawalConfirmedAt:   now,
		SourceOfferings:         sourceOfferings,
	})
}

func (s *careLifecycleService) ListPendingWithdrawals(
	ctx context.Context,
	filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	if s.withdrawalRepo == nil {
		return nil, 0, errors.New("care lifecycle: withdrawal repository is not configured")
	}
	if filter.StudentID < 0 {
		return nil, 0, ErrCareWithdrawalNotFound
	}
	return s.withdrawalRepo.ListPending(ctx, filter.Normalized())
}

func (s *careLifecycleService) ListResolvedWithdrawals(
	ctx context.Context,
	filter userModels.CareWithdrawalCompletionFilter,
) ([]*userModels.CareWithdrawalCompletion, int, error) {
	if s.withdrawalRepo == nil {
		return nil, 0, errors.New("care lifecycle: withdrawal repository is not configured")
	}
	return s.withdrawalRepo.ListResolved(ctx, filter.Normalized())
}

func (s *careLifecycleService) GetPendingWithdrawal(ctx context.Context, id int64) (*userModels.CareWithdrawalCompletion, error) {
	if s.withdrawalRepo == nil || id <= 0 {
		return nil, ErrCareWithdrawalNotFound
	}
	completion, err := s.withdrawalRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if completion == nil {
		return nil, ErrCareWithdrawalNotFound
	}
	if completion.State != userModels.CareWithdrawalStatePending || completion.StudentID == nil {
		return nil, userModels.ErrCareWithdrawalAlreadyResolved
	}
	return completion, nil
}

func (s *careLifecycleService) PreviewWithdrawalCareEnd(
	ctx context.Context,
	completionID int64,
	input CareExitInput,
) (*CareExitPreview, error) {
	completion, err := s.GetPendingWithdrawal(ctx, completionID)
	if err != nil {
		return nil, err
	}
	input.StudentIDs = []int64{*completion.StudentID}
	normalized, err := normalizeCareExitInputMode(input, true)
	if err != nil {
		return nil, err
	}
	if err := s.validateWithdrawalCareEnd(ctx, completion, normalized); err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, normalized, false, map[int64][]userModels.CareExitSourceOffering{
		*completion.StudentID: completion.SourceOfferings,
	})
}

func (s *careLifecycleService) validateWithdrawalCareEnd(
	ctx context.Context,
	completion *userModels.CareWithdrawalCompletion,
	input CareExitInput,
) error {
	if completion == nil || completion.StudentID == nil {
		return ErrCareWithdrawalNotFound
	}
	if input.LastCareDay.After(completion.FirstBookinglessDay.AddDays(-1)) {
		return ErrCareWithdrawalAfterGap
	}
	students, err := s.studentRepo.FindByIDs(ctx, []int64{*completion.StudentID})
	if err != nil {
		return err
	}
	student := students[*completion.StudentID]
	if student == nil {
		return ErrCareWithdrawalNotFound
	}
	if student.EnrolledFrom != nil && input.LastCareDay.Before(*student.EnrolledFrom) {
		return &CareWithdrawalDateError{Message: fmt.Sprintf(careBlockerBeforeStart, student.EnrolledFrom.Format("02.01.2006"))}
	}
	latest, err := s.cleanupRepo.LatestAttendanceDate(ctx, *completion.StudentID)
	if err != nil {
		return err
	}
	if latest != nil && input.LastCareDay.Before(*latest) {
		return &CareWithdrawalDateError{Message: fmt.Sprintf("Eine Anwesenheit ist bis zum %s erfasst. Der letzte Betreuungstag darf nicht davor liegen.", latest.Format("02.01.2006"))}
	}
	return nil
}

// ApplyDueEffects is the effect-day half of the contract. It runs from the
// scheduler for every tenant and is idempotent: a second pass finds nothing
// open, no tag to release and no request to close.
func (s *careLifecycleService) ApplyDueEffects(ctx context.Context, asOf timezone.Date) (int, error) {
	if err := s.reconcileExpiredCareBookings(ctx, asOf); err != nil {
		return 0, err
	}
	// The candidate set is exactly the one the activate-students tick is about
	// to move to 'inactive': still 'active', interval already run out. That
	// makes the pass self-limiting — once the status has flipped, the same
	// children are not looked at again — and it must therefore run BEFORE the
	// status transition, which the scheduler guarantees.
	//
	// FindActiveDueForDeactivation compares enrolled_until <= its argument and
	// the interval's upper bound is INCLUSIVE, so the boundary handed in is the
	// day before asOf: a child is still in care on their last care day.
	due, err := s.studentRepo.FindActiveDueForDeactivation(ctx, asOf.AddDays(-1))
	if err != nil {
		return 0, err
	}
	if len(due) == 0 {
		return 0, nil
	}
	ids := make([]int64, 0, len(due))
	for _, student := range due {
		ids = append(ids, student.ID)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	applied := 0
	closedPresence := 0
	closedRequests := 0
	var releasedTags map[int64]string
	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if s.lockCareBookingWrites != nil {
			if err := s.lockCareBookingWrites(txCtx); err != nil {
				return fmt.Errorf("care lifecycle: lock care booking writes for due effects: %w", err)
			}
		}
		// The first query above is deliberately only a candidate lookup. An
		// operator can change or resume an exit before this transaction starts,
		// so lock and revalidate every row before any irreversible effect runs.
		locked, err := s.studentRepo.FindByIDsForUpdate(txCtx, ids)
		if err != nil {
			return err
		}
		current := ids[:0]
		for _, id := range ids {
			student := locked[id]
			if student != nil && student.Status == userModels.StudentStatusActive && student.CareEndedOn(asOf) {
				current = append(current, id)
			}
		}
		if len(current) == 0 {
			return nil
		}

		now := time.Now()
		closedPresence, err = s.cleanupRepo.CloseOpenPresence(txCtx, current, now)
		if err != nil {
			return err
		}
		closedRequests, err = s.cleanupRepo.CloseOpenRequests(txCtx, current, nil, now)
		if err != nil {
			return err
		}
		releasedTags, err = s.tagReleaser.ReleaseStudentTagsByIDs(txCtx, current)
		if err != nil {
			return err
		}
		if err := s.removePlansWrittenAfterExitConfirmation(txCtx, current, locked); err != nil {
			return err
		}
		// The exit is final now. What it removed from the plan stays removed, so
		// the ledger that would have put it back is dropped (#2487).
		if err := s.cleanupRepo.DiscardRemovals(txCtx, current); err != nil {
			return err
		}
		applied = len(current)
		return nil
	})
	if err != nil {
		return 0, err
	}

	if closedPresence > 0 || closedRequests > 0 || len(releasedTags) > 0 {
		s.getLogger().Info("care exit effects applied",
			slog.Int("students", applied),
			slog.Int("presence_records_closed", closedPresence),
			slog.Int("parent_requests_closed", closedRequests),
			slog.Int("tags_released", len(releasedTags)),
		)
	}
	return applied, nil
}

func (s *careLifecycleService) reconcileExpiredCareBookings(ctx context.Context, asOf timezone.Date) error {
	if s.bookingsAuthoritative == nil {
		return errors.New("care lifecycle: bookings-authoritative resolver is not configured")
	}
	return s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		if s.lockCareBookingWrites != nil {
			if err := s.lockCareBookingWrites(txCtx); err != nil {
				return fmt.Errorf("care lifecycle: lock care booking writes for booking expiry: %w", err)
			}
		}
		authoritative, err := s.bookingsAuthoritative(txCtx)
		if err != nil || !authoritative {
			return err
		}
		evaluations, err := s.evaluateCareBookings(txCtx, asOf)
		if err != nil {
			return err
		}
		return s.reconcileBookingEvaluations(txCtx, evaluations, asOf)
	})
}

func optionalPositiveID(value int64) *int64 {
	if value <= 0 {
		return nil
	}
	return &value
}

func (s *careLifecycleService) removePlansWrittenAfterExitConfirmation(
	ctx context.Context,
	studentIDs []int64,
	students map[int64]*userModels.Student,
) error {
	for _, studentID := range studentIDs {
		student := students[studentID]
		if student == nil || student.EnrolledUntil == nil {
			continue
		}
		ids := []int64{studentID}
		if _, err := s.cleanupRepo.DeletePlannedByStudentIDsAfter(ctx, ids, *student.EnrolledUntil); err != nil {
			return err
		}
		validUntil := student.EnrolledUntil.AddDays(1)
		if _, err := s.cleanupRepo.CapByStudentIDs(ctx, ids, validUntil); err != nil {
			return err
		}
		if _, err := s.cleanupRepo.EndSourceBookingsAndSchedules(ctx, ids, validUntil, nil); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------

// buildPreview resolves every child, collects the impacts and derives the
// token. With lock=true the student rows are read FOR UPDATE, which is what
// makes the confirmation's comparison meaningful.
func (s *careLifecycleService) buildPreview(
	ctx context.Context,
	input CareExitInput,
	lock bool,
	withdrawalOfferings map[int64][]userModels.CareExitSourceOffering,
) (*CareExitPreview, error) {
	ids := input.StudentIDs
	var (
		students map[int64]*userModels.Student
		err      error
	)
	if lock {
		students, err = s.studentRepo.FindByIDsForUpdate(ctx, ids)
	} else {
		students, err = s.studentRepo.FindByIDs(ctx, ids)
	}
	if err != nil {
		return nil, err
	}
	if lock {
		if err := s.cleanupRepo.LockOpenRequestsForCareExit(ctx, ids); err != nil {
			return nil, err
		}
		// Lock every live plan row before counting it for the bindende preview.
		// Confirmation then deletes or caps only a state that cannot change after
		// its token was derived.
		if err := s.cleanupRepo.LockPlanningForCareExit(ctx, ids, input.LastCareDay); err != nil {
			return nil, err
		}
		if err := s.cleanupRepo.LockImpactRowsForCareExit(ctx, ids); err != nil {
			return nil, err
		}
	}

	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := s.personRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, err
	}

	rosterCounts, err := s.cleanupRepo.CountPlannedByStudentIDsAfter(ctx, ids, input.LastCareDay)
	if err != nil {
		return nil, err
	}
	bookingCounts, err := s.cleanupRepo.CountRunningByStudentIDsAfter(ctx, ids, input.LastCareDay.AddDays(1))
	if err != nil {
		return nil, err
	}
	sourceOfferings, err := s.cleanupRepo.ListSourceOfferingsAfter(ctx, ids, input.LastCareDay.AddDays(1))
	if err != nil {
		return nil, err
	}
	weeklyPlans, err := s.cleanupRepo.ListWeeklyPlanPatterns(ctx, ids)
	if err != nil {
		return nil, err
	}
	requestCounts, err := s.cleanupRepo.CountOpenRequests(ctx, ids)
	if err != nil {
		return nil, err
	}
	presence, err := s.cleanupRepo.FindOpenPresence(ctx, ids)
	if err != nil {
		return nil, err
	}

	today := timezone.TodayDate()
	preview := &CareExitPreview{
		LastCareDay: input.LastCareDay,
		Reason:      input.Reason,
		ReasonNote:  input.ReasonNote,
		Students:    make([]CareExitImpact, 0, len(ids)),
	}

	for _, id := range ids {
		impact := CareExitImpact{StudentID: id}
		student := students[id]
		switch {
		case student == nil:
			impact.Blocker = careBlockerUnknown
		case student.IsAlumnus():
			impact.Blocker = careBlockerAlumnus
		case student.CareEndedOn(today):
			impact.Blocker = careBlockerAlreadyEnded
		case student.EnrolledFrom != nil && input.LastCareDay.Before(*student.EnrolledFrom):
			impact.Blocker = fmt.Sprintf(careBlockerBeforeStart, student.EnrolledFrom.Format("02.01.2006"))
		}
		if student != nil {
			impact.SchoolClass = student.SchoolClass
			impact.PlannedEndsOn = student.EnrolledUntil
			if person := persons[student.PersonID]; person != nil {
				impact.FirstName = person.FirstName
				impact.LastName = person.LastName
				impact.HasRFIDTag = person.TagID != nil && *person.TagID != ""
			}
			impact.PlannedRosterRows = rosterCounts[id]
			impact.ActivityBookings = bookingCounts[id]
			impact.SourceOfferings = mergeCareExitSourceOfferings(sourceOfferings[id], withdrawalOfferings[id])
			impact.WeeklyPlans = weeklyPlans[id]
			impact.OpenParentRequests = requestCounts[id]
			impact.CurrentlyPresent = presence[id]
		}
		if impact.Blocker != "" {
			preview.Blocked = true
		}
		preview.Students = append(preview.Students, impact)
	}

	preview.Token = careExitToken(input, students, preview.Students)
	return preview, nil
}

func mergeCareExitSourceOfferings(
	live []userModels.CareExitSourceOffering,
	withdrawn []userModels.CareExitSourceOffering,
) []userModels.CareExitSourceOffering {
	merged := make([]userModels.CareExitSourceOffering, 0, len(live)+len(withdrawn))
	seen := make(map[string]bool, len(live)+len(withdrawn))
	appendUnique := func(rows []userModels.CareExitSourceOffering) {
		for _, row := range rows {
			key := row.Name + "\x00" + strings.Join(row.Days, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, row)
		}
	}
	appendUnique(withdrawn)
	appendUnique(live)
	return merged
}

// recordCareEndAudit writes one change-history row per child whose last care
// day actually moved. The reason never goes in here: the change history is
// readable with users:read, the reason only with users:delete.
func (s *careLifecycleService) recordCareEndAudit(
	ctx context.Context,
	before map[int64]*userModels.Student,
	ids []int64,
	newLastCareDay *timezone.Date,
	actorAccountID int64,
) error {
	if s.auditService == nil {
		return nil
	}
	for _, id := range ids {
		previous := before[id]
		if previous == nil {
			previous = &userModels.Student{}
		}
		after := &userModels.Student{EnrolledUntil: newLastCareDay}
		after.ID = id
		if err := s.auditService.RecordChangesForActor(ctx, previous, after, actorAccountID); err != nil {
			return err
		}
	}
	return nil
}

// cloneCareFields keeps only what the care-end audit diffs, so a "before"
// snapshot cannot accidentally carry an unrelated field into the history.
func cloneCareFields(student *userModels.Student) *userModels.Student {
	clone := &userModels.Student{EnrolledUntil: student.EnrolledUntil}
	clone.ID = student.ID
	return clone
}

// normalizeCareExitInput validates the whole-action fields and returns the
// canonical form every later step works from.
func normalizeCareExitInput(input CareExitInput) (CareExitInput, error) {
	return normalizeCareExitInputMode(input, false)
}

func normalizeCareExitInputMode(input CareExitInput, allowPast bool) (CareExitInput, error) {
	ids := dedupeSortedIDs(input.StudentIDs)
	if len(ids) == 0 {
		return input, ErrCareExitNoStudents
	}
	if len(ids) > MaxCareExitBatchSize {
		return input, ErrCareExitTooManyStudents
	}
	if !allowPast && input.LastCareDay.Before(timezone.TodayDate()) {
		return input, ErrCareExitDayInPast
	}

	probe := &userModels.CareExit{StudentID: ids[0], Reason: input.Reason}
	note := strings.TrimSpace(input.ReasonNote)
	if note != "" {
		probe.ReasonNote = &note
	}
	if err := probe.Validate(); err != nil {
		return input, err
	}

	normalized := CareExitInput{
		StudentIDs:  ids,
		LastCareDay: input.LastCareDay,
		Reason:      probe.Reason,
	}
	if probe.ReasonNote != nil {
		normalized.ReasonNote = *probe.ReasonNote
	}
	return normalized, nil
}

// careExitToken fingerprints exactly what the person confirming has seen: the
// selection, the day, the reason, every child's row version, and every impact
// number the preview showed. Anything moving underneath changes the token, and
// the confirmation refuses instead of doing something else than promised.
func careExitToken(
	input CareExitInput,
	students map[int64]*userModels.Student,
	impacts []CareExitImpact,
) string {
	type childState struct {
		ID        int64                               `json:"id"`
		UpdatedAt int64                               `json:"updated_at"`
		Roster    int                                 `json:"roster"`
		Bookings  int                                 `json:"bookings"`
		Offerings []userModels.CareExitSourceOffering `json:"offerings"`
		Requests  int                                 `json:"requests"`
		Present   bool                                `json:"present"`
		Tag       bool                                `json:"tag"`
		Blocker   string                              `json:"blocker"`
	}
	payload := struct {
		LastCareDay string       `json:"last_care_day"`
		Reason      string       `json:"reason"`
		ReasonNote  string       `json:"reason_note"`
		Children    []childState `json:"children"`
	}{
		LastCareDay: input.LastCareDay.String(),
		Reason:      input.Reason,
		ReasonNote:  input.ReasonNote,
		Children:    make([]childState, 0, len(impacts)),
	}
	for _, impact := range impacts {
		state := childState{
			ID:        impact.StudentID,
			Roster:    impact.PlannedRosterRows,
			Bookings:  impact.ActivityBookings,
			Offerings: impact.SourceOfferings,
			Requests:  impact.OpenParentRequests,
			Present:   impact.CurrentlyPresent,
			Tag:       impact.HasRFIDTag,
			Blocker:   impact.Blocker,
		}
		if student := students[impact.StudentID]; student != nil {
			state.UpdatedAt = student.UpdatedAt.UnixNano()
		}
		payload.Children = append(payload.Children, state)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		// Marshalling a struct of scalars cannot fail; a token nothing can
		// match is still the safe answer if it ever did.
		return "unmatchable"
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}

func equalCareToken(actual, expected string) bool {
	actualBytes, actualErr := hex.DecodeString(actual)
	expectedBytes, expectedErr := hex.DecodeString(strings.TrimSpace(expected))
	if actualErr != nil || expectedErr != nil || len(actualBytes) != len(expectedBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(actualBytes, expectedBytes) == 1
}

// dedupeSortedIDs folds duplicates and sorts ascending — the project-wide lock
// order, and the order the token depends on.
func dedupeSortedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// careExitAuditField is referenced so the audit constant and this service move
// together if the field is ever renamed.
var _ = auditModels.StudentFieldCareEnd
