package domain

import (
	"context"
	"errors"
	"time"
)

// ErrStudentNoteTeamAudience reports a note narrower than the whole team
// (#6854): only a note every colleague may read is announced to all of them.
var ErrStudentNoteTeamAudience = errors.New("only a note for the whole team can be shared with the team")

// MaxStudentNoteTeamNotices bounds the reader's unread list. A marker older
// than the newest fifty is still unread on its note; the list in the header
// is a pointer, not an archive.
const MaxStudentNoteTeamNotices = 50

// InformStudentNoteTeam tells the colleagues about one of the actor's notes.
type InformStudentNoteTeam struct {
	NoteID              int64
	StudentID           int64
	ActorAccountID      int64
	RecipientAccountIDs []int64
	Notify              func(context.Context, []int64) error
}

// Recipients returns the distinct colleagues to tell, without the actor and
// in the order given.
func (i InformStudentNoteTeam) Recipients() []int64 {
	seen := make(map[int64]bool, len(i.RecipientAccountIDs))
	out := make([]int64, 0, len(i.RecipientAccountIDs))
	for _, id := range i.RecipientAccountIDs {
		if id <= 0 || id == i.ActorAccountID || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// AllowsTeamNotice reports whether the actor may tell the team about this
// note: the author, about a note of their own that every colleague may read.
// A carried-over hint and a day hint from moto schule have no author here.
func (n StudentNote) AllowsTeamNotice(actorAccountID int64) error {
	if n.Origin != StudentNoteOriginStaff || n.AuthorAccountID == nil || *n.AuthorAccountID != actorAccountID {
		return ErrStudentNoteNotAuthor
	}
	if n.Visibility != StudentNoteVisibilityAllStaff {
		return ErrStudentNoteTeamAudience
	}
	return nil
}

// StudentNoteTeamNotice is one unread marker as the store reads it.
type StudentNoteTeamNotice struct {
	NoteID          int64
	StudentID       int64
	AuthorAccountID *int64
	InformedAt      time.Time
}

// StudentNoteTeamNoticeRevision identifies the announcement loaded by a reader.
type StudentNoteTeamNoticeRevision struct {
	NoteID     int64
	InformedAt time.Time
}
