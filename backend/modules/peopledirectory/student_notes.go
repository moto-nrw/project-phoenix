package peopledirectory

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrStudentNoteInvalid reports a malformed note.
	ErrStudentNoteInvalid = errors.New("invalid student note")
	// ErrStudentNoteNotFound reports a note that does not exist, was deleted,
	// or belongs to another tenant — three cases a caller must not be able to
	// tell apart.
	ErrStudentNoteNotFound = errors.New("student note not found")
	// ErrStudentNoteNotAuthor reports an edit by somebody other than the
	// author. Removing an entry is a leadership matter with its own gate;
	// rewriting one under its author's name never is.
	ErrStudentNoteNotAuthor = errors.New("only the author may edit a student note")
	// ErrStudentNoteImmutable reports an edit of a note that has no author:
	// the durable hints carried over from the former Betreuernotizen field.
	// They can be deleted or replaced, never rewritten under a name that was
	// never on them.
	ErrStudentNoteImmutable = errors.New("a carried-over note has no author and cannot be edited")
	// ErrStudentNoteDeleteForbidden reports a removal without the leadership or
	// administration fact the caller resolved for this child.
	ErrStudentNoteDeleteForbidden = errors.New("deleting a student note requires group leadership or administration")
)

// MaxStudentNoteRunes bounds one note.
const MaxStudentNoteRunes = 4000

// Note lifetimes.
const (
	// StudentNoteKindPermanent is the durable hint shown with the master data.
	StudentNoteKindPermanent = "permanent"
	// StudentNoteKindJournal is one dated entry of the chronicle.
	StudentNoteKindJournal = "journal"
)

// Note audiences, widest first.
const (
	StudentNoteVisibilityAllStaff   = "all_staff"
	StudentNoteVisibilityCareTeam   = "care_team"
	StudentNoteVisibilityGroupLeads = "group_leads"
)

// Note origins.
const (
	StudentNoteOriginStaff      = "staff"
	StudentNoteOriginMasterData = "master_data"
)

// Note categories.
const (
	StudentNoteCategoryGeneral       = "general"
	StudentNoteCategoryBehaviour     = "behaviour"
	StudentNoteCategoryConversation  = "conversation"
	StudentNoteCategoryIncident      = "incident"
	StudentNoteCategoryPositive      = "positive"
	StudentNoteCategoryParentContact = "parent_contact"
)

// StudentNoteSubject is what a note is about besides the child. At most one
// group reference is set; one occurrence of an activity is ActivityGroupID
// together with Date.
type StudentNoteSubject struct {
	Date             *calendar.Date
	ActivityGroupID  *int64
	EducationGroupID *int64
}

// StudentNote is one entry of a child's note card.
type StudentNote struct {
	ID        int64
	StudentID int64
	// AuthorAccountID is nil exactly for the hints carried over from the
	// former master-data field; Origin then reads master_data.
	AuthorAccountID *int64
	Origin          string
	Kind            string
	Visibility      string
	Category        string
	Body            string
	Subject         StudentNoteSubject
	// AuthorName is resolved on read from the author's current person record;
	// it is empty for a carried-over hint and for an author whose record is
	// gone.
	AuthorName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Edited reports whether the wording changed after the note was written.
func (n StudentNote) Edited() bool { return n.UpdatedAt.After(n.CreatedAt) }

// CreateStudentNote is a new entry.
type CreateStudentNote struct {
	StudentID       int64
	AuthorAccountID int64
	Kind            string
	Visibility      string
	Category        string
	Body            string
	Subject         StudentNoteSubject
	// RevalidateSubject is supplied by the caller that owns group and activity
	// assignments. The note service invokes it after locking the child.
	RevalidateSubject func(context.Context) error
}

// UpdateStudentNote corrects an entry. Its group reference stays fixed, but an
// author may correct the described day or convert between entry and hint.
type UpdateStudentNote struct {
	ID int64
	// StudentID is the child the caller authorized against; the owner
	// re-checks it under the row lock.
	StudentID      int64
	ActorAccountID int64
	Kind           string
	Visibility     string
	Category       string
	Body           string
	SubjectDate    *calendar.Date
}

// DeleteStudentNote removes one entry from every timeline while keeping the
// row, so a removal stays distinguishable from a note that was never written.
type DeleteStudentNote struct {
	ID int64
	// StudentID is the child the caller authorized against; the owner
	// re-checks it under the row lock alongside the caller's authorization
	// facts, so a different note cannot be removed on the earlier read.
	StudentID      int64
	ActorAccountID int64
	Authorization  StudentNoteDeleteAuthorization
	// ResolveAuthorization reloads leadership and the child's group after the
	// note service has locked the child and note rows for this deletion.
	ResolveAuthorization func(context.Context) (StudentNoteDeleteAuthorization, error)
}

// StudentNoteDeleteAuthorization contains the caller-resolved facts that are
// evaluated again while the note row is locked. The People Directory owns the
// note and its reference, while callers own the current group assignments.
type StudentNoteDeleteAuthorization struct {
	Admin                 bool
	LedActivityGroupIDs   []int64
	LedEducationGroupIDs  []int64
	ChildEducationGroupID *int64
}

// StudentNoteAudience is the reader, resolved by the caller. This owner cannot
// resolve it: who "has" a child follows from groups, activities and school
// classes that belong to School Structure, Timetable & Activities and School
// Membership. The caller answers those questions once and passes the answer.
type StudentNoteAudience struct {
	// Visibilities are the audiences this reader belongs to for this child.
	Visibilities []string
	// LedActivityGroupIDs and LedEducationGroupIDs are the groups the reader
	// leads, which unlock the group_leads notes that refer to them.
	LedActivityGroupIDs  []int64
	LedEducationGroupIDs []int64
	// ReaderAccountID always sees its own notes, whatever else changed.
	ReaderAccountID int64
}

// StudentNoteFilter is one child's timeline as this reader may see it.
type StudentNoteFilter struct {
	StudentID int64
	// NoteID narrows the read to one note of that child. Zero returns all of
	// them; a named note passes the same audience predicate as the timeline,
	// so naming an ID never reaches past the reader's audience.
	NoteID   int64
	Audience StudentNoteAudience
	// Kind narrows to one lifetime; empty returns both.
	Kind string
}

// StudentNoteQuery reads a child's note card.
type StudentNoteQuery interface {
	// ListStudentNotes returns the visible notes of one child, newest first,
	// with each author's name resolved. Set StudentNoteFilter.NoteID to read a
	// single note through the same audience predicate.
	ListStudentNotes(context.Context, StudentNoteFilter) ([]StudentNote, error)
}

// StudentNoteCommand writes a child's note card. Deciding WHO may write, edit
// or delete stays with the caller; this owner decides what a valid note is and
// that only its author may reword one.
type StudentNoteCommand interface {
	CreateStudentNote(context.Context, CreateStudentNote) (StudentNote, error)
	// UpdateStudentNote returns ErrStudentNoteNotAuthor for anyone but the
	// author and ErrStudentNoteImmutable for a carried-over hint.
	UpdateStudentNote(context.Context, UpdateStudentNote) (StudentNote, error)
	DeleteStudentNote(context.Context, DeleteStudentNote) error
}

// ValidStudentNoteKind reports whether kind is a known lifetime.
func ValidStudentNoteKind(kind string) bool {
	return kind == StudentNoteKindPermanent || kind == StudentNoteKindJournal
}

// ValidStudentNoteVisibility reports whether visibility is a known audience.
func ValidStudentNoteVisibility(visibility string) bool {
	switch visibility {
	case StudentNoteVisibilityAllStaff, StudentNoteVisibilityCareTeam, StudentNoteVisibilityGroupLeads:
		return true
	default:
		return false
	}
}

// ValidStudentNoteCategory reports whether category is known. Empty is valid
// and means no category.
func ValidStudentNoteCategory(category string) bool {
	switch category {
	case "",
		StudentNoteCategoryGeneral, StudentNoteCategoryBehaviour, StudentNoteCategoryConversation,
		StudentNoteCategoryIncident, StudentNoteCategoryPositive, StudentNoteCategoryParentContact:
		return true
	default:
		return false
	}
}

func (m *Module) ListStudentNotes(ctx context.Context, filter StudentNoteFilter) ([]StudentNote, error) {
	if filter.StudentID <= 0 {
		return nil, &InvalidStudentError{Reason: "student notes require a positive student ID"}
	}
	if filter.Kind != "" && !ValidStudentNoteKind(filter.Kind) {
		return nil, ErrStudentNoteInvalid
	}
	return m.engine.ListStudentNotes(ctx, filter)
}

func (m *Module) CreateStudentNote(ctx context.Context, input CreateStudentNote) (StudentNote, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.StudentID <= 0 || input.AuthorAccountID <= 0 {
		return StudentNote{}, ErrStudentNoteInvalid
	}
	return m.engine.CreateStudentNote(ctx, input)
}

func (m *Module) UpdateStudentNote(ctx context.Context, input UpdateStudentNote) (StudentNote, error) {
	input.Body = strings.TrimSpace(input.Body)
	if input.ID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return StudentNote{}, ErrStudentNoteInvalid
	}
	return m.engine.UpdateStudentNote(ctx, input)
}

func (m *Module) DeleteStudentNote(ctx context.Context, input DeleteStudentNote) error {
	if input.ID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return ErrStudentNoteInvalid
	}
	return m.engine.DeleteStudentNote(ctx, input)
}
