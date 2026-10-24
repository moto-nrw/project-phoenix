package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Lehrkraft's day hint for the OGS (#4795) is a dated note with origin
// school. This owner decides what a valid hint is, that one author keeps one
// live hint per child, day and target, and that every removal is recorded.
// Who may write one, for which class, and on which day is the school portal's
// decision; it hands down the children the hint is for. The capability runs
// the work over the note service's store, so there is one layer between the
// contract and the rows. Every caller (the school portal routes and the OGS
// views) already holds the request's school unit; the hints join it and never
// open one of their own.

// ListSchoolDayHints returns the day hints of one day for the named children,
// oldest first, with each author's name resolved.
func (s *Service) ListSchoolDayHints(ctx context.Context, date calendar.Date, studentIDs []int64) ([]peopledirectory.SchoolDayHint, error) {
	if date.IsZero() {
		return nil, peopledirectory.ErrStudentNoteInvalid
	}
	if len(studentIDs) == 0 {
		return []peopledirectory.SchoolDayHint{}, nil
	}
	notes := s.studentNotes
	var hints []domain.StudentNote
	err := notes.run(ctx, "list_school_day_hints", joinSuppliedUnit(notes.tx), func(txCtx context.Context, stats *domain.OperationStats) error {
		rows, queryStats, listErr := notes.store.ListSchoolDayHints(txCtx, domain.SchoolDayHintFilter{Date: date, StudentIDs: studentIDs})
		stats.Add(queryStats)
		if listErr != nil {
			return listErr
		}
		persons, personStats, personErr := notes.persons.ListByAccounts(txCtx, noteAuthorAccountIDs(rows))
		stats.Add(personStats)
		if personErr != nil {
			return personErr
		}
		hints = withAuthorNames(rows, persons)
		return nil
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]peopledirectory.SchoolDayHint, 0, len(hints))
	for _, hint := range hints {
		result = append(result, toPublicSchoolDayHint(hint))
	}
	return result, nil
}

// SetSchoolDayHint writes or rewords one day hint. Each child is locked
// first, like every note: a hint written into the moment a child graduates
// would otherwise survive in a card nobody opens again. A class hint skips a
// child that left; a hint for one child that left is a not-found. A class
// hint that no longer reaches a child of the class (it changed class since
// the first write) is withdrawn from that child.
func (s *Service) SetSchoolDayHint(ctx context.Context, input peopledirectory.SetSchoolDayHint) error {
	hint := domain.SetSchoolDayHint{
		AuthorAccountID: input.AuthorAccountID, Date: input.Date,
		SchoolClass: strings.TrimSpace(input.SchoolClass), StudentIDs: input.StudentIDs,
		Body: strings.TrimSpace(input.Body),
	}
	if err := hint.Validate(); err != nil {
		return MapError(err)
	}
	notes := s.studentNotes
	return MapError(notes.run(ctx, "set_school_day_hint", joinSuppliedUnit(notes.tx), func(txCtx context.Context, stats *domain.OperationStats) error {
		enrolled, err := notes.lockEnrolledStudents(txCtx, stats, hint.StudentIDs)
		if err != nil {
			return err
		}
		if hint.SchoolClass == "" && len(enrolled) == 0 {
			return domain.ErrStudentNotFound
		}
		if hint.SchoolClass != "" {
			target := domain.ClearSchoolDayHint{
				AuthorAccountID: hint.AuthorAccountID, Date: hint.Date, SchoolClass: hint.SchoolClass,
			}
			if _, err := notes.retireSchoolDayHints(txCtx, stats, target, enrolled); err != nil {
				return err
			}
		}
		if len(enrolled) == 0 {
			return nil
		}
		hint.StudentIDs = enrolled
		upsertStats, err := notes.store.UpsertSchoolDayHints(txCtx, hint)
		stats.Add(upsertStats)
		return err
	}))
}

// ClearSchoolDayHint withdraws one of the author's day hints. Nothing to
// withdraw is a not-found, so a stale screen learns the hint is gone.
func (s *Service) ClearSchoolDayHint(ctx context.Context, input peopledirectory.ClearSchoolDayHint) error {
	target := domain.ClearSchoolDayHint{
		AuthorAccountID: input.AuthorAccountID, Date: input.Date,
		SchoolClass: strings.TrimSpace(input.SchoolClass), StudentID: input.StudentID,
	}
	if err := target.Validate(); err != nil {
		return MapError(err)
	}
	notes := s.studentNotes
	return MapError(notes.run(ctx, "clear_school_day_hint", joinSuppliedUnit(notes.tx), func(txCtx context.Context, stats *domain.OperationStats) error {
		retired, err := notes.retireSchoolDayHints(txCtx, stats, target, nil)
		if err != nil {
			return err
		}
		if retired == 0 {
			return domain.ErrStudentNoteNotFound
		}
		return nil
	}))
}

// joinSuppliedUnit runs fn in the caller's school unit and refuses to run
// without one: a hint written outside the request's transaction would commit
// apart from the checks the school portal made before it.
func joinSuppliedUnit(tx ports.Transaction) func(context.Context, func(context.Context) error) error {
	return func(ctx context.Context, fn func(context.Context) error) error {
		if err := tx.RequireTransaction(ctx); err != nil {
			return err
		}
		return fn(ctx)
	}
}

// lockEnrolledStudents locks each child's lifecycle and returns the children
// that are still enrolled, in the order given.
func (s *StudentNoteService) lockEnrolledStudents(
	ctx context.Context, stats *domain.OperationStats, studentIDs []int64,
) ([]int64, error) {
	enrolled := make([]int64, 0, len(studentIDs))
	seen := make(map[int64]bool, len(studentIDs))
	for _, studentID := range studentIDs {
		if seen[studentID] {
			continue
		}
		seen[studentID] = true
		status, found, lockStats, err := s.students.LockLifecycle(ctx, studentID)
		stats.Add(lockStats)
		if err != nil {
			return nil, err
		}
		if found && status != domain.StudentStatusAlumnus {
			enrolled = append(enrolled, studentID)
		}
	}
	return enrolled, nil
}

// retireSchoolDayHints hides the matching hints and records each removal in
// the deletion ledger, as every removed note is.
func (s *StudentNoteService) retireSchoolDayHints(
	ctx context.Context, stats *domain.OperationStats, target domain.ClearSchoolDayHint, keep []int64,
) (int, error) {
	retired, retireStats, err := s.store.RetireSchoolDayHints(ctx, target, keep)
	stats.Add(retireStats)
	if err != nil {
		return 0, err
	}
	if len(retired) > 0 && s.audit == nil {
		return 0, ports.ErrStudentNoteDeletionAuditUnavailable
	}
	for _, note := range retired {
		if err := s.audit.RecordStudentNoteDeletion(ctx, note.StudentID, note.ID, target.AuthorAccountID); err != nil {
			return 0, fmt.Errorf("record school day hint deletion: %w", err)
		}
	}
	return len(retired), nil
}
