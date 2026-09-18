// Package carelifecycle is the Care Plan compatibility adapter for the care
// exit, companion and care-document application (#3350).
//
// The three services this package holds used to live in `services/users`,
// which the architecture policy classifies under People Directory: the whole
// "Betreuung beenden" flow, the "läuft mit" graph and the child Dokumente tab
// were written there although `users.student_care_exits`,
// `users.student_care_exit_removals`, `users.student_care_exit_source_removals`,
// `users.student_companions`, `users.student_documents` and
// `users.care_withdrawal_completions` all belong to Care Plan. The persistence
// moved to the owner in #2682; this package moves the application that drives
// it, so `api/students` reaches all three only through Care Plan.
//
// It is an adapter, not the target shape. The relocated code still speaks the
// retained `models/users` rows, the retained audit contracts, the shared
// authorization helpers and the Bun database, so it could not land on Care
// Plan's `application` point: those imports are permissions PR mode cannot
// record as debt for an existing package. The services sit behind the public
// `careplan.CareExitWorkflow`, `careplan.CompanionLinks` and
// `careplan.CareDocuments` contracts instead, so the handlers already talk to
// the owner's vocabulary while the implementation catches up (#2731).
package carelifecycle

import (
	"context"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// MaxStudentCompanions caps how many children one child may be linked to. A
// Laufgemeinschaft is a handful of neighbours walking home together; a request
// with more than this is a client bug or an attempt to use the field as a group
// list, and both should fail loudly rather than produce an unreadable grouping.
const MaxStudentCompanions = 10

// The care lifecycle, companion and document sentinels. Their German messages
// reach the user unchanged, so the text is part of the contract `api/students`
// renders; every one of them moved here verbatim with the code that raises it.
var (
	// ErrStudentNotFound indicates a child could not be found in this tenant.
	ErrStudentNotFound = errors.New("student not found")

	// ErrCompanionNotFound indicates a submitted companion is not a child of
	// this school
	ErrCompanionNotFound = errors.New("Ein ausgewähltes Kind wurde nicht gefunden.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrDuplicateCompanion indicates the same child was submitted twice
	ErrDuplicateCompanion = errors.New("Ein Kind kann nur einmal in der Laufgemeinschaft stehen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionWeekdayRequired indicates a companion was submitted without
	// any weekday — a link that applies on no day is not a link
	ErrCompanionWeekdayRequired = errors.New("Bitte mindestens einen Wochentag auswählen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionDayNotAllowed indicates a link on a weekday the child's own
	// departure plan does not permit leaving with another child
	ErrCompanionDayNotAllowed = errors.New("An diesem Tag ist \"Anderes Kind\" als Heimweg nicht erlaubt.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrTooManyCompanions indicates more companions than MaxStudentCompanions
	ErrTooManyCompanions = errors.New("Es können höchstens 10 Kinder verknüpft werden.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionAtLimit indicates the cap is already reached at the FAR end of
	// a submitted link: an edge counts for both children, so a list that is
	// short enough for the child being edited can still overflow a companion who
	// is already linked to MaxStudentCompanions others.
	ErrCompanionAtLimit = errors.New("Ein ausgewähltes Kind ist bereits mit 10 Kindern verknüpft.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionWouldLoseDeparture indicates that removing a link would leave
	// the OTHER child with an accompanied ("Anderes Kind") departure plan and no
	// remaining detail — neither another link nor a free-text note. Refused
	// rather than silently narrowing that child's plan: one child's edit must
	// never change another child's departure permissions, and leaving the
	// contradiction in place would block every later edit of that child.
	//
	// Re-exported from models/users: the People Directory departure-plan write
	// path returns the same instance, so errors.Is matches regardless of which
	// owner refused.
	ErrCompanionWouldLoseDeparture = userModels.ErrCompanionWouldLoseDeparture

	// ErrCompanionLockBusy indicates a linked child's row was locked by another
	// edit that this transaction may not wait for without risking a deadlock.
	// Transient: the same request succeeds on retry. Re-exported from
	// models/users for the same reason as the sentinel above.
	ErrCompanionLockBusy = userModels.ErrCompanionLockBusy

	// ErrCompanionsChanged indicates the submitted list was built on a snapshot
	// that is no longer the stored one — someone else changed this child's
	// Laufgemeinschaft in between.
	//
	// The submitted list REPLACES the stored one, so without this check two
	// staff members editing the same child from the same snapshot would both
	// send a complete list and the second write would silently delete the links
	// the first one committed. The row locks decide the ORDER of the two writes;
	// only the fingerprint comparison under those locks can tell that the second
	// one is stale. Retriable after a reload, hence a 409.
	ErrCompanionsChanged = errors.New("Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionExtensionNotAuthorized indicates the write path was asked to
	// widen a companion's departure plan that the caller was never authorized
	// for. Not user-facing: it can only fire if the authorization pass and the
	// write pass disagree, which the shared row locks are supposed to prevent —
	// so it must surface as a 500 (rolling the transaction back), never as a
	// 4xx the middleware would commit.
	ErrCompanionExtensionNotAuthorized = errors.New("companion departure-plan extension was not authorized")
)

// StudentChangeRecorder is the consumer-owned audit seam every care write that
// moves a tracked child field appends through. People Directory decides which
// of its fields are tracked and how a change reads; Care Plan only says that
// its write changed one.
type StudentChangeRecorder interface {
	// RecordChangesForActor diffs the before/after snapshots and appends one
	// audit row per changed tracked field, attributed to the authenticated
	// actor. A no-op when nothing tracked changed.
	RecordChangesForActor(ctx context.Context, before, after *userModels.Student, editedBy int64) error
}
