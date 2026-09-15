// Package studentdeletion coordinates the permanent deletion and
// anonymization of a child across owner capabilities (#2710). It owns no
// tables and performs no irreversible work itself: every row is removed or
// anonymized by its owner inside one tenant transaction, and file cleanup is
// recorded as durable intents that the owners execute after commit.
package studentdeletion

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// Deletion reasons the confirmation dialog offers. They are part of the
// audit contract and the wire contract of DELETE /students/{id}.
const (
	ReasonTestData       = "test_data"
	ReasonIncorrectEntry = "incorrect_entry"
	ReasonDuplicate      = "duplicate"
	ReasonPrivacyRequest = "privacy_request"
	ReasonGraduatePurge  = "graduate_purge"
	// ReasonRetentionExpired is the one reason that describes a NORMAL end of
	// a record's life: the child left regularly and the retention period ran
	// out (#2487). It is only valid for a child whose care has ended.
	ReasonRetentionExpired = "retention_expired"
)

// Sentinel errors. The German texts are user-facing and part of the HTTP
// contract; the HTTP adapter maps the sentinels to status codes and codes.
var (
	ErrUnauthorized    = errors.New("student deletion requires an authorized staff member with users:delete")
	ErrStudentNotFound = errors.New("student not found")
	// ErrNotGraduated refuses a purge for a child restored between the
	// Abgänge list and the locked re-read.
	ErrNotGraduated = errors.New("Kind ist kein Abgänger mehr und wurde nicht gelöscht. Bitte Liste neu laden.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrGraduatedUnderLock refuses an ordinary deletion for a child who
	// graduated while the request was in flight; the adapter answers the
	// same 404 the alumnus gate returns.
	ErrGraduatedUnderLock   = errors.New("student graduated between snapshot and lock")
	ErrPreviewChanged       = errors.New("Die Daten haben sich seit der Vorschau geändert. Bitte Auswirkungen erneut prüfen.")          //nolint:staticcheck // ST1005: user-facing German message
	ErrConfirmationMismatch = errors.New("Der eingegebene Name stimmt nicht mit dem Kind überein.")                                     //nolint:staticcheck // ST1005: user-facing German message
	ErrNotAcknowledged      = errors.New("Die Unwiderruflichkeit der Löschung muss bestätigt werden.")                                  //nolint:staticcheck // ST1005: user-facing German message
	ErrInvalidReason        = errors.New("Ungültiger Löschgrund.")                                                                      //nolint:staticcheck // ST1005: user-facing German message
	ErrAlumnus              = errors.New("Abgänger werden über den Jahrgangswechsel endgültig gelöscht.")                               //nolint:staticcheck // ST1005: user-facing German message
	ErrRetentionNotEnded    = errors.New("Der Grund „Aufbewahrungsfrist abgelaufen“ gilt nur für Kinder, deren Betreuung beendet ist.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionWouldLoseDeparture refuses a deletion that would strand a
	// linked child: their plan says "Anderes Kind" for a weekday this child
	// covered and nothing else answers "mit wem".
	ErrCompanionWouldLoseDeparture = errors.New("Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionLockBusy is the retriable refusal when a linked child's row
	// is held by a concurrent edit this transaction must not wait for.
	ErrCompanionLockBusy         = errors.New("Ein verknüpftes Kind wird gerade an anderer Stelle bearbeitet. Bitte in einem Moment erneut speichern.") //nolint:staticcheck // ST1005: user-facing German message
	ErrWithdrawalNotFound        = errors.New("Diese Abmeldung gibt es nicht oder nicht mehr.")                                                         //nolint:staticcheck // ST1005: user-facing German message
	ErrWithdrawalAlreadyResolved = errors.New("Die Abmeldung wurde bereits erledigt oder ist nicht mehr aktuell.")                                      //nolint:staticcheck // ST1005: user-facing German message
)

// Counts groups every child-owned or child-linked row affected by a permanent
// deletion. The groups are deliberately user-facing: the API and the
// confirmation dialog expose these names, not the physical table layout.
type Counts struct {
	TimetableAssignments int `json:"timetable_assignments"`
	ActivityEnrollments  int `json:"activity_enrollments"`
	AttendanceRecords    int `json:"attendance_records"`
	CareSchedules        int `json:"care_schedules"`
	GuardianLinks        int `json:"guardian_links"`
	CompanionLinks       int `json:"companion_links"`
	Communications       int `json:"communications"`
	Consents             int `json:"consents"`
	EnrollmentReferences int `json:"enrollment_references"`
	OtherRecords         int `json:"other_records"`
}

// Total is the number of dependent rows the deletion removes or unlinks. The
// student and person rows themselves are intentionally not included.
func (c Counts) Total() int {
	return c.TimetableAssignments + c.ActivityEnrollments + c.AttendanceRecords + c.CareSchedules +
		c.GuardianLinks + c.CompanionLinks + c.Communications + c.Consents + c.EnrollmentReferences + c.OtherRecords
}

// Actor is the authenticated principal a deletion is attributed to.
type Actor struct {
	TenantID  int64
	AccountID int64
}

// Confirmation is what the actor typed after reading the preview.
type Confirmation struct {
	ExpectedFingerprint string
	ConfirmationName    string
	Reason              string
	Acknowledged        bool
}

// Preview is the owner-consistent impact snapshot. Fingerprint binds the
// confirmation to the exact student, person and counts it was shown for.
type Preview struct {
	StudentID        int64
	PersonID         int64
	ConfirmationName string
	Fingerprint      string
	Counts           Counts
	Alumnus          bool
	CareEnded        bool
	PhotoPath        string
	CompanionIDs     []int64
	personUpdatedAt  time.Time
}

// Result reports what one committed deletion changed.
type Result struct {
	StudentID           int64
	PersonID            int64
	Reason              string
	Counts              Counts
	PrimaryRowsDeleted  int
	PhotoPath           string
	CompanionIDs        []int64
	DocumentCleanups    int
	WithdrawalsRedacted int
	HistoryAnonymized   int64
}

// Directory is the People Directory port: student and person reads under
// lock, and the owner's deletion commands.
type Directory interface {
	ReadEnrollmentStudent(ctx context.Context, id int64, lock string) (peopledirectory.EnrollmentRecord, error)
	// LockEnrollmentClassWrites takes the shared class-writes gate. Withdrawal
	// deletion holds it before the recurrence gate so a concurrent grade
	// transition cannot deadlock against the locked student re-read.
	LockEnrollmentClassWrites(ctx context.Context) error
	FindPerson(ctx context.Context, id int64) (peopledirectory.Person, error)
	FindPersonForMutation(ctx context.Context, id int64) (peopledirectory.Person, error)
	peopledirectory.StudentDeletionCommand
}

// CarePlan is the Care Plan port: companion facts, care-plan counts and the
// owner's deletion commands.
type CarePlan interface {
	ListCompanionLinks(context.Context, []int64) (map[int64][]careplan.CompanionLink, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
	CountCompanionLinks(context.Context, int64) (int, error)
	CountStudentScheduleRows(context.Context, int64) (int, error)
	CountCarePlanDeletionRecords(context.Context, int64) (careplan.CarePlanDeletionCounts, error)
	careplan.StudentDeletionCommand
}

// Timetable is the Timetable & Activities port.
type Timetable interface {
	CountStudentAssignments(context.Context, int64) (int, error)
	CountStudentRosterRemovals(context.Context, int64) (int, error)
	DeleteStudentAssignments(context.Context, int64) (int64, error)
}

// Structure is the School Structure port for the grade-transition ledger.
type Structure interface {
	CountStudentTransitionHistory(context.Context, int64) (int, error)
	AnonymizeStudentTransitionHistory(context.Context, int64) (int64, error)
}

// Conversations is the Communication port: how many conversation rows the
// child has, and the thread locks that freeze that number until commit.
type Conversations interface {
	CountStudentConversationRecords(context.Context, int64) (int, error)
	LockStudentMessageThreads(context.Context, int64) error
}

// Counter is an owner query answering one preview number for a child.
type Counter func(ctx context.Context, studentID int64) (int, error)

// Dependencies are consumer-owned ports. UnitOfWork must join an ambient
// tenant transaction or open one, with commit hooks. Authorize must resolve a
// tenant principal with users:delete who is an administrator or verified
// staff. PhotoRemoved and CompanionsChanged run after commit in the owners
// that implement them; they are wake-up hints, never the deletion itself.
type Dependencies struct {
	UnitOfWork    func(context.Context, func(context.Context) error) error
	Authorize     func(context.Context) (Actor, error)
	Today         func() string
	Now           func() time.Time
	Directory     Directory
	CarePlan      CarePlan
	Timetable     Timetable
	Structure     Structure
	Conversations Conversations

	CountAttendance           Counter
	CountVisits               Counter
	CountConsents             Counter
	CountEnrollmentReferences Counter
	CountActivityEnrollments  Counter
	CountAppointments         Counter
	CountFeedback             Counter
	CountAuditReferences      Counter
	CountGuardianInvitations  Counter

	// LockCareBookingWrites is the tenant-wide recurrence gate every
	// care-booking writer takes. The withdrawal path holds it after the shared
	// class-writes gate so a stale task cannot commit beside newly booked care
	// and a concurrent grade-transition apply cannot deadlock.
	LockCareBookingWrites func(context.Context) error
	AppendAudit           func(context.Context, Actor, Result) error
	PhotoRemoved          func(context.Context, Actor, string)
	CompanionsChanged     func(context.Context, Actor, int64)
	Observe               func(Observation)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Result    Result
	Err       error
}

type Workflow struct{ deps Dependencies }

func New(deps Dependencies) (*Workflow, error) {
	if deps.UnitOfWork == nil || deps.Authorize == nil || deps.Today == nil || deps.Now == nil ||
		deps.Directory == nil || deps.CarePlan == nil || deps.Timetable == nil || deps.Structure == nil || deps.Conversations == nil ||
		deps.CountAttendance == nil || deps.CountVisits == nil || deps.CountConsents == nil || deps.CountEnrollmentReferences == nil ||
		deps.CountActivityEnrollments == nil || deps.CountAppointments == nil || deps.CountFeedback == nil ||
		deps.CountAuditReferences == nil || deps.CountGuardianInvitations == nil ||
		deps.LockCareBookingWrites == nil || deps.AppendAudit == nil || deps.PhotoRemoved == nil || deps.CompanionsChanged == nil || deps.Observe == nil {
		return nil, errors.New("student deletion: all dependencies are required")
	}
	return &Workflow{deps: deps}, nil
}

// Preview reads the impact of deleting an active child without taking locks.
// It refuses a graduate: the Abgänge purge is the only path for those.
func (w *Workflow) Preview(ctx context.Context, studentID int64) (preview Preview, err error) {
	err = w.run(ctx, "preview", func(txCtx context.Context, _ Actor) (Result, error) {
		preview, err = w.snapshot(txCtx, studentID, false)
		if err != nil {
			return Result{}, err
		}
		if preview.Alumnus {
			return Result{}, ErrAlumnus
		}
		return Result{}, nil
	})
	if err != nil {
		return Preview{}, err
	}
	return preview, nil
}

// Execute deletes an active child after the actor confirmed the preview.
func (w *Workflow) Execute(ctx context.Context, studentID int64, confirmation Confirmation) (result Result, err error) {
	if err := validateConfirmation(confirmation); err != nil {
		return Result{}, err
	}
	err = w.run(ctx, "execute", func(txCtx context.Context, actor Actor) (Result, error) {
		result, err = w.deleteConfirmed(txCtx, actor, studentID, confirmation)
		return result, err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// PurgeGraduate hard-deletes a child that a grade transition graduated. It
// requires the alumnus state under lock and needs no typed confirmation: the
// Abgänge view is the only surface that offers it.
func (w *Workflow) PurgeGraduate(ctx context.Context, studentID int64) (result Result, err error) {
	err = w.run(ctx, "purge", func(txCtx context.Context, actor Actor) (Result, error) {
		preview, err := w.lockedSnapshot(txCtx, studentID)
		if err != nil {
			return Result{}, err
		}
		if !preview.Alumnus {
			return Result{}, ErrNotGraduated
		}
		result, err = w.mutate(txCtx, actor, preview, ReasonGraduatePurge, 2)
		return result, err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

// PreviewWithdrawal previews the deletion of the child a pending withdrawal
// task names.
func (w *Workflow) PreviewWithdrawal(ctx context.Context, completionID int64) (preview Preview, err error) {
	err = w.run(ctx, "preview_withdrawal", func(txCtx context.Context, _ Actor) (Result, error) {
		studentID, err := w.withdrawalStudent(txCtx, completionID, false)
		if err != nil {
			return Result{}, err
		}
		preview, err = w.snapshot(txCtx, studentID, false)
		if err != nil {
			return Result{}, err
		}
		if preview.Alumnus {
			return Result{}, ErrAlumnus
		}
		return Result{}, nil
	})
	if err != nil {
		return Preview{}, err
	}
	return preview, nil
}

// ExecuteWithdrawal resolves the pending withdrawal task as deleted and
// deletes its child in the same transaction. A stale preview rolls both back.
func (w *Workflow) ExecuteWithdrawal(ctx context.Context, completionID int64, confirmation Confirmation) (result Result, err error) {
	if err := validateConfirmation(confirmation); err != nil {
		return Result{}, err
	}
	err = w.run(ctx, "execute_withdrawal", func(txCtx context.Context, actor Actor) (Result, error) {
		// Shared class-writes gate BEFORE the recurrence gate: a grade
		// transition takes class-writes exclusively first, then recurrence
		// (lockRecurrenceThenTransitions). Taking recurrence here and the
		// shared class gate only later (inside a locked ReadEnrollmentStudent)
		// would let the two transactions wait on each other until PostgreSQL
		// aborts one. Re-entrant, so the later locked student reads stay a no-op.
		if err := w.deps.Directory.LockEnrollmentClassWrites(txCtx); err != nil {
			return Result{}, err
		}
		if err := w.deps.LockCareBookingWrites(txCtx); err != nil {
			return Result{}, err
		}
		studentID, err := w.withdrawalStudent(txCtx, completionID, true)
		if err != nil {
			return Result{}, err
		}
		resolved, err := w.deps.CarePlan.ResolvePendingWithdrawalAsDeleted(txCtx, completionID, actor.AccountID, w.deps.Now())
		if err != nil {
			return Result{}, err
		}
		if !resolved {
			return Result{}, ErrWithdrawalAlreadyResolved
		}
		result, err = w.deleteConfirmed(txCtx, actor, studentID, confirmation)
		return result, err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func validateConfirmation(confirmation Confirmation) error {
	if strings.TrimSpace(confirmation.ExpectedFingerprint) == "" {
		return ErrPreviewChanged
	}
	if !confirmation.Acknowledged {
		return ErrNotAcknowledged
	}
	switch confirmation.Reason {
	case ReasonTestData, ReasonIncorrectEntry, ReasonDuplicate, ReasonPrivacyRequest, ReasonRetentionExpired:
		return nil
	default:
		return ErrInvalidReason
	}
}

func (w *Workflow) withdrawalStudent(ctx context.Context, completionID int64, lock bool) (int64, error) {
	studentID, err := w.deps.CarePlan.FindPendingWithdrawalStudent(ctx, completionID, lock)
	switch {
	case errors.Is(err, careplan.ErrWithdrawalNotFound):
		return 0, ErrWithdrawalNotFound
	case errors.Is(err, careplan.ErrWithdrawalNotPending):
		return 0, ErrWithdrawalAlreadyResolved
	case err != nil:
		return 0, err
	}
	return studentID, nil
}

func (w *Workflow) run(ctx context.Context, operation string, fn func(context.Context, Actor) (Result, error)) (err error) {
	started := time.Now()
	var result Result
	defer func() {
		if err != nil {
			result = Result{}
		}
		w.deps.Observe(Observation{Operation: operation, Duration: time.Since(started), Result: result, Err: err})
	}()
	return w.deps.UnitOfWork(ctx, func(txCtx context.Context) error {
		actor, err := w.deps.Authorize(txCtx)
		if err != nil {
			return err
		}
		if actor.TenantID <= 0 || actor.AccountID <= 0 {
			return ErrUnauthorized
		}
		result, err = fn(txCtx, actor)
		return err
	})
}

// deleteConfirmed is the shared body of Execute and ExecuteWithdrawal: lock,
// re-read, compare with what the actor confirmed, then mutate.
func (w *Workflow) deleteConfirmed(ctx context.Context, actor Actor, studentID int64, confirmation Confirmation) (Result, error) {
	preview, err := w.lockedSnapshot(ctx, studentID)
	if err != nil {
		return Result{}, err
	}
	if preview.Alumnus {
		// The ordinary delete hard-deletes the very row graduation preserved;
		// a child who graduated meanwhile must go through the purge instead.
		return Result{}, ErrGraduatedUnderLock
	}
	if confirmation.Reason == ReasonRetentionExpired && !preview.CareEnded {
		return Result{}, ErrRetentionNotEnded
	}
	if !equalFingerprint(preview.Fingerprint, confirmation.ExpectedFingerprint) {
		return Result{}, ErrPreviewChanged
	}
	if confirmation.ConfirmationName != preview.ConfirmationName {
		return Result{}, ErrConfirmationMismatch
	}
	return w.mutate(ctx, actor, preview, confirmation.Reason, 1)
}

// lockedSnapshot takes the locks in the blocker-defined order and then reads
// the same snapshot the preview showed:
//
//  1. Care Plan companion far ends, then People Directory student rows for
//     the subject and every far end in ascending id order (late lower ids
//     NOWAIT, so the pass cannot deadlock against a writer coming up);
//  2. the stranding check for every linked child;
//  3. Communication thread locks, so no message or read cursor can be added
//     behind the rechecked counts;
//  4. the People Directory person row FOR UPDATE;
//  5. every owner count, twice: once for the fingerprint, once to prove that
//     nothing changed between the preview and the cascade.
func (w *Workflow) lockedSnapshot(ctx context.Context, studentID int64) (Preview, error) {
	companionIDs, err := w.lockCompanionGraph(ctx, studentID)
	if err != nil {
		return Preview{}, err
	}
	if err := w.checkCompanionStranding(ctx, studentID); err != nil {
		return Preview{}, err
	}
	if err := w.deps.Conversations.LockStudentMessageThreads(ctx, studentID); err != nil {
		return Preview{}, err
	}
	preview, err := w.snapshot(ctx, studentID, true)
	if err != nil {
		return Preview{}, err
	}
	preview.CompanionIDs = companionIDs
	// Child rows can be removed by another transaction without touching the
	// locked student or person rows. Re-read every counted category before the
	// cascade so the confirmation and both audit records describe this
	// transaction's deletion, not an earlier snapshot.
	current, err := w.loadCounts(ctx, studentID, preview.PersonID)
	if err != nil {
		return Preview{}, err
	}
	if current != preview.Counts {
		return Preview{}, ErrPreviewChanged
	}
	return preview, nil
}

func (w *Workflow) snapshot(ctx context.Context, studentID int64, lock bool) (Preview, error) {
	lockMode := ""
	if lock {
		lockMode = "update"
	}
	student, err := w.deps.Directory.ReadEnrollmentStudent(ctx, studentID, lockMode)
	if errors.Is(err, peopledirectory.ErrStudentNotFound) {
		return Preview{}, ErrStudentNotFound
	}
	if err != nil {
		return Preview{}, err
	}
	var person peopledirectory.Person
	if lock {
		person, err = w.deps.Directory.FindPersonForMutation(ctx, student.PersonID)
	} else {
		person, err = w.deps.Directory.FindPerson(ctx, student.PersonID)
	}
	if err != nil {
		return Preview{}, err
	}
	counts, err := w.loadCounts(ctx, studentID, student.PersonID)
	if err != nil {
		return Preview{}, err
	}
	fingerprint, err := fingerprintOf(student.UpdatedAt, person.UpdatedAt, counts)
	if err != nil {
		return Preview{}, err
	}
	preview := Preview{
		StudentID: studentID, PersonID: student.PersonID,
		ConfirmationName: strings.TrimSpace(person.FirstName + " " + person.LastName),
		Fingerprint:      fingerprint, Counts: counts,
		Alumnus:         student.Status == peopledirectory.StudentStatusAlumnus,
		CareEnded:       student.EnrolledUntil != "" && w.deps.Today() > student.EnrolledUntil,
		personUpdatedAt: person.UpdatedAt,
	}
	if student.PhotoPath != nil {
		preview.PhotoPath = *student.PhotoPath
	}
	return preview, nil
}

func (w *Workflow) loadCounts(ctx context.Context, studentID, personID int64) (Counts, error) {
	var counts Counts
	var err error
	if counts.TimetableAssignments, err = w.deps.Timetable.CountStudentAssignments(ctx, studentID); err != nil {
		return Counts{}, err
	}
	if counts.ActivityEnrollments, err = w.deps.CountActivityEnrollments(ctx, studentID); err != nil {
		return Counts{}, err
	}
	visits, err := w.deps.CountVisits(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	attendance, err := w.deps.CountAttendance(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	carePlanCounts, err := w.deps.CarePlan.CountCarePlanDeletionRecords(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	counts.AttendanceRecords = visits + attendance + carePlanCounts.StatusDays + carePlanCounts.ExcusedRequests
	if counts.CareSchedules, err = w.deps.CarePlan.CountStudentScheduleRows(ctx, studentID); err != nil {
		return Counts{}, err
	}
	if counts.GuardianLinks, err = w.deps.Directory.CountStudentGuardianLinks(ctx, studentID, personID); err != nil {
		return Counts{}, err
	}
	if counts.CompanionLinks, err = w.deps.CarePlan.CountCompanionLinks(ctx, studentID); err != nil {
		return Counts{}, err
	}
	invitations, err := w.deps.CountGuardianInvitations(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	conversations, err := w.deps.Conversations.CountStudentConversationRecords(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	counts.Communications = invitations + conversations + carePlanCounts.CareRequests + carePlanCounts.DataRequests
	if counts.Consents, err = w.deps.CountConsents(ctx, studentID); err != nil {
		return Counts{}, err
	}
	if counts.EnrollmentReferences, err = w.deps.CountEnrollmentReferences(ctx, studentID); err != nil {
		return Counts{}, err
	}
	rosterRemovals, err := w.deps.Timetable.CountStudentRosterRemovals(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	history, err := w.deps.Structure.CountStudentTransitionHistory(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	appointments, err := w.deps.CountAppointments(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	auditReferences, err := w.deps.CountAuditReferences(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	feedback, err := w.deps.CountFeedback(ctx, studentID)
	if err != nil {
		return Counts{}, err
	}
	counts.OtherRecords = rosterRemovals + history + appointments + auditReferences + feedback
	return counts, nil
}

// lockCompanionGraph locks the subject and the far end of every stored
// companion edge in one ascending-id pass and returns the far ends.
//
// The stored-companion snapshot is read before the locks, so an edge committed
// between the snapshot and the lock pass could have a far end this pass never
// locked. Every writer that creates or removes an edge touching a subject
// locks the subject's row first, so once the subject is held no further edges
// can appear, and one top-up pass over late edges suffices. A late edge may
// point at an id below ids already held; those downward locks are taken with
// NOWAIT so the pass either gets them or refuses the request as retriable,
// never waiting head-on against a writer coming up the other way.
func (w *Workflow) lockCompanionGraph(ctx context.Context, subjectID int64) ([]int64, error) {
	links, err := w.deps.CarePlan.ListCompanionLinks(ctx, []int64{subjectID})
	if err != nil {
		return nil, err
	}
	ids := []int64{subjectID}
	for _, link := range links[subjectID] {
		ids = append(ids, link.CompanionStudentID)
	}
	held := make(map[int64]bool, len(ids))
	var maxHeld int64
	for _, id := range ascending(ids) {
		found, err := w.lockStudent(ctx, id, "update")
		if err != nil {
			return nil, err
		}
		if !found && id == subjectID {
			return nil, ErrStudentNotFound
		}
		held[id] = true
		if id > maxHeld {
			maxHeld = id
		}
	}
	links, err = w.deps.CarePlan.ListCompanionLinks(ctx, []int64{subjectID})
	if err != nil {
		return nil, err
	}
	companionIDs := make([]int64, 0, len(links[subjectID]))
	for _, link := range links[subjectID] {
		companionIDs = append(companionIDs, link.CompanionStudentID)
	}
	for _, id := range ascending(companionIDs) {
		if held[id] {
			continue
		}
		mode := "update"
		if id < maxHeld {
			mode = "nowait"
		}
		if _, err := w.lockStudent(ctx, id, mode); err != nil {
			return nil, err
		}
		held[id] = true
	}
	return companionIDs, nil
}

func (w *Workflow) lockStudent(ctx context.Context, id int64, mode string) (bool, error) {
	_, err := w.deps.Directory.ReadEnrollmentStudent(ctx, id, mode)
	switch {
	case errors.Is(err, peopledirectory.ErrStudentNotFound):
		return false, nil
	case errors.Is(err, peopledirectory.ErrStudentLockBusy):
		return false, ErrCompanionLockBusy
	case err != nil:
		return false, err
	}
	return true, nil
}

// checkCompanionStranding refuses the deletion when a linked child would be
// left with an accompanied plan and nothing that answers "mit wem": the
// cascade removes every edge of the subject, which edits the other child's
// record, and the removal protection every list edit has must hold here too.
func (w *Workflow) checkCompanionStranding(ctx context.Context, subjectID int64) error {
	links, err := w.deps.CarePlan.ListCompanionLinks(ctx, []int64{subjectID})
	if err != nil {
		return err
	}
	removedDays := make(map[int64][]string, len(links[subjectID]))
	for _, link := range links[subjectID] {
		removedDays[link.CompanionStudentID] = append(removedDays[link.CompanionStudentID], link.Weekdays...)
	}
	if len(removedDays) == 0 {
		return nil
	}
	removed := make([]int64, 0, len(removedDays))
	for id := range removedDays {
		removed = append(removed, id)
	}
	removed = ascending(removed)
	covered, err := w.deps.CarePlan.CompanionDaysCoveredExcluding(ctx, removed, subjectID)
	if err != nil {
		return err
	}
	for _, id := range removed {
		// The rows are already held by lockCompanionGraph; this read only
		// fetches the plan under the lock this transaction owns.
		companion, err := w.deps.Directory.ReadEnrollmentStudent(ctx, id, "")
		if errors.Is(err, peopledirectory.ErrStudentNotFound) {
			continue // deleted or another tenant: nothing left to strand
		}
		if err != nil {
			return err
		}
		if companion.DepartureCompanionNote != nil && strings.TrimSpace(*companion.DepartureCompanionNote) != "" {
			continue // the free-text note carries the detail for every day
		}
		accompanied := accompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
		for _, day := range removedDays[id] {
			if accompanied[day] && !covered[id][day] {
				return ErrCompanionWouldLoseDeparture
			}
		}
	}
	return nil
}

// accompaniedWeekdays reports the weekdays on which a plan permits leaving
// with another child, from either the allowed-modes map or the day mode.
func accompaniedWeekdays(allowed map[string][]string, days map[string]string) map[string]bool {
	out := make(map[string]bool, len(peopledirectory.PickupDayOrder))
	for _, day := range peopledirectory.PickupDayOrder {
		for _, mode := range allowed[day] {
			if mode == string(peopledirectory.DepartureAccompanied) {
				out[day] = true
			}
		}
		if days[day] == string(peopledirectory.DepartureAccompanied) {
			out[day] = true
		}
	}
	return out
}

// mutate runs the owner commands in the cutover order. Every step is a
// reversible row change inside the caller's transaction; the file cleanup
// intents are durable rows the owners execute only after commit.
func (w *Workflow) mutate(ctx context.Context, actor Actor, preview Preview, reason string, primaryRows int) (Result, error) {
	now := w.deps.Now()
	result := Result{
		StudentID: preview.StudentID, PersonID: preview.PersonID, Reason: reason, Counts: preview.Counts,
		PrimaryRowsDeleted: primaryRows, PhotoPath: preview.PhotoPath, CompanionIDs: preview.CompanionIDs,
	}
	deletedAssignments, err := w.deps.Timetable.DeleteStudentAssignments(ctx, preview.StudentID)
	if err != nil {
		return result, err
	}
	if deletedAssignments != int64(preview.Counts.TimetableAssignments) {
		return result, ErrPreviewChanged
	}
	if _, err := w.deps.Directory.DeleteLegacyGuardianLinks(ctx, preview.PersonID); err != nil {
		return result, err
	}
	// Before the cascade: the document rows go with the child, the queued
	// intents do not, and they are what gets the bytes off disk.
	if result.DocumentCleanups, err = w.deps.CarePlan.QueueCareDocumentCleanupForDeletedStudent(ctx, preview.StudentID, now); err != nil {
		return result, err
	}
	if result.WithdrawalsRedacted, err = w.deps.CarePlan.RedactWithdrawalsForDeletedStudent(ctx, preview.StudentID, actor.AccountID, now); err != nil {
		return result, err
	}
	deleted, err := w.deps.Directory.DeleteStudent(ctx, preview.StudentID)
	if err != nil {
		return result, err
	}
	if deleted != 1 {
		return result, ErrPreviewChanged
	}
	if result.HistoryAnonymized, err = w.deps.Structure.AnonymizeStudentTransitionHistory(ctx, preview.StudentID); err != nil {
		return result, err
	}
	anonymized, err := w.deps.Directory.AnonymizeDeletedStudentPerson(ctx, preview.PersonID, preview.personUpdatedAt)
	if err != nil {
		return result, err
	}
	if !anonymized {
		return result, ErrPreviewChanged
	}
	if err := w.deps.AppendAudit(ctx, actor, result); err != nil {
		return result, err
	}
	if result.PhotoPath != "" {
		w.deps.PhotoRemoved(ctx, actor, result.PhotoPath)
	}
	if len(result.CompanionIDs) > 0 {
		w.deps.CompanionsChanged(ctx, actor, result.StudentID)
	}
	return result, nil
}

func fingerprintOf(studentUpdatedAt, personUpdatedAt time.Time, counts Counts) (string, error) {
	payload := struct {
		StudentUpdatedAt int64  `json:"student_updated_at"`
		PersonUpdatedAt  int64  `json:"person_updated_at"`
		Counts           Counts `json:"counts"`
	}{StudentUpdatedAt: studentUpdatedAt.UnixNano(), PersonUpdatedAt: personUpdatedAt.UnixNano(), Counts: counts}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func equalFingerprint(actual, expected string) bool {
	actualBytes, actualErr := hex.DecodeString(actual)
	expectedBytes, expectedErr := hex.DecodeString(strings.TrimSpace(expected))
	if actualErr != nil || expectedErr != nil || len(actualBytes) != len(expectedBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(actualBytes, expectedBytes) == 1
}

func ascending(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
