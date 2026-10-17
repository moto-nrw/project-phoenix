package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
)

// StudentNoteStore is the persistence port over users.student_notes. Every
// read is audience-filtered by the caller-resolved domain.StudentNoteFilter;
// the store never widens it.
type StudentNoteStore interface {
	// List serves both the timeline and the single-note read: domain.StudentNoteFilter
	// carries an optional NoteID, so a caller that names one note does not
	// need a second store method with its own audience rules.
	List(context.Context, domain.StudentNoteFilter) ([]domain.StudentNote, domain.OperationStats, error)
	// FindByID reads one note regardless of audience — the writers use it to
	// re-check authorship and ownership under the lock they write with.
	FindByID(ctx context.Context, id int64, lock string) (domain.StudentNote, bool, domain.OperationStats, error)
	Insert(context.Context, domain.CreateStudentNote) (domain.StudentNote, domain.OperationStats, error)
	Update(context.Context, domain.UpdateStudentNote) (domain.StudentNote, domain.OperationStats, error)
	SoftDelete(context.Context, domain.DeleteStudentNote) (domain.OperationStats, error)
	// ImportPermanentHint writes an imported „Betreuernotizen" text as the
	// child's carried-over permanent hint: it replaces the live one, or
	// creates one unless a school deliberately deleted it.
	ImportPermanentHint(ctx context.Context, studentID int64, body string) (domain.OperationStats, error)
}
