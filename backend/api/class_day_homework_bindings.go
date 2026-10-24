package api

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	classdayCompose "github.com/moto-nrw/project-phoenix/modules/classday/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassDayHomework binds Class Day View's homework port to Timetable's
// homework (#4796): it translates the projection's writes into Timetable's
// inputs and Timetable's refusals into the projection's sentinels, so the
// school portal classifies them without naming Timetable.
type ClassDayHomework struct{ Book timetable.HomeworkBook }

// NewClassDayHomework binds the port; nil leaves it unbound, and the class
// view then carries no homework.
func NewClassDayHomework(book timetable.HomeworkBook) ClassDayHomeworkPort {
	if book == nil {
		return nil
	}
	return ClassDayHomework{Book: book}
}

// ClassDayHomeworkPort is the projection's port as the root names it.
type ClassDayHomeworkPort interface {
	ClassHomework(ctx context.Context, schoolClass string, date timezone.Date) ([]classdayCompose.HomeworkEntry, error)
	HomeworkFeedbackOn(ctx context.Context, date timezone.Date, studentIDs []int64) ([]classdayCompose.HomeworkFeedbackEntry, error)
	AddHomework(ctx context.Context, in classday.HomeworkWrite) (classdayCompose.HomeworkEntry, error)
	ChangeHomework(ctx context.Context, in classday.HomeworkChange) (classdayCompose.HomeworkEntry, error)
	DeleteHomework(ctx context.Context, id int64, schoolClass string, staffID int64) error
}

func (b ClassDayHomework) ClassHomework(ctx context.Context, schoolClass string, date timezone.Date) ([]classdayCompose.HomeworkEntry, error) {
	entries, err := b.Book.ClassHomework(ctx, []string{schoolClass}, date)
	if err != nil {
		return nil, classDayHomeworkError(err)
	}
	out := make([]classdayCompose.HomeworkEntry, 0, len(entries))
	for _, entry := range entries {
		out = append(out, classDayHomeworkEntry(entry))
	}
	return out, nil
}

func (b ClassDayHomework) HomeworkFeedbackOn(ctx context.Context, date timezone.Date, studentIDs []int64) ([]classdayCompose.HomeworkFeedbackEntry, error) {
	feedback, err := b.Book.HomeworkFeedbackOn(ctx, date, studentIDs)
	if err != nil {
		return nil, classDayHomeworkError(err)
	}
	out := make([]classdayCompose.HomeworkFeedbackEntry, 0, len(feedback))
	for _, entry := range feedback {
		out = append(out, classdayCompose.HomeworkFeedbackEntry{StudentID: entry.StudentID, Status: entry.Status, Note: entry.Note})
	}
	return out, nil
}

func (b ClassDayHomework) AddHomework(ctx context.Context, in classday.HomeworkWrite) (classdayCompose.HomeworkEntry, error) {
	entry, err := b.Book.AddHomework(ctx, timetable.HomeworkInput{
		SchoolClass: in.SchoolClass, Date: timezone.Date(in.Date), Subject: in.Subject, Description: in.Description, StaffID: in.StaffID,
	})
	if err != nil {
		return classdayCompose.HomeworkEntry{}, classDayHomeworkError(err)
	}
	return classDayHomeworkEntry(entry), nil
}

func (b ClassDayHomework) ChangeHomework(ctx context.Context, in classday.HomeworkChange) (classdayCompose.HomeworkEntry, error) {
	entry, err := b.Book.ChangeHomework(ctx, timetable.HomeworkChange{
		ID: in.ID, SchoolClass: in.SchoolClass, Subject: in.Subject, Description: in.Description, StaffID: in.StaffID,
	})
	if err != nil {
		return classdayCompose.HomeworkEntry{}, classDayHomeworkError(err)
	}
	return classDayHomeworkEntry(entry), nil
}

func (b ClassDayHomework) DeleteHomework(ctx context.Context, id int64, schoolClass string, staffID int64) error {
	return classDayHomeworkError(b.Book.DeleteHomework(ctx, timetable.HomeworkRef{ID: id, SchoolClass: schoolClass, StaffID: staffID}))
}

func classDayHomeworkEntry(entry timetable.HomeworkEntry) classdayCompose.HomeworkEntry {
	return classdayCompose.HomeworkEntry{ID: entry.ID, Subject: entry.Subject, Description: entry.Description, CreatedBy: entry.CreatedBy}
}

// classDayHomeworkError pairs Timetable's refusals with the projection's
// sentinels and keeps Timetable's error for the server log.
func classDayHomeworkError(err error) error {
	switch {
	case errors.Is(err, timetable.ErrInvalidHomework), errors.Is(err, timetable.ErrInvalidHomeworkFeedback):
		return fmt.Errorf("%w: %w", classday.ErrHomeworkInvalid, err)
	case errors.Is(err, timetable.ErrHomeworkNotFound):
		return fmt.Errorf("%w: %w", classday.ErrHomeworkNotFound, err)
	}
	return err
}
