package domain

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

var (
	// ErrStudentNoteInvalid reports a malformed note.
	ErrStudentNoteInvalid = errors.New("invalid student note")
	// ErrStudentNoteNotFound reports a note that does not exist, is
	// soft-deleted, or belongs to another tenant — the three cases a caller
	// must not be able to tell apart.
	ErrStudentNoteNotFound = errors.New("student note not found")
	// ErrStudentNoteNotAuthor reports an edit by somebody other than the
	// author. Deleting is a leadership matter and has its own gate; correcting
	// the wording of an entry is not, and never becomes one.
	ErrStudentNoteNotAuthor = errors.New("only the author may edit a student note")
	// ErrStudentNoteImmutable reports a write against a note that carries no
	// author at all: the durable hints carried over from the former master-data
	// field. They may be deleted or replaced, never silently rewritten under a
	// name that was never on them.
	ErrStudentNoteImmutable = errors.New("a carried-over note has no author and cannot be edited")
	// ErrStudentNoteDeleteForbidden reports a removal without leadership of the
	// referenced group or the child group, or without administration rights.
	ErrStudentNoteDeleteForbidden = errors.New("deleting a student note requires group leadership or administration")
)

// MaxStudentNoteRunes bounds one note. The column is TEXT; the bound keeps a
// single entry readable in a timeline and stops an unbounded paste.
const MaxStudentNoteRunes = 4000

// Note lifetimes. A permanent note is the durable hint that belongs with the
// master data; a journal note is one entry in the chronicle.
const (
	StudentNoteKindPermanent = "permanent"
	StudentNoteKindJournal   = "journal"
)

// Note visibilities, from widest to narrowest.
const (
	// StudentNoteVisibilityAllStaff is today's read scope for child data:
	// everyone who may read the child.
	StudentNoteVisibilityAllStaff = "all_staff"
	// StudentNoteVisibilityCareTeam is the staff who actually have the child —
	// through their own group, their own activity, or their own school class.
	StudentNoteVisibilityCareTeam = "care_team"
	// StudentNoteVisibilityGroupLeads is the leadership of the group or
	// activity the note refers to.
	StudentNoteVisibilityGroupLeads = "group_leads"
)

// Note origins. A carried-over note has no author because the column it came
// from stored a text, not an authorship.
const (
	StudentNoteOriginStaff      = "staff"
	StudentNoteOriginMasterData = "master_data"
)

// Note categories. Free text alone makes a timeline unfilterable; an open
// vocabulary makes it unreadable. These six cover what a Klassenbuch records.
const (
	StudentNoteCategoryGeneral       = "general"
	StudentNoteCategoryBehaviour     = "behaviour"
	StudentNoteCategoryConversation  = "conversation"
	StudentNoteCategoryIncident      = "incident"
	StudentNoteCategoryPositive      = "positive"
	StudentNoteCategoryParentContact = "parent_contact"
)

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

// ValidStudentNoteCategory reports whether category is a known category. The
// empty string is valid and means "no category".
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

// StudentNoteSubject is the single thing a note is about besides the child.
// At most one field is set; all of them empty is the general note that used to
// live in the master-data field.
type StudentNoteSubject struct {
	// Date is the calendar day the entry describes. Journal notes only.
	Date *calendar.Date
	// ActivityGroupID names the activity (Angebot/Kurs) the note is about.
	// One occurrence of it needs no reference of its own: it is this plus
	// Date, the same way the timetable separates its durable Wochennotiz from
	// its Tagesnotiz.
	ActivityGroupID *int64
	// EducationGroupID names the child's OGS group.
	EducationGroupID *int64
}

// References reports how many group references the subject carries. The table
// permits at most one; two would leave the group_leads audience with two
// different leaderships to ask.
func (s StudentNoteSubject) References() int {
	count := 0
	for _, reference := range []*int64{s.ActivityGroupID, s.EducationGroupID} {
		if reference != nil {
			count++
		}
	}
	return count
}

// HasReference reports whether the note points at a group or activity.
func (s StudentNoteSubject) HasReference() bool { return s.References() > 0 }

// StudentNote is one stored entry of a child's note card.
type StudentNote struct {
	ID              int64
	TenantID        int64
	StudentID       int64
	AuthorAccountID *int64
	Origin          string
	Kind            string
	Visibility      string
	Category        string
	Body            string
	Subject         StudentNoteSubject
	// AuthorName is resolved on read, not stored: a note carries the account
	// that wrote it, and the name follows that person's current record.
	AuthorName string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Edited reports whether the note was changed after it was written. The
// timeline marks those entries so a reader knows the wording moved.
func (n StudentNote) Edited() bool { return n.UpdatedAt.After(n.CreatedAt) }

// CreateStudentNote is a new entry.
type CreateStudentNote struct {
	StudentID         int64
	AuthorAccountID   int64
	Kind              string
	Visibility        string
	Category          string
	Body              string
	Subject           StudentNoteSubject
	RevalidateSubject func(context.Context) error
}

// UpdateStudentNote rewrites the parts of an entry its author may correct.
// Its group reference stays fixed; its day changes with a correction or a
// conversion between a dated entry and an undated durable hint.
type UpdateStudentNote struct {
	ID int64
	// StudentID is the child the caller believes the note belongs to; the
	// service re-checks it under the row lock.
	StudentID      int64
	ActorAccountID int64
	Kind           string
	Visibility     string
	Category       string
	Body           string
	SubjectDate    *calendar.Date
}

// DeleteStudentNote soft-deletes one entry. Who may ask is the caller's
// decision; that it stays recoverable, and that it really is an entry of that
// child, is this owner's.
type DeleteStudentNote struct {
	ID int64
	// StudentID is the child the caller believes the note belongs to. The
	// service re-checks it under the row lock, so a note that moved between
	// the caller's authorization read and this write cannot be deleted on the
	// strength of the old answer.
	StudentID            int64
	ActorAccountID       int64
	Authorization        StudentNoteDeleteAuthorization
	ResolveAuthorization func(context.Context) (StudentNoteDeleteAuthorization, error)
}

// StudentNoteDeleteAuthorization is the caller-resolved leadership data that
// the owner evaluates with the locked note. It names facts rather than another
// module's policy types to keep the ownership boundary one-way.
type StudentNoteDeleteAuthorization struct {
	Admin                 bool
	LedActivityGroupIDs   []int64
	LedEducationGroupIDs  []int64
	ChildEducationGroupID *int64
}

// CurrentAuthorization reloads the caller-owned authorization facts when the
// inbound adapter supplied a resolver for this write.
func (d DeleteStudentNote) CurrentAuthorization(ctx context.Context) (StudentNoteDeleteAuthorization, error) {
	if d.ResolveAuthorization == nil {
		return d.Authorization, nil
	}
	return d.ResolveAuthorization(ctx)
}

// StudentNoteFilter is the audience-resolved read. The caller resolves WHO the
// reader is — that needs the reader's groups, activities and school classes,
// which belong to other owners — and hands the result down as data. This owner
// turns it into one predicate and never guesses at an audience itself.
type StudentNoteFilter struct {
	StudentID int64
	// NoteID narrows the read to one note of that child. Zero returns all of
	// them. It is a filter rather than a second read path so a named note goes
	// through exactly the same audience predicate as the timeline.
	NoteID int64
	// Visibilities are the audiences the reader belongs to for this child.
	// Empty means the reader is in none of them, which still returns their own
	// notes and, for a leadership reader, the notes of the groups they lead.
	Visibilities []string
	// LedActivityGroupIDs and LedEducationGroupIDs are the groups the reader
	// leads. A group_leads note referring to one of them is visible even when
	// the reader is in no other audience.
	LedActivityGroupIDs  []int64
	LedEducationGroupIDs []int64
	// ReaderAccountID always sees its own notes. An author who leaves a group
	// keeps access to what they wrote; losing sight of your own words is not a
	// privacy gain.
	ReaderAccountID int64
	// Kind narrows the read to one lifetime. Empty returns both.
	Kind string
}

// Validate checks a new note against the table's rules, so an invalid write
// fails before it reaches Postgres and returns a constraint name to a user.
func (c CreateStudentNote) Validate() error {
	if c.StudentID <= 0 || c.AuthorAccountID <= 0 {
		return ErrStudentNoteInvalid
	}
	if !ValidStudentNoteKind(c.Kind) || !ValidStudentNoteVisibility(c.Visibility) ||
		!ValidStudentNoteCategory(c.Category) {
		return ErrStudentNoteInvalid
	}
	if c.Body == "" || len([]rune(c.Body)) > MaxStudentNoteRunes {
		return ErrStudentNoteInvalid
	}
	if c.Subject.References() > 1 {
		return ErrStudentNoteInvalid
	}
	if c.Visibility == StudentNoteVisibilityGroupLeads && !c.Subject.HasReference() {
		return ErrStudentNoteInvalid
	}
	if (c.Kind == StudentNoteKindPermanent && c.Subject.Date != nil) ||
		(c.Kind == StudentNoteKindJournal && c.Subject.Date == nil) {
		return ErrStudentNoteInvalid
	}
	return nil
}

// Validate checks a correction. It repeats the ordinary request rules; the
// service checks the requested day and the stored, immutable group reference
// together because those constraints span both versions of the note.
func (u UpdateStudentNote) Validate() error {
	if u.ID <= 0 || u.ActorAccountID <= 0 {
		return ErrStudentNoteInvalid
	}
	if !ValidStudentNoteKind(u.Kind) || !ValidStudentNoteVisibility(u.Visibility) ||
		!ValidStudentNoteCategory(u.Category) {
		return ErrStudentNoteInvalid
	}
	if u.Body == "" || len([]rune(u.Body)) > MaxStudentNoteRunes {
		return ErrStudentNoteInvalid
	}
	return nil
}

// AllowsUpdate reports whether the stored note may take this correction. It
// re-checks the coupled constraints against the requested day: a durable hint
// has no day, every entry has one, and a note narrowed to leadership refers to
// a group.
func (n StudentNote) AllowsUpdate(update UpdateStudentNote) error {
	if n.AuthorAccountID == nil {
		return ErrStudentNoteImmutable
	}
	if *n.AuthorAccountID != update.ActorAccountID {
		return ErrStudentNoteNotAuthor
	}
	if update.Kind == StudentNoteKindPermanent && update.SubjectDate != nil {
		return ErrStudentNoteInvalid
	}
	if update.Kind == StudentNoteKindJournal && update.SubjectDate == nil {
		return ErrStudentNoteInvalid
	}
	if update.Visibility == StudentNoteVisibilityGroupLeads && !n.Subject.HasReference() {
		return ErrStudentNoteInvalid
	}
	return nil
}

// AllowsDelete evaluates the caller-provided administration and leadership
// facts only after the note itself is row-locked. A general note belongs to
// the child's current OGS group; a referenced note belongs to that reference.
func (n StudentNote) AllowsDelete(authorization StudentNoteDeleteAuthorization) error {
	if authorization.Admin {
		return nil
	}
	if n.Visibility == StudentNoteVisibilityGroupLeads && !n.Subject.HasReference() {
		return ErrStudentNoteDeleteForbidden
	}
	if n.Subject.ActivityGroupID != nil &&
		slices.Contains(authorization.LedActivityGroupIDs, *n.Subject.ActivityGroupID) {
		return nil
	}
	if n.Subject.EducationGroupID != nil &&
		slices.Contains(authorization.LedEducationGroupIDs, *n.Subject.EducationGroupID) {
		return nil
	}
	if n.Subject.HasReference() || authorization.ChildEducationGroupID == nil ||
		!slices.Contains(authorization.LedEducationGroupIDs, *authorization.ChildEducationGroupID) {
		return ErrStudentNoteDeleteForbidden
	}
	return nil
}
