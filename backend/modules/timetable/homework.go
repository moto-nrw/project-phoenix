package timetable

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Homework and the OGS feedback on it (#4796). The Lehrkraft of a class
// enters what the class has to do on one day (education.class_homework); in
// a Lernzeit block the OGS records per child whether it was finished
// (education.homework_feedback). Timetable owns both tables; the class-day
// projection reaches them through HomeworkBook. Parents see none of it.

// Feedback statuses the OGS records per child and day.
const (
	HomeworkFeedbackDone      = "done"
	HomeworkFeedbackNotDone   = "not_done"
	HomeworkFeedbackDifficult = "difficult"
)

// Length limits in characters; the migration's checks hold the same values.
const (
	HomeworkSubjectMaxLength      = 60
	HomeworkDescriptionMaxLength  = 2000
	HomeworkFeedbackNoteMaxLength = 200
)

var (
	// ErrInvalidHomework refuses an entry without a class, a valid date, a
	// text, an author, or with a text beyond its limit.
	ErrInvalidHomework = errors.New("invalid homework entry")
	// ErrHomeworkNotFound means the entry does not exist, belongs to another
	// class, or was entered by somebody else.
	ErrHomeworkNotFound = errors.New("homework entry not found")
	// ErrInvalidHomeworkFeedback refuses an unknown status, a note beyond its
	// limit, or a feedback without child, class or date.
	ErrInvalidHomeworkFeedback = errors.New("invalid homework feedback")
	// ErrHomeworkFeedbackWithoutHomework refuses a feedback for a child whose
	// class has no homework on that day.
	ErrHomeworkFeedbackWithoutHomework = errors.New("the child's class has no homework on this day")
	// ErrHomeworkRetentionWindow refuses a retention window that is not
	// positive: nothing is deleted on a wrong period.
	ErrHomeworkRetentionWindow = errors.New("homework retention window must be positive")
)

// HomeworkEntry is one stored homework of a class and day.
type HomeworkEntry struct {
	ID          int64
	SchoolClass string
	Date        calendar.Date
	Subject     *string
	Description string
	// CreatedBy is the users.staff_school_memberships row of the author;
	// nil once that membership is gone.
	CreatedBy *int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// HomeworkInput is a new entry as the Lehrkraft enters it.
type HomeworkInput struct {
	SchoolClass string
	Date        calendar.Date
	Subject     *string
	Description string
	// StaffID is the author's users.staff_school_memberships row.
	StaffID int64
}

// HomeworkChange replaces subject and text of one of the author's own
// entries of the class.
type HomeworkChange struct {
	ID          int64
	SchoolClass string
	Subject     *string
	Description string
	StaffID     int64
}

// HomeworkRef names one of the author's own entries of the class.
type HomeworkRef struct {
	ID          int64
	SchoolClass string
	StaffID     int64
}

// HomeworkFeedback is the OGS feedback of one child and day.
type HomeworkFeedback struct {
	StudentID int64
	Date      calendar.Date
	Status    string
	Note      *string
	UpdatedAt time.Time
}

// HomeworkFeedbackInput records the feedback of one child. SchoolClass is
// the child's class: the feedback needs homework of that class on Date.
type HomeworkFeedbackInput struct {
	StudentID   int64
	SchoolClass string
	Date        calendar.Date
	Status      string
	Note        *string
}

// HomeworkQuery reads the homework of classes and the feedback of children
// on one day. Class names match trimmed and case-insensitive.
type HomeworkQuery interface {
	ClassHomework(ctx context.Context, classes []string, date calendar.Date) ([]HomeworkEntry, error)
	HomeworkFeedbackOn(ctx context.Context, date calendar.Date, studentIDs []int64) ([]HomeworkFeedback, error)
}

// HomeworkCommand writes the homework of a class and the feedback of a
// child. Which classes and children the caller may reach is decided before.
type HomeworkCommand interface {
	AddHomework(ctx context.Context, in HomeworkInput) (HomeworkEntry, error)
	// ChangeHomework and DeleteHomework reach only the author's own entries
	// of the named class; any other entry answers ErrHomeworkNotFound.
	ChangeHomework(ctx context.Context, in HomeworkChange) (HomeworkEntry, error)
	DeleteHomework(ctx context.Context, ref HomeworkRef) error
	// RecordHomeworkFeedback stores the child's feedback of the day,
	// replacing an earlier one.
	RecordHomeworkFeedback(ctx context.Context, in HomeworkFeedbackInput) (HomeworkFeedback, error)
	// DeleteExpiredHomework deletes the entries and feedback dated before the
	// first Berlin calendar day a window of retentionDays keeps. It joins the
	// school's cleanup transaction.
	DeleteExpiredHomework(ctx context.Context, retentionDays int) (int, error)
}

// HomeworkBook is the homework capability of the Timetable owner.
type HomeworkBook interface {
	HomeworkQuery
	HomeworkCommand
}

// NormalizedHomework validates and trims an entry's subject and text. An
// empty subject becomes nil.
func NormalizedHomework(subject *string, description string) (*string, string, error) {
	description = strings.TrimSpace(description)
	if description == "" || utf8.RuneCountInString(description) > HomeworkDescriptionMaxLength {
		return nil, "", ErrInvalidHomework
	}
	trimmed, ok := optionalText(subject, HomeworkSubjectMaxLength)
	if !ok {
		return nil, "", ErrInvalidHomework
	}
	return trimmed, description, nil
}

// NormalizedHomeworkFeedback validates the status and trims the note. An
// empty note becomes nil.
func NormalizedHomeworkFeedback(status string, note *string) (*string, error) {
	switch status {
	case HomeworkFeedbackDone, HomeworkFeedbackNotDone, HomeworkFeedbackDifficult:
	default:
		return nil, ErrInvalidHomeworkFeedback
	}
	trimmed, ok := optionalText(note, HomeworkFeedbackNoteMaxLength)
	if !ok {
		return nil, ErrInvalidHomeworkFeedback
	}
	return trimmed, nil
}

// optionalText trims an optional text; empty becomes nil, a text beyond
// limit characters is refused.
func optionalText(value *string, limit int) (*string, bool) {
	if value == nil {
		return nil, true
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil, true
	}
	if utf8.RuneCountInString(trimmed) > limit {
		return nil, false
	}
	return &trimmed, true
}
