package application

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentNoteService serves the note card of one child: the durable hints that
// belong with the master data and the dated entries of the chronicle.
//
// It decides what a valid note is, that a note belongs to the child the caller
// named, and that only the author may reword one. It deliberately does NOT
// decide who may read or remove a note: that needs the reader's groups,
// activities and school classes, which belong to other owners, so the caller
// resolves it and hands the answer down as a filter.
type StudentNoteService struct {
	store ports.StudentNoteStore
	// persons resolves the authors' names in the same unit of work as the
	// notes. A timeline without names is unreadable, and leaving the lookup to
	// the caller means every caller repeats it.
	persons ports.Store
	// students is the lifecycle gate: a note is written about a child that is
	// still there, so the row is locked and checked before the insert. Mirrors
	// how the family-protection ledger refuses an entry for a graduate.
	students ports.StudentStore
	tx       ports.Transaction
	observe  ports.Observer
}

func NewStudentNotes(
	store ports.StudentNoteStore,
	persons ports.Store,
	students ports.StudentStore,
	tx ports.Transaction,
	observe ports.Observer,
) *StudentNoteService {
	if store == nil || persons == nil || students == nil || tx == nil || observe == nil {
		panic("people directory application: all student note dependencies are required")
	}
	return &StudentNoteService{store: store, persons: persons, students: students, tx: tx, observe: observe}
}

// List returns the notes of one child that the resolved audience may read,
// newest first, with each author's name resolved. A filter naming one note
// returns that note or nothing — the audience predicate is the same either way,
// so a caller cannot read a note past its reach by naming its ID.
func (s *StudentNoteService) List(ctx context.Context, filter domain.StudentNoteFilter) (result []domain.StudentNote, err error) {
	if filter.StudentID <= 0 {
		return nil, domain.ErrStudentNoteInvalid
	}
	err = s.run(ctx, "list_student_notes", s.tx.RunRead, func(txCtx context.Context, stats *domain.OperationStats) error {
		notes, queryStats, listErr := s.store.List(txCtx, filter)
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		persons, personStats, personErr := s.persons.ListByAccounts(txCtx, noteAuthorAccountIDs(notes))
		stats.Add(personStats)
		if personErr != nil {
			return personErr
		}
		result = withAuthorNames(notes, persons)
		return nil
	})
	return result, err
}

// Create writes one note about a child that is still enrolled, and returns it
// as stored. The student row is locked first: a note written into the same
// moment a child is graduated away would otherwise survive in a card nobody
// opens again.
func (s *StudentNoteService) Create(ctx context.Context, input domain.CreateStudentNote) (result domain.StudentNote, err error) {
	input.Body = strings.TrimSpace(input.Body)
	if validationErr := input.Validate(); validationErr != nil {
		return domain.StudentNote{}, validationErr
	}
	err = s.run(ctx, "create_student_note", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		status, found, lockStats, lockErr := s.students.LockLifecycle(txCtx, input.StudentID)
		stats.Add(lockStats)
		if lockErr != nil {
			return lockErr
		}
		if !found || status == domain.StudentStatusAlumnus {
			return domain.ErrStudentNotFound
		}
		var writeStats domain.OperationStats
		result, writeStats, err = s.store.Insert(txCtx, input)
		stats.Add(writeStats)
		return err
	})
	return result, err
}

// Update corrects one note. Only its author may: a Klassenbuch entry is a
// statement by a person, and letting a colleague rewrite it under the original
// name would make the authorship a lie. The stored row is locked first, so two
// concurrent edits cannot both pass the authorship check against a row the
// other one is replacing.
func (s *StudentNoteService) Update(ctx context.Context, input domain.UpdateStudentNote) (result domain.StudentNote, err error) {
	input.Body = strings.TrimSpace(input.Body)
	if validationErr := input.Validate(); validationErr != nil {
		return domain.StudentNote{}, validationErr
	}
	err = s.run(ctx, "update_student_note", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		stored, readErr := s.lockNote(txCtx, stats, input.ID, input.StudentID)
		if readErr != nil {
			return readErr
		}
		if allowErr := stored.AllowsUpdate(input); allowErr != nil {
			return allowErr
		}
		var writeStats domain.OperationStats
		result, writeStats, err = s.store.Update(txCtx, input)
		stats.Add(writeStats)
		return err
	})
	return result, err
}

// Delete hides one note. Who may ask is the caller's decision — the school's
// leadership, not the author — so this service only re-checks that the note is
// the one the caller authorized against and records who removed it.
func (s *StudentNoteService) Delete(ctx context.Context, input domain.DeleteStudentNote) error {
	if input.ID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return domain.ErrStudentNoteInvalid
	}
	return s.run(ctx, "delete_student_note", s.tx.RunWrite, func(txCtx context.Context, stats *domain.OperationStats) error {
		stored, found, readStats, readErr := s.store.FindByID(txCtx, input.ID, "UPDATE")
		stats.Add(readStats)
		if readErr != nil {
			return readErr
		}
		if !found || stored.StudentID != input.StudentID {
			return domain.ErrStudentNoteNotFound
		}
		deleteStats, err := s.store.SoftDelete(txCtx, input)
		stats.Add(deleteStats)
		return err
	})
}

// lockNote reads one note FOR UPDATE and confirms it belongs to the child the
// caller named. A note of another child behind a valid ID is a not-found: the
// caller authorized against a child, and must not be able to reach past it.
func (s *StudentNoteService) lockNote(
	ctx context.Context, stats *domain.OperationStats, noteID, studentID int64,
) (domain.StudentNote, error) {
	stored, found, readStats, err := s.store.FindByID(ctx, noteID, "UPDATE")
	stats.Add(readStats)
	if err != nil {
		return domain.StudentNote{}, err
	}
	if !found || (studentID > 0 && stored.StudentID != studentID) {
		return domain.StudentNote{}, domain.ErrStudentNoteNotFound
	}
	return stored, nil
}

// noteAuthorAccountIDs collects the distinct authors of the notes. The hints
// carried over from the master-data field have none and are skipped.
func noteAuthorAccountIDs(notes []domain.StudentNote) []int64 {
	seen := make(map[int64]bool, len(notes))
	ids := make([]int64, 0, len(notes))
	for _, note := range notes {
		if note.AuthorAccountID == nil || seen[*note.AuthorAccountID] {
			continue
		}
		seen[*note.AuthorAccountID] = true
		ids = append(ids, *note.AuthorAccountID)
	}
	return ids
}

// withAuthorNames fills in the display name of each author. An author whose
// person row is gone keeps an empty name rather than losing the note.
func withAuthorNames(notes []domain.StudentNote, persons []domain.Person) []domain.StudentNote {
	if len(notes) == 0 {
		return notes
	}
	names := make(map[int64]string, len(persons))
	for _, person := range persons {
		if person.AccountID != nil {
			names[*person.AccountID] = strings.TrimSpace(person.FirstName + " " + person.LastName)
		}
	}
	for index := range notes {
		if notes[index].AuthorAccountID != nil {
			notes[index].AuthorName = names[*notes[index].AuthorAccountID]
		}
	}
	return notes
}

func (s *StudentNoteService) run(
	ctx context.Context,
	operation string,
	run func(context.Context, func(context.Context) error) error,
	fn func(context.Context, *domain.OperationStats) error,
) error {
	return observeRun(ctx, s.observe, operation, run, fn)
}
