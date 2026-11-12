package peopledirectory

import (
	"context"
	"errors"
	"time"
)

// Telling the team about a note (#6854). The author saves a note and, with a
// second action, tells the colleagues who may read it. Each recipient gets an
// unread marker that links to the child's notes; it clears when the person
// opens them. Who the colleagues are, and how they hear about it on their
// devices, is the caller's to bind: the staff directory and Delivery are
// built after this owner.

// ErrStudentNoteTeamAudience reports a note whose audience is narrower than
// the whole team. Only a note every colleague may read is announced to every
// colleague; the narrower audiences have no recipient list of their own.
var ErrStudentNoteTeamAudience = errors.New("only a note for the whole team can be shared with the team")

// InformStudentNoteTeam tells the colleagues about one of the actor's notes.
type InformStudentNoteTeam struct {
	NoteID int64
	// StudentID is the child the caller authorized against; the owner
	// re-checks it under the row lock.
	StudentID      int64
	ActorAccountID int64
	// RecipientAccountIDs are the colleagues who may read the child's notes,
	// resolved by the caller. The actor is dropped from it.
	RecipientAccountIDs []int64
	// Notify runs in the same transaction after the markers are stored, with
	// the accounts that were told. Its error rolls the markers back, so the
	// caller never reports a team as informed that was not. Nil sends no
	// device notification.
	Notify func(ctx context.Context, recipientAccountIDs []int64) error
}

// StudentNoteTeamNotice is one unread marker of the reader: a note a
// colleague told the team about.
type StudentNoteTeamNotice struct {
	NoteID           int64
	StudentID        int64
	StudentFirstName string
	StudentLastName  string
	// AuthorName is the author's current name, empty when the person is gone.
	AuthorName string
	InformedAt time.Time
}

// StudentNoteTeamNoticeRevision identifies the announcement loaded by a reader.
type StudentNoteTeamNoticeRevision struct {
	NoteID     int64
	InformedAt time.Time
}

// StudentNoteTeamNotices is the capability behind "Speichern und Team
// informieren" and the unread markers. It stands beside the owner's facade.
type StudentNoteTeamNotices interface {
	// InformStudentNoteTeam stores the markers and returns how many
	// colleagues were told. It returns ErrStudentNoteNotFound for a note of
	// another child, ErrStudentNoteNotAuthor for anyone but the author and
	// ErrStudentNoteTeamAudience for a note narrower than the whole team.
	InformStudentNoteTeam(context.Context, InformStudentNoteTeam) (int, error)
	// ListUnreadStudentNoteTeamNotices returns the reader's unread markers,
	// newest first. A deleted note, a note no longer meant for the whole team
	// and a child that left no longer appear.
	ListUnreadStudentNoteTeamNotices(ctx context.Context, readerAccountID int64) ([]StudentNoteTeamNotice, error)
	// MarkStudentNoteTeamNoticesRead clears only the observed revisions for
	// one child and returns the markers still unread. A new announcement
	// arriving while the notes load must be read separately.
	MarkStudentNoteTeamNoticesRead(ctx context.Context, readerAccountID, studentID int64, observed []StudentNoteTeamNoticeRevision) ([]StudentNoteTeamNotice, error)
}
