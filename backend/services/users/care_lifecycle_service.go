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
	ErrCareResumeStartInPast = errors.New("Der neue Beginn darf nicht in der Vergangenheit liegen.")
	//nolint:staticcheck // ST1005: user-facing German message
	ErrCareResumeNotChecked = errors.New("Bitte bestätigen Sie zuerst die Prüfung. Gruppe, Angebote, Wochenplan und Zeiten bleiben sonst ungeprüft.")
)

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
}

type careLifecycleService struct {
	studentRepo  userModels.StudentRepository
	personRepo   userModels.PersonRepository
	careExitRepo userModels.CareExitRepository
	cleanupRepo  userModels.CareExitCleanupRepository
	tagReleaser  CareExitTagReleaser
	auditService StudentAuditService
	txHandler    *modelBase.TxHandler
	logger       *slog.Logger
}

// CareLifecycleDependencies wires the service. Every field is required except
// the logger; a nil collaborator would silently skip a documented effect.
type CareLifecycleDependencies struct {
	StudentRepo  userModels.StudentRepository
	PersonRepo   userModels.PersonRepository
	CareExitRepo userModels.CareExitRepository
	CleanupRepo  userModels.CareExitCleanupRepository
	TagReleaser  CareExitTagReleaser
	AuditService StudentAuditService
	DB           *bun.DB
	Logger       *slog.Logger
}

// NewCareLifecycleService builds the service.
func NewCareLifecycleService(deps CareLifecycleDependencies) CareLifecycleService {
	return &careLifecycleService{
		studentRepo:  deps.StudentRepo,
		personRepo:   deps.PersonRepo,
		careExitRepo: deps.CareExitRepo,
		cleanupRepo:  deps.CleanupRepo,
		tagReleaser:  deps.TagReleaser,
		auditService: deps.AuditService,
		txHandler:    modelBase.NewTxHandler(deps.DB),
		logger:       deps.Logger,
	}
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
	return s.buildPreview(ctx, normalized, false)
}

func (s *careLifecycleService) Confirm(
	ctx context.Context,
	token string,
	input CareExitInput,
	actorAccountID int64,
) (*CareExitResult, error) {
	normalized, err := normalizeCareExitInput(input)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, ErrCareExitPreviewChanged
	}

	result := new(CareExitResult)
	err = s.txHandler.RunInTx(ctx, func(txCtx context.Context, _ bun.Tx) error {
		// Locked re-read: the preview is only authoritative while nobody else
		// can move these rows. Everything below reads the locked state.
		preview, err := s.buildPreview(txCtx, normalized, true)
		if err != nil {
			return err
		}
		if !equalCareToken(preview.Token, token) {
			return ErrCareExitPreviewChanged
		}
		if preview.Blocked {
			return ErrCareExitBlocked
		}

		ids := make([]int64, 0, len(preview.Students))
		for _, impact := range preview.Students {
			ids = append(ids, impact.StudentID)
		}

		before, err := s.studentRepo.FindByIDs(txCtx, ids)
		if err != nil {
			return err
		}

		// A second run over the same child is a CHANGE, and a change applies to
		// the untouched plan, not to the remains of the previous attempt: an
		// exit moved from June to July must leave July's rosters and bookings
		// in place. So the previous exit is undone first and the new last care
		// day is applied to the baseline the preview counted (#2487).
		if _, err := s.cleanupRepo.RestoreRemovals(txCtx, ids); err != nil {
			return err
		}

		if _, err := s.studentRepo.SetEnrolledUntilByIDs(txCtx, ids, &normalized.LastCareDay); err != nil {
			return err
		}

		for _, id := range ids {
			exit := &userModels.CareExit{
				StudentID:  id,
				Reason:     normalized.Reason,
				RecordedBy: &actorAccountID,
			}
			if normalized.ReasonNote != "" {
				note := normalized.ReasonNote
				exit.ReasonNote = &note
			}
			if err := s.careExitRepo.Upsert(txCtx, exit); err != nil {
				return err
			}
		}

		// Rosters and bookings are reconciled NOW, not on the effect day: the
		// planning screens have to stop showing the child on days they will
		// not attend as soon as the school has decided it, otherwise staff
		// keep planning around a child who is leaving.
		removed, err := s.cleanupRepo.DeletePlannedByStudentIDsAfter(txCtx, ids, normalized.LastCareDay)
		if err != nil {
			return err
		}
		result.RosterRowsRemoved = removed

		// valid_until is an EXCLUSIVE upper bound, so a booking that must
		// still count on the last care day ends the day after it.
		capped, err := s.cleanupRepo.CapByStudentIDs(txCtx, ids, normalized.LastCareDay.AddDays(1))
		if err != nil {
			return err
		}
		result.BookingsEnded = int(capped)

		if err := s.recordCareEndAudit(txCtx, before, ids, &normalized.LastCareDay, actorAccountID); err != nil {
			return err
		}

		result.StudentsEnded = len(ids)
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.getLogger().Info("care ended",
		slog.Int("students", result.StudentsEnded),
		slog.String("last_care_day", normalized.LastCareDay.String()),
		slog.String("reason", normalized.Reason),
		slog.Int64("actor_account_id", actorAccountID),
	)
	return result, nil
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
		if _, err := s.studentRepo.SetEnrolledUntilByIDs(txCtx, ids, nil); err != nil {
			return err
		}
		if err := s.careExitRepo.DeleteByStudentIDs(txCtx, ids); err != nil {
			return err
		}
		if err := s.recordCareEndAudit(txCtx, before, ids, nil, actorAccountID); err != nil {
			return err
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

// ApplyDueEffects is the effect-day half of the contract. It runs from the
// scheduler for every tenant and is idempotent: a second pass finds nothing
// open, no tag to release and no request to close.
func (s *careLifecycleService) ApplyDueEffects(ctx context.Context, asOf timezone.Date) (int, error) {
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

	now := time.Now()
	closedPresence, err := s.cleanupRepo.CloseOpenPresence(ctx, ids, now)
	if err != nil {
		return 0, err
	}
	closedRequests, err := s.cleanupRepo.CloseOpenRequests(ctx, ids, nil, now)
	if err != nil {
		return 0, err
	}
	releasedTags, err := s.tagReleaser.ReleaseStudentTagsByIDs(ctx, ids)
	if err != nil {
		return 0, err
	}
	// The exit is final now. What it removed from the plan stays removed, so
	// the ledger that would have put it back is dropped (#2487).
	if err := s.cleanupRepo.DiscardRemovals(ctx, ids); err != nil {
		return 0, err
	}

	if closedPresence > 0 || closedRequests > 0 || len(releasedTags) > 0 {
		s.getLogger().Info("care exit effects applied",
			slog.Int("students", len(ids)),
			slog.Int("presence_records_closed", closedPresence),
			slog.Int("parent_requests_closed", closedRequests),
			slog.Int("tags_released", len(releasedTags)),
		)
	}
	return len(ids), nil
}

// ---------------------------------------------------------------------------

// buildPreview resolves every child, collects the impacts and derives the
// token. With lock=true the student rows are read FOR UPDATE, which is what
// makes the confirmation's comparison meaningful.
func (s *careLifecycleService) buildPreview(
	ctx context.Context,
	input CareExitInput,
	lock bool,
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
	ids := dedupeSortedIDs(input.StudentIDs)
	if len(ids) == 0 {
		return input, ErrCareExitNoStudents
	}
	if len(ids) > MaxCareExitBatchSize {
		return input, ErrCareExitTooManyStudents
	}
	if input.LastCareDay.Before(timezone.TodayDate()) {
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
		ID        int64  `json:"id"`
		UpdatedAt int64  `json:"updated_at"`
		Roster    int    `json:"roster"`
		Bookings  int    `json:"bookings"`
		Requests  int    `json:"requests"`
		Present   bool   `json:"present"`
		Tag       bool   `json:"tag"`
		Blocker   string `json:"blocker"`
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
			ID:       impact.StudentID,
			Roster:   impact.PlannedRosterRows,
			Bookings: impact.ActivityBookings,
			Requests: impact.OpenParentRequests,
			Present:  impact.CurrentlyPresent,
			Tag:      impact.HasRFIDTag,
			Blocker:  impact.Blocker,
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
