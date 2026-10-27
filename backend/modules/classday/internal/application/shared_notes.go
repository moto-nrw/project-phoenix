package application

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/classday"
	"github.com/moto-nrw/project-phoenix/modules/classday/internal/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// SharedNoteReader reads the live notes of one child the OGS released for the
// school, newest first, with the authors' names.
type SharedNoteReader interface {
	ListSchoolSharedNotes(ctx context.Context, studentID int64) ([]peopledirectory.StudentNote, error)
}

// ClassTeacherAssignments reads the classes a staff member is assigned to in
// education.class_teachers. Unlike the caller's class reach it ignores
// class_day:all_classes: the shared notes go to the Lehrkraft of the class,
// never to the Schulverwaltung (#4797).
type ClassTeacherAssignments interface {
	ListSchoolClassesByStaff(ctx context.Context, staffID int64) ([]string, error)
}

// sharedNotes serves the notes the OGS released for the school (#4797). It
// decides who reads them — the staff member assigned to the child's class —
// and records every read; People Directory decides which notes are released.
type sharedNotes struct {
	notes       SharedNoteReader
	assignments ClassTeacherAssignments
	caller      ports.Caller
	students    SheetStudents
	accessLog   AccessLog
}

func newSharedNotes(deps ClassDayDependencies) sharedNotes {
	return sharedNotes{
		notes: deps.SharedNotes, assignments: deps.ClassTeachers, caller: deps.Caller,
		students: deps.Students, accessLog: deps.AccessLog,
	}
}

func (n sharedNotes) configured() bool {
	return n.notes != nil && n.assignments != nil && n.students != nil && n.accessLog != nil
}

// MayReadSharedNotes reports whether the caller is assigned to the class in
// education.class_teachers. An unconfigured seam and an account without a
// staff record read no shared notes.
func (n sharedNotes) MayReadSharedNotes(ctx context.Context, schoolClass string) (bool, error) {
	if !n.configured() || strings.TrimSpace(schoolClass) == "" {
		return false, nil
	}
	staffID, err := n.caller.CurrentStaffID(ctx)
	if errors.Is(err, classday.ErrStaffRecordRequired) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("shared notes: resolve staff: %w", err)
	}
	classes, err := n.assignments.ListSchoolClassesByStaff(ctx, staffID)
	if err != nil {
		return false, fmt.Errorf("shared notes: class assignments: %w", err)
	}
	_, err = resolveRequestedClass(schoolClass, classes)
	return err == nil, nil
}

// StudentSharedNotes returns the shared notes of one child of an assigned
// class and records the read in the GDPR access log, in the request's unit:
// a read whose record fails is no read.
func (n sharedNotes) StudentSharedNotes(ctx context.Context, in classday.SharedNotesRequest) (*classday.StudentSharedNotes, error) {
	if !n.configured() {
		return nil, classday.ErrSharedNotesNotConfigured
	}
	if in.StudentID <= 0 {
		return nil, classday.ErrSharedNoteStudentNotInClass
	}
	assigned, err := n.MayReadSharedNotes(ctx, in.SchoolClass)
	if err != nil {
		return nil, err
	}
	if !assigned {
		return nil, classday.ErrClassNotAssigned
	}
	student, err := n.students.FindSheetStudent(ctx, in.StudentID)
	if err != nil {
		return nil, fmt.Errorf("shared notes: load student: %w", err)
	}
	if student == nil || schoolstructure.NormalizeClass(student.SchoolClass) != schoolstructure.NormalizeClass(in.SchoolClass) {
		return nil, classday.ErrSharedNoteStudentNotInClass
	}
	view := &classday.StudentSharedNotes{StudentID: student.ID, SchoolClass: student.SchoolClass, Notes: []classday.SharedNote{}}
	persons, err := n.students.PersonsByID(ctx, []int64{student.PersonID})
	if err != nil {
		return nil, fmt.Errorf("shared notes: load person: %w", err)
	}
	if person, ok := persons[student.PersonID]; ok {
		view.FirstName, view.LastName = person.FirstName, person.LastName
	}
	notes, err := n.notes.ListSchoolSharedNotes(ctx, student.ID)
	if err != nil {
		return nil, fmt.Errorf("shared notes: list: %w", err)
	}
	for _, note := range notes {
		view.Notes = append(view.Notes, toSharedNote(note))
	}
	if err := n.record(ctx, view, in.Actor); err != nil {
		return nil, err
	}
	return view, nil
}

// record writes one access-log row per read: who, when, which child.
// Unlike the class day view it is never deduplicated — every opening of a
// child's notes is a disclosure of its own.
func (n sharedNotes) record(ctx context.Context, view *classday.StudentSharedNotes, actor classday.Actor) error {
	const errPrefix = "shared notes audit"
	now := time.Now()
	entry, err := newAccessRecord(errPrefix, actor.AccountID, actor.Roles, now, now, now)
	if err != nil {
		return err
	}
	studentID := view.StudentID
	entry.StudentID = &studentID
	entry.Metadata = map[string]any{
		"school_class": view.SchoolClass,
		"note_count":   len(view.Notes),
	}
	if err := n.accessLog.RecordSharedNotes(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}

func toSharedNote(note peopledirectory.StudentNote) classday.SharedNote {
	return classday.SharedNote{
		ID: note.ID, Kind: note.Kind, Category: note.Category, Body: note.Body,
		SubjectDate: note.Subject.Date, AuthorName: note.AuthorName, Edited: note.Edited(),
		CreatedAt: note.CreatedAt, UpdatedAt: note.UpdatedAt,
	}
}
