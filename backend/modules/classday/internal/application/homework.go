package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HomeworkEntry is one homework entry of a class and day as the projection
// reads it. CreatedBy is the author's users.staff_school_memberships row.
type HomeworkEntry struct {
	ID          int64
	Subject     *string
	Description string
	CreatedBy   *int64
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
	DeleteHomework(ctx context.Context, id int64, schoolClass string, staffID int64) error
}

// HomeworkDependencies wire the homework seam (#4796). Without Homework the
// report carries no homework and the writes answer
// classday.ErrHomeworkNotConfigured.
type HomeworkDependencies struct {
	Homework HomeworkBook
}

// homework is the third write seam of the class-day view: the Lehrkraft
// enters the homework of an assigned class; the OGS feedback per child is
// read into the day report.
type homework struct {
	book   HomeworkBook
	caller ports.Caller
}

func newHomework(deps ClassDayDependencies) homework {
	return homework{book: deps.Homework, caller: deps.Caller}
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
	return dayHomework(entry, in.StaffID), nil
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
	return dayHomework(entry, in.StaffID), nil
}

func (h homework) DeleteHomework(ctx context.Context, id int64, schoolClass string, staffID int64) error {
	book, err := h.writable()
	if err != nil {
		return err
	}
	if id <= 0 {
		return classday.ErrHomeworkInvalid
	}
	return book.DeleteHomework(ctx, id, schoolClass, staffID)
}

// applyHomework fills in the class's homework of the date and each child's
// OGS feedback. Own entries are marked for the caller's staff row; a caller
// without one owns nothing.
func (h homework) applyHomework(ctx context.Context, report *classday.DayReport, date calendar.Date) error {
	if h.book == nil {
		return nil
	}
	entries, err := h.book.ClassHomework(ctx, report.SchoolClass, date)
	if err != nil {
		return fmt.Errorf("class day report: load homework: %w", err)
	}
	staffID, err := h.callerStaffID(ctx, len(entries) > 0)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		report.Homework = append(report.Homework, *dayHomework(entry, staffID))
	}
	return h.applyFeedback(ctx, report, date)
}

// callerStaffID resolves the caller's staff row only when an entry needs
// the own flag.
func (h homework) callerStaffID(ctx context.Context, needed bool) (int64, error) {
	if !needed || h.caller == nil {
		return 0, nil
	}
	staffID, err := h.caller.CurrentStaffID(ctx)
	if errors.Is(err, classday.ErrStaffRecordRequired) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("class day report: resolve staff for homework: %w", err)
	}
	return staffID, nil
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

func dayHomework(entry HomeworkEntry, staffID int64) *classday.DayHomework {
	out := &classday.DayHomework{ID: entry.ID, Description: entry.Description}
	if entry.Subject != nil {
		out.Subject = *entry.Subject
	}
	out.Own = staffID > 0 && entry.CreatedBy != nil && *entry.CreatedBy == staffID
	return out
}
