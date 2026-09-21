package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// CareLifecycleOwners are the owners the care lifecycle drives through their
// consumer-owned ports. Every one is required: a missing collaborator would
// silently skip a documented effect.
type CareLifecycleOwners struct {
	Students    ports.CareStudentDirectory
	Roster      ports.CareExitRoster
	Bookings    ports.CareExitBookings
	Enrollment  ports.CareExitEnrollment
	Presence    ports.CareExitPresence
	Calendar    ports.CareCalendarPeriods
	Tags        ports.CareExitTagReleaser
	ChangeTrail ports.CareEndRecorder
}

// CareLifecycleDependencies wires the lifecycle. Records is Care Plan's own
// record capability; Directory is the named student directory projection.
type CareLifecycleDependencies struct {
	Owners    CareLifecycleOwners
	Records   careplan.Capability
	Directory ports.CareExitDirectory
	Unit      ports.UnitOfWork
	// LockCareBookingWrites is the transaction-scoped gate authoritative
	// offering adjustments take. Taking it before any plan lock makes
	// rebooking and care-end confirmation a total order instead of letting a
	// stale exit commit beside newly booked care.
	LockCareBookingWrites func(context.Context) error
	// BookingsAuthoritative resolves the school's booking-led care setting.
	BookingsAuthoritative func(context.Context) (bool, error)
	// Fingerprint hashes the binding preview content into its token.
	Fingerprint func([]byte) string
	Logger      *slog.Logger
	Today       func() calendar.Date
}

// CareLifecycle is the native care-lifecycle application (#3427).
type CareLifecycle struct {
	owners                CareLifecycleOwners
	records               careplan.Capability
	directory             ports.CareExitDirectory
	unit                  ports.UnitOfWork
	lockCareBookingWrites func(context.Context) error
	bookingsAuthoritative func(context.Context) (bool, error)
	fingerprint           func([]byte) string
	logger                *slog.Logger
	today                 func() calendar.Date
}

var _ careplan.CareLifecycle = (*CareLifecycle)(nil)

// NewCareLifecycle validates the wiring and builds the lifecycle.
func NewCareLifecycle(deps CareLifecycleDependencies) (*CareLifecycle, error) {
	owners := deps.Owners
	if owners.Students == nil || owners.Roster == nil || owners.Bookings == nil || owners.Enrollment == nil ||
		owners.Presence == nil || owners.Calendar == nil || owners.Tags == nil || owners.ChangeTrail == nil {
		return nil, errors.New("care lifecycle: every owner port is required")
	}
	if deps.Records == nil || deps.Directory == nil || deps.Unit == nil ||
		deps.LockCareBookingWrites == nil || deps.BookingsAuthoritative == nil || deps.Fingerprint == nil || deps.Today == nil {
		return nil, errors.New("care lifecycle: records, directory, unit of work, booking lock, booking setting, fingerprint and clock are required")
	}
	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &CareLifecycle{
		owners: owners, records: deps.Records, directory: deps.Directory, unit: deps.Unit,
		lockCareBookingWrites: deps.LockCareBookingWrites, bookingsAuthoritative: deps.BookingsAuthoritative,
		fingerprint: deps.Fingerprint, logger: logger, today: deps.Today,
	}, nil
}

func (s *CareLifecycle) Preview(ctx context.Context, input careplan.CareExitInput) (*careplan.CareExitPreview, error) {
	normalized, err := domain.NormalizeCareExitInput(input, false, s.today())
	if err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, normalized, false, nil)
}

func (s *CareLifecycle) Confirm(ctx context.Context, token string, input careplan.CareExitInput, actorAccountID int64) (*careplan.CareExitResult, error) {
	return s.confirm(ctx, nil, token, input, actorAccountID, false)
}

// careExitConfirmation is the state one confirmation carries through its
// transaction.
type careExitConfirmation struct {
	completion     *careplan.WithdrawalCompletion
	token          string
	input          careplan.CareExitInput
	actorAccountID int64
	studentIDs     []int64
	before         map[int64]*calendar.Date
	exits          map[int64]careplan.CareExit
	result         careplan.CareExitResult
}

func (s *CareLifecycle) confirm(
	ctx context.Context, completion *careplan.WithdrawalCompletion, token string,
	input careplan.CareExitInput, actorAccountID int64, allowPast bool,
) (*careplan.CareExitResult, error) {
	normalized, err := domain.NormalizeCareExitInput(input, allowPast, s.today())
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(token) == "" {
		return nil, careplan.ErrCareExitPreviewChanged
	}
	state := &careExitConfirmation{completion: completion, token: token, input: normalized, actorAccountID: actorAccountID}
	if err := s.unit(ctx, func(txCtx context.Context) error {
		return s.applyCareExitConfirmation(txCtx, state)
	}); err != nil {
		return nil, err
	}
	s.logger.Info("care ended",
		slog.Int("students", state.result.StudentsEnded),
		slog.String("last_care_day", state.input.LastCareDay.String()),
		slog.String("reason", state.input.Reason),
		slog.Int64("actor_account_id", state.actorAccountID),
	)
	return &state.result, nil
}

func (s *CareLifecycle) applyCareExitConfirmation(ctx context.Context, state *careExitConfirmation) error {
	if err := s.prepareLockedCareExitPreview(ctx, state); err != nil {
		return err
	}
	if err := s.loadCareExitBaseline(ctx, state); err != nil {
		return err
	}
	if _, err := s.restoreRemovals(ctx, state.studentIDs); err != nil {
		return err
	}
	lastCareDay := state.input.LastCareDay
	if err := s.owners.Students.EndCare(ctx, state.studentIDs, &lastCareDay); err != nil {
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

func (s *CareLifecycle) prepareLockedCareExitPreview(ctx context.Context, state *careExitConfirmation) error {
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return fmt.Errorf("care lifecycle: lock care booking writes: %w", err)
	}
	preview, err := s.buildPreview(ctx, state.input, true, withdrawalOfferings(state.completion))
	if err != nil {
		return err
	}
	if state.completion != nil {
		preview, err = s.refreshLockedWithdrawalPreview(ctx, state)
		if err != nil {
			return err
		}
	}
	if !domain.EqualCareToken(preview.Token, state.token) {
		return careplan.ErrCareExitPreviewChanged
	}
	if preview.Blocked {
		return careplan.ErrCareExitBlocked
	}
	state.studentIDs = make([]int64, 0, len(preview.Students))
	for _, impact := range preview.Students {
		state.studentIDs = append(state.studentIDs, impact.StudentID)
	}
	return nil
}

func (s *CareLifecycle) loadCareExitBaseline(ctx context.Context, state *careExitConfirmation) error {
	students, err := s.owners.Students.FindCareStudents(ctx, state.studentIDs, false)
	if err != nil {
		return err
	}
	state.before = careEndsOf(students)
	exits, err := s.records.FindCareExits(ctx, state.studentIDs)
	if err != nil {
		return err
	}
	state.exits = exits
	return nil
}

// careEndsOf keeps only what the care-end history diffs, so a "before"
// snapshot cannot carry an unrelated field into the history.
func careEndsOf(students map[int64]domain.CareStudent) map[int64]*calendar.Date {
	ends := make(map[int64]*calendar.Date, len(students))
	for id, student := range students {
		ends[id] = student.EnrolledUntil
	}
	return ends
}

func (s *CareLifecycle) upsertCareExitRecords(ctx context.Context, state *careExitConfirmation) error {
	for _, id := range state.studentIDs {
		exit := careplan.CareExit{StudentID: id, Reason: state.input.Reason, RecordedBy: &state.actorAccountID}
		if state.completion != nil {
			completionID := state.completion.ID
			exit.WithdrawalCompletionID = &completionID
		}
		if _, recorded := state.exits[id]; !recorded {
			exit.PreviousEnrolledUntil = carePlanDate(state.before[id])
		}
		if state.input.ReasonNote != "" {
			note := state.input.ReasonNote
			exit.ReasonNote = &note
		}
		if err := s.records.UpsertCareExit(ctx, exit); err != nil {
			return fmt.Errorf("care lifecycle: upsert care exit: %w", err)
		}
	}
	return nil
}

func (s *CareLifecycle) cleanupConfirmedCareExit(ctx context.Context, state *careExitConfirmation) error {
	removed, err := s.removePlannedRosterAfter(ctx, state.studentIDs, state.input.LastCareDay)
	if err != nil {
		return err
	}
	state.result.RosterRowsRemoved = removed
	validUntil := state.input.LastCareDay.AddDays(1)
	capped, err := s.capBookings(ctx, state.studentIDs, validUntil)
	if err != nil {
		return err
	}
	// Recurring arrival and pickup plans have no date range. Keep them
	// through a future last care day, but end them immediately once that day
	// has passed. A completed care exit ends every remaining source booking,
	// including non-care offers and bookings another enrollment request
	// created; the completion's source child is audit context, not a cleanup
	// boundary.
	endSchedules := state.input.LastCareDay.Before(s.today())
	sourceEnded, err := s.endSourceBookings(ctx, state.studentIDs, validUntil, endSchedules)
	if err != nil {
		return err
	}
	state.result.BookingsEnded = int(capped + sourceEnded)
	return nil
}

func (s *CareLifecycle) finishCareExitConfirmation(ctx context.Context, state *careExitConfirmation) error {
	lastCareDay := state.input.LastCareDay
	if err := s.recordCareEnds(ctx, state.before, state.studentIDs, &lastCareDay, state.actorAccountID); err != nil {
		return err
	}
	if state.completion == nil {
		return nil
	}
	resolved, err := s.records.ResolveWithdrawal(ctx, state.completion.ID, state.actorAccountID, time.Now())
	if err != nil {
		return fmt.Errorf("care lifecycle: resolve care withdrawal completion: %w", err)
	}
	if !resolved {
		return careplan.ErrCareWithdrawalAlreadyResolved
	}
	return nil
}

// Cancel withdraws a planned exit. Only exits that have NOT taken effect can
// be cancelled: once the child is out, reopening the care is a decision with
// a new start day, not an undo (Resume).
func (s *CareLifecycle) Cancel(ctx context.Context, studentIDs []int64, actorAccountID int64) (int, error) {
	ids := domain.DedupeSortedIDs(studentIDs)
	if len(ids) == 0 {
		return 0, careplan.ErrCareExitNoStudents
	}
	if len(ids) > careplan.MaxCareExitBatchSize {
		return 0, careplan.ErrCareExitTooManyStudents
	}
	if err := s.unit(ctx, func(txCtx context.Context) error {
		return s.cancelCareExits(txCtx, ids, actorAccountID)
	}); err != nil {
		return 0, err
	}
	s.logger.Info("planned care end cancelled",
		slog.Int("students", len(ids)),
		slog.Int64("actor_account_id", actorAccountID),
	)
	return len(ids), nil
}

func (s *CareLifecycle) cancelCareExits(ctx context.Context, ids []int64, actorAccountID int64) error {
	if err := s.lockCareBookingWrites(ctx); err != nil {
		return fmt.Errorf("care lifecycle: lock care booking writes for cancellation: %w", err)
	}
	exits, before, err := s.plannedCareExits(ctx, ids)
	if err != nil {
		return err
	}
	// Cancellation restores the removed plan as well as the care end (#2487).
	if _, err := s.restoreRemovals(ctx, ids); err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.owners.Students.EndCare(ctx, []int64{id}, calendarDate(exits[id].PreviousEnrolledUntil)); err != nil {
			return err
		}
	}
	if err := s.records.DeleteCareExits(ctx, ids); err != nil {
		return fmt.Errorf("care lifecycle: delete care exits: %w", err)
	}
	for _, id := range ids {
		if err := s.reopenCancelledExit(ctx, id, exits[id], before, actorAccountID); err != nil {
			return err
		}
	}
	return nil
}

// plannedCareExits reads the locked children and their recorded exits and
// refuses unless every one has an exit that has not taken effect yet. It
// returns the recorded exits and each child's current last care day.
func (s *CareLifecycle) plannedCareExits(ctx context.Context, ids []int64) (map[int64]careplan.CareExit, map[int64]*calendar.Date, error) {
	today := s.today()
	locked, err := s.owners.Students.FindCareStudents(ctx, ids, true)
	if err != nil {
		return nil, nil, err
	}
	exits, err := s.records.FindCareExits(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("care lifecycle: find care exits: %w", err)
	}
	before := make(map[int64]*calendar.Date, len(ids))
	for _, id := range ids {
		student, found := locked[id]
		if _, recorded := exits[id]; !found || student.EnrolledUntil == nil || !recorded {
			return nil, nil, careplan.ErrCareExitNotPlanned
		}
		if student.CareEndedOn(today) {
			return nil, nil, careplan.ErrCareExitAlreadyEffective
		}
		before[id] = student.EnrolledUntil
	}
	return exits, before, nil
}

// reopenCancelledExit puts back the withdrawal task the exit had closed and
// records the restored care end in the child's history.
func (s *CareLifecycle) reopenCancelledExit(ctx context.Context, id int64, exit careplan.CareExit, before map[int64]*calendar.Date, actorAccountID int64) error {
	if completionID := exit.WithdrawalCompletionID; completionID != nil {
		if _, err := s.records.ReopenWithdrawalAfterCancelledExit(ctx, *completionID, id, time.Now()); err != nil {
			return fmt.Errorf("care lifecycle: reopen care withdrawal after cancelled exit: %w", err)
		}
	}
	return s.recordCareEnds(ctx, before, []int64{id}, calendarDate(exit.PreviousEnrolledUntil), actorAccountID)
}

// Resume reopens one child's care from a new start day. Master data
// survives; group, offerings, weekly plan and arrival/pickup times are NOT
// switched back on: the caller confirms having reviewed them and sets them
// up again in the ordinary screens.
func (s *CareLifecycle) Resume(ctx context.Context, input careplan.CareResumeInput) error {
	if input.StudentID <= 0 {
		return careplan.ErrCareExitNoStudents
	}
	if !input.Checked {
		return careplan.ErrCareResumeNotChecked
	}
	today := s.today()
	if input.NewStart.Before(today) {
		return careplan.ErrCareResumeStartInPast
	}
	if err := s.unit(ctx, func(txCtx context.Context) error {
		return s.resumeCare(txCtx, input, today)
	}); err != nil {
		return err
	}
	s.logger.Info("care resumed",
		slog.Int64("student_id", input.StudentID),
		slog.String("new_start", input.NewStart.String()),
		slog.Int64("actor_account_id", input.ActorAccountID),
	)
	return nil
}

func (s *CareLifecycle) resumeCare(ctx context.Context, input careplan.CareResumeInput, today calendar.Date) error {
	locked, err := s.owners.Students.FindCareStudents(ctx, []int64{input.StudentID}, true)
	if err != nil {
		return err
	}
	student, found := locked[input.StudentID]
	if !found {
		return errors.New(careplan.CareBlockerUnknown) //nolint:staticcheck // ST1005: user-facing German message
	}
	if student.IsAlumnus() {
		return errors.New(careplan.CareBlockerAlumnus) //nolint:staticcheck // ST1005: user-facing German message
	}
	if !student.CareEndedOn(today) {
		return careplan.ErrCareResumeNotEnded
	}
	exits, err := s.records.FindCareExits(ctx, []int64{input.StudentID})
	if err != nil {
		return fmt.Errorf("care lifecycle: find care exits: %w", err)
	}
	if _, recorded := exits[input.StudentID]; !recorded {
		return careplan.ErrCareResumeMissing
	}
	// Future starts wait for the activation tick; today's starts are active.
	status := careplan.StudentStatusPending
	if !input.NewStart.After(today) {
		status = careplan.StudentStatusActive
	}
	if err := s.owners.Students.ResumeCare(ctx, input.StudentID, input.NewStart, status, today); err != nil {
		return err
	}
	if err := s.records.DeleteCareExits(ctx, []int64{input.StudentID}); err != nil {
		return fmt.Errorf("care lifecycle: delete care exits: %w", err)
	}
	// Resume never restores planning automatically; discard the old ledger (#2487).
	if err := s.discardRemovals(ctx, []int64{input.StudentID}); err != nil {
		return err
	}
	return s.recordCareEnds(ctx, map[int64]*calendar.Date{input.StudentID: student.EnrolledUntil},
		[]int64{input.StudentID}, nil, input.ActorAccountID)
}

// RecordedExitStudentIDs reduces the exit rows to their bare existence. The
// reason and its note stay here: they are readable with users:delete only,
// while the fact that an exit was recorded travels on the ordinary student
// payload so the list can label exactly those children (#2487).
func (s *CareLifecycle) RecordedExitStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	ids := domain.DedupeSortedIDs(studentIDs)
	recorded := make(map[int64]bool, len(ids))
	if len(ids) == 0 {
		return recorded, nil
	}
	exits, err := s.records.FindCareExits(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("care lifecycle: find care exits: %w", err)
	}
	for id := range exits {
		recorded[id] = true
	}
	return recorded, nil
}

// ListEnded is the archive. It reads the CHILDREN whose enrolment interval
// has run out rather than the reason rows: the archive holds every regularly
// ended care, including one that ended with an enrolment phase and never got
// a manually recorded reason.
func (s *CareLifecycle) ListEnded(ctx context.Context, filter careplan.EndedCareFilter) ([]careplan.EndedCare, int, error) {
	students, total, err := s.directory.ListEndedCare(ctx, careplan.Date(s.today()), filter)
	if err != nil {
		return nil, 0, fmt.Errorf("care lifecycle: list ended care: %w", err)
	}
	rows := make([]careplan.EndedCare, 0, len(students))
	studentIDs := make([]int64, 0, len(students))
	for _, student := range students {
		rows = append(rows, careplan.EndedCare{
			StudentID: student.StudentID, FirstName: student.FirstName, LastName: student.LastName,
			SchoolClass: student.SchoolClass, LastCareDay: student.LastCareDay,
		})
		studentIDs = append(studentIDs, student.StudentID)
	}
	exits, err := s.records.FindCareExits(ctx, studentIDs)
	if err != nil {
		return nil, 0, fmt.Errorf("care lifecycle: find care exits: %w", err)
	}
	for index := range rows {
		exit, recorded := exits[rows[index].StudentID]
		if !recorded {
			continue
		}
		reason := exit.Reason
		recordedAt := calendar.DateFromTime(exit.CreatedAt)
		rows[index].Reason, rows[index].ReasonNote, rows[index].RecordedBy = &reason, exit.ReasonNote, exit.RecordedBy
		rows[index].RecordedAt = &recordedAt
	}
	return rows, total, nil
}

// recordCareEnds writes one change-history row per child whose last care day
// actually moved; the recorder skips a child whose end did not change.
func (s *CareLifecycle) recordCareEnds(ctx context.Context, before map[int64]*calendar.Date, ids []int64, after *calendar.Date, actorAccountID int64) error {
	for _, id := range ids {
		if err := s.owners.ChangeTrail.RecordCareEndChange(ctx, id, before[id], after, actorAccountID); err != nil {
			return err
		}
	}
	return nil
}

func withdrawalOfferings(completion *careplan.WithdrawalCompletion) map[int64][]careplan.CareExitSourceOffering {
	if completion == nil || completion.StudentID == nil {
		return nil
	}
	return map[int64][]careplan.CareExitSourceOffering{*completion.StudentID: withdrawalSourceOfferings(*completion)}
}

func carePlanDate(value *calendar.Date) *careplan.Date {
	if value == nil {
		return nil
	}
	converted := careplan.Date(*value)
	return &converted
}

func calendarDate(value *careplan.Date) *calendar.Date {
	if value == nil {
		return nil
	}
	converted := calendar.Date(*value)
	return &converted
}
