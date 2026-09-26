package departure

import "errors"

// Sentinel errors for companion links ("läuft mit", Laufgemeinschaft).
var (
	// ErrCompanionSelfLink is returned when a child is linked to itself.
	ErrCompanionSelfLink = errors.New("a child cannot be its own departure companion")

	// ErrCompanionInvalidWeekday is returned for a weekday outside Mon..Fri.
	ErrCompanionInvalidWeekday = errors.New("companion weekday must be one of mon/tue/wed/thu/fri")

	// ErrCompanionStudentIDRequired is returned when an edge is built without a
	// usable child id on one end — which is what a missing or non-positive
	// companion_student_id in the request body decodes to. A sentinel, not a
	// fresh error: the handler's error table maps it to a 400, whereas an
	// untyped error would leak malformed client input as a 500.
	ErrCompanionStudentIDRequired = errors.New("Bitte ein Kind für die Laufgemeinschaft auswählen.") //nolint:staticcheck // user-facing German message

	// ErrCompanionWouldLoseDeparture indicates that removing a link would leave
	// the OTHER child with an accompanied ("Anderes Kind") departure plan and no
	// remaining detail — neither another link nor a free-text note. Refused
	// rather than silently narrowing that child's plan: one child's edit must
	// never change another child's departure permissions, and leaving the
	// contradiction in place would block every later edit of that child.
	//
	// The German text goes straight to the UI; the retained model layer
	// re-exports the same value so existing errors.Is call sites keep matching.
	ErrCompanionWouldLoseDeparture = errors.New("Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.") //nolint:staticcheck // ST1005: user-facing German message

	// ErrCompanionLockBusy indicates that a linked child's row is currently
	// locked by another transaction that this one may NOT wait for without
	// risking a deadlock — the companion graph is discovered while locks are
	// already held, so an id below the ones we hold has to be taken without
	// waiting (see the lock protocol on StudentRepository.lockCompanionFarEnds
	// and the student routes' lockStudentCompanionGraph).
	//
	// It is a transient, retriable conflict, not a data problem: the same
	// request succeeds once the other edit commits. Mapped to 409 so the client
	// can say "please try again" instead of showing a 500 for a legitimate edit.
	ErrCompanionLockBusy = errors.New("Ein verknüpftes Kind wird gerade an anderer Stelle bearbeitet. Bitte in einem Moment erneut speichern.") //nolint:staticcheck // ST1005: user-facing German message
)
