package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HomeworkEntry is one homework entry of a class and day as the projection
// reads it.
type HomeworkEntry struct {
	ID          int64
	Subject     *string
	Description string
}

// HomeworkFeedbackEntry is the OGS feedback of one child on one day.
type HomeworkFeedbackEntry struct {
	StudentID int64
	Status    string
	Note      *string
}

// HomeworkBook reads and writes the homework of the classes and reads the
// OGS feedback on it (#4796). The root binds Timetable's homework to it.
// Refusals answer errors.Is for classday.ErrHomeworkInvalid and
// classday.ErrHomeworkNotFound.
type HomeworkBook interface {
	ClassHomework(ctx context.Context, schoolClass string, date calendar.Date) ([]HomeworkEntry, error)
	HomeworkFeedbackOn(ctx context.Context, date calendar.Date, studentIDs []int64) ([]HomeworkFeedbackEntry, error)
	AddHomework(ctx context.Context, in classday.HomeworkWrite) (HomeworkEntry, error)
	ChangeHomework(ctx context.Context, in classday.HomeworkChange) (HomeworkEntry, error)
	DeleteHomework(ctx context.Context, id int64, schoolClass string) error
}

// HomeworkDependencies wire the homework seam (#4796). Without Homework the
// report carries no homework and the writes answer
// classday.ErrHomeworkNotConfigured.
type HomeworkDependencies struct {
	Homework HomeworkBook
}

// homework is the third write seam of the class-day view: the Lehrkraft
// enters the homework of an assigned class and changes or deletes every entry
// of it; the OGS feedback per child is read into the day report.
type homework struct {
	book HomeworkBook
}

func newHomework(deps ClassDayDependencies) homework {
	return homework{book: deps.Homework}
}

// writable returns the store of the writes, or ErrHomeworkNotConfigured.
func (h homework) writable() (HomeworkBook, error) {
	if h.book == nil {
		return nil, classday.ErrHomeworkNotConfigured
	}
	return h.book, nil
}

func (h homework) AddHomework(ctx context.Context, in classday.HomeworkWrite) (*classday.DayHomework, error) {
	book, err := h.writable()
	if err != nil {
		return nil, err
	}
	if _, err := parseDate(in.Date); err != nil {
		return nil, fmt.Errorf("%w: %w", classday.ErrHomeworkInvalid, err)
	}
	entry, err := book.AddHomework(ctx, in)
	if err != nil {
		return nil, err
	}
	return dayHomework(entry), nil
}

func (h homework) ChangeHomework(ctx context.Context, in classday.HomeworkChange) (*classday.DayHomework, error) {
	book, err := h.writable()
	if err != nil {
		return nil, err
	}
	if in.ID <= 0 {
		return nil, classday.ErrHomeworkInvalid
	}
	entry, err := book.ChangeHomework(ctx, in)
	if err != nil {
		return nil, err
	}
	return dayHomework(entry), nil
}

func (h homework) DeleteHomework(ctx context.Context, id int64, schoolClass string) error {
	book, err := h.writable()
	if err != nil {
		return err
	}
	if id <= 0 {
		return classday.ErrHomeworkInvalid
	}
	return book.DeleteHomework(ctx, id, schoolClass)
}

// applyHomework fills in the class's homework of the date and each child's
// OGS feedback. Whether the caller may change the entries is the report's
// CanWriteHomework verdict, the same for every entry of the class.
func (h homework) applyHomework(ctx context.Context, report *classday.DayReport, date calendar.Date) error {
	if h.book == nil {
		return nil
	}
	entries, err := h.book.ClassHomework(ctx, report.SchoolClass, date)
	if err != nil {
		return fmt.Errorf("class day report: load homework: %w", err)
	}
	for _, entry := range entries {
		report.Homework = append(report.Homework, *dayHomework(entry))
	}
	return h.applyFeedback(ctx, report, date)
}

func (h homework) applyFeedback(ctx context.Context, report *classday.DayReport, date calendar.Date) error {
	studentIDs := make([]int64, 0, len(report.Rows))
	for _, row := range report.Rows {
		if !row.ListEntry && row.StudentID > 0 {
			studentIDs = append(studentIDs, row.StudentID)
		}
	}
	if len(studentIDs) == 0 {
		return nil
	}
	feedback, err := h.book.HomeworkFeedbackOn(ctx, date, studentIDs)
	if err != nil {
		return fmt.Errorf("class day report: load homework feedback: %w", err)
	}
	byStudent := make(map[int64]classday.DayHomeworkFeedback, len(feedback))
	for _, entry := range feedback {
		value := classday.DayHomeworkFeedback{Status: entry.Status}
		if entry.Note != nil {
			value.Note = *entry.Note
		}
		byStudent[entry.StudentID] = value
	}
	for i := range report.Rows {
		row := &report.Rows[i]
		if value, ok := byStudent[row.StudentID]; ok && !row.ListEntry {
			row.HomeworkFeedback = &value
		}
	}
	return nil
}

func dayHomework(entry HomeworkEntry) *classday.DayHomework {
	out := &classday.DayHomework{ID: entry.ID, Description: entry.Description}
	if entry.Subject != nil {
		out.Subject = *entry.Subject
	}
	return out
}
