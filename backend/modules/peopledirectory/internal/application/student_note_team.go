package application

import (
	"context"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/ports"
)

// StudentNoteTeamService serves "Speichern und Team informieren" (#6854): the
// author tells the colleagues about a note, each of them gets an unread
// marker, and opening the child's notes clears it.
//
// It decides that only the author tells the team, only about a note the
// whole team may read, and that a marker follows its note. It does not decide
// who the team is or how they hear about it on their devices: the caller
// resolves the recipients and hands the device notification in as a callback
// that joins this transaction.
type StudentNoteTeamService struct {
	notes    ports.StudentNoteStore
	team     ports.StudentNoteTeamStore
	students ports.StudentStore
	persons  ports.Store
	tx       ports.Transaction
	observe  ports.Observer
}

var _ peopledirectory.StudentNoteTeamNotices = (*StudentNoteTeamService)(nil)

func NewStudentNoteTeam(
	notes ports.StudentNoteStore,
	team ports.StudentNoteTeamStore,
	students ports.StudentStore,
	persons ports.Store,
	tx ports.Transaction,
	observe ports.Observer,
) *StudentNoteTeamService {
	if notes == nil || team == nil || students == nil || persons == nil || tx == nil || observe == nil {
		panic("people directory application: all student note team dependencies are required")
	}
	return &StudentNoteTeamService{notes: notes, team: team, students: students, persons: persons, tx: tx, observe: observe}
}

// InformStudentNoteTeam stores the markers and runs the device notification
// in a savepoint of the caller's unit: a failure of either leaves neither
// behind, whatever the caller answers. The child row is locked before the
// note, in the order the deletion takes them, so a note cannot be announced
// while it is removed.
func (s *StudentNoteTeamService) InformStudentNoteTeam(ctx context.Context, input peopledirectory.InformStudentNoteTeam) (informed int, err error) {
	if input.NoteID <= 0 || input.StudentID <= 0 || input.ActorAccountID <= 0 {
		return 0, peopledirectory.ErrStudentNoteInvalid
	}
	command := domain.InformStudentNoteTeam{
		NoteID: input.NoteID, StudentID: input.StudentID, ActorAccountID: input.ActorAccountID,
		RecipientAccountIDs: input.RecipientAccountIDs, Notify: input.Notify,
	}
	err = observeRun(ctx, s.observe, "inform_student_note_team", s.inCallerSavepoint, func(txCtx context.Context, stats *domain.OperationStats) error {
		note, lockErr := s.lockTeamNote(txCtx, stats, command)
		if lockErr != nil {
			return lockErr
		}
		if allowErr := note.AllowsTeamNotice(command.ActorAccountID); allowErr != nil {
			return allowErr
		}
		var tellErr error
		informed, tellErr = s.tellTeam(txCtx, stats, note.ID, command)
		return tellErr
	})
	if err != nil {
		return 0, MapError(err)
	}
	return informed, nil
}

// tellTeam stores the markers of the colleagues and runs the device
// notification in the same unit; its error rolls the markers back.
func (s *StudentNoteTeamService) tellTeam(
	ctx context.Context, stats *domain.OperationStats, noteID int64, command domain.InformStudentNoteTeam,
) (int, error) {
	recipients := command.Recipients()
	if len(recipients) == 0 {
		return 0, nil
	}
	count, writeStats, err := s.team.UpsertTeamNotices(ctx, noteID, recipients)
	stats.Add(writeStats)
	if err != nil {
		return 0, err
	}
	if command.Notify != nil {
		if err := command.Notify(ctx, recipients); err != nil {
			return 0, err
		}
	}
	return count, nil
}

// inCallerUnit runs fn in the caller's school unit; the routes and the
// workflows that reach this capability already hold one.
func (s *StudentNoteTeamService) inCallerUnit(ctx context.Context, fn func(context.Context) error) error {
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return err
	}
	return fn(ctx)
}

// inCallerSavepoint is inCallerUnit inside a savepoint, so a failed write
// rolls back only its own rows.
func (s *StudentNoteTeamService) inCallerSavepoint(ctx context.Context, fn func(context.Context) error) error {
	if err := s.tx.RequireTransaction(ctx); err != nil {
		return err
	}
	return s.tx.RunSavepoint(ctx, fn)
}

func (s *StudentNoteTeamService) lockTeamNote(
	ctx context.Context, stats *domain.OperationStats, command domain.InformStudentNoteTeam,
) (domain.StudentNote, error) {
	status, found, lockStats, err := s.students.LockLifecycle(ctx, command.StudentID)
	stats.Add(lockStats)
	if err != nil {
		return domain.StudentNote{}, err
	}
	if !found || status == domain.StudentStatusAlumnus {
		return domain.StudentNote{}, domain.ErrStudentNoteNotFound
	}
	note, found, readStats, err := s.notes.FindByID(ctx, command.NoteID, "UPDATE")
	stats.Add(readStats)
	if err != nil {
		return domain.StudentNote{}, err
	}
	if !found || note.StudentID != command.StudentID {
		return domain.StudentNote{}, domain.ErrStudentNoteNotFound
	}
	return note, nil
}

// ListUnreadStudentNoteTeamNotices returns the reader's unread markers with
// the child's and the author's names, in four statements whatever their
// number.
func (s *StudentNoteTeamService) ListUnreadStudentNoteTeamNotices(ctx context.Context, readerAccountID int64) (result []peopledirectory.StudentNoteTeamNotice, err error) {
	if readerAccountID <= 0 {
		return nil, peopledirectory.ErrStudentNoteInvalid
	}
	err = observeRun(ctx, s.observe, "list_student_note_team_notices", s.inCallerUnit, func(txCtx context.Context, stats *domain.OperationStats) error {
		var listErr error
		result, listErr = s.unread(txCtx, stats, readerAccountID)
		return listErr
	})
	if err != nil {
		return nil, MapError(err)
	}
	return result, nil
}

// MarkStudentNoteTeamNoticesRead clears the observed announcement revisions
// on the child's notes and returns those still unread.
func (s *StudentNoteTeamService) MarkStudentNoteTeamNoticesRead(ctx context.Context, readerAccountID, studentID int64, observed []peopledirectory.StudentNoteTeamNoticeRevision) (result []peopledirectory.StudentNoteTeamNotice, err error) {
	if readerAccountID <= 0 || studentID <= 0 {
		return nil, peopledirectory.ErrStudentNoteInvalid
	}
	revisions := make([]domain.StudentNoteTeamNoticeRevision, 0, len(observed))
	for _, notice := range observed {
		if notice.NoteID <= 0 || notice.InformedAt.IsZero() {
			return nil, peopledirectory.ErrStudentNoteInvalid
		}
		revisions = append(revisions, domain.StudentNoteTeamNoticeRevision{NoteID: notice.NoteID, InformedAt: notice.InformedAt})
	}
	err = observeRun(ctx, s.observe, "mark_student_note_team_notices_read", s.inCallerUnit, func(txCtx context.Context, stats *domain.OperationStats) error {
		markStats, markErr := s.team.MarkTeamNoticesRead(txCtx, readerAccountID, studentID, revisions)
		stats.Add(markStats)
		if markErr != nil {
			return markErr
		}
		var listErr error
		result, listErr = s.unread(txCtx, stats, readerAccountID)
		return listErr
	})
	if err != nil {
		return nil, MapError(err)
	}
	return result, nil
}

func (s *StudentNoteTeamService) unread(
	ctx context.Context, stats *domain.OperationStats, readerAccountID int64,
) ([]peopledirectory.StudentNoteTeamNotice, error) {
	notices, listStats, err := s.team.ListUnreadTeamNotices(ctx, readerAccountID, domain.MaxStudentNoteTeamNotices)
	stats.Add(listStats)
	if err != nil || len(notices) == 0 {
		return []peopledirectory.StudentNoteTeamNotice{}, err
	}
	studentIDs, authorIDs := noticeReferences(notices)
	students, studentStats, err := s.students.ListByIDs(ctx, studentIDs)
	stats.Add(studentStats)
	if err != nil {
		return nil, err
	}
	names, nameStats, err := s.students.ListNamesByIDs(ctx, enrolledStudentIDs(students))
	stats.Add(nameStats)
	if err != nil {
		return nil, err
	}
	authors, authorStats, err := s.persons.ListByAccounts(ctx, authorIDs)
	stats.Add(authorStats)
	if err != nil {
		return nil, err
	}
	return withNoticeNames(notices, names, authors), nil
}

// enrolledStudentIDs keeps the children that have not left: a marker about a
// graduate points to a card nobody opens again.
func enrolledStudentIDs(students []peopledirectory.Student) []int64 {
	ids := make([]int64, 0, len(students))
	for _, student := range students {
		if !student.IsAlumnus() {
			ids = append(ids, student.ID)
		}
	}
	return ids
}

func noticeReferences(notices []domain.StudentNoteTeamNotice) (students, authors []int64) {
	seenStudents, seenAuthors := map[int64]bool{}, map[int64]bool{}
	for _, notice := range notices {
		if !seenStudents[notice.StudentID] {
			seenStudents[notice.StudentID] = true
			students = append(students, notice.StudentID)
		}
		if notice.AuthorAccountID != nil && !seenAuthors[*notice.AuthorAccountID] {
			seenAuthors[*notice.AuthorAccountID] = true
			authors = append(authors, *notice.AuthorAccountID)
		}
	}
	return students, authors
}

func withNoticeNames(
	notices []domain.StudentNoteTeamNotice, names []peopledirectory.StudentName, authors []peopledirectory.Person,
) []peopledirectory.StudentNoteTeamNotice {
	children := make(map[int64]peopledirectory.StudentName, len(names))
	for _, name := range names {
		children[name.StudentID] = name
	}
	authorNames := make(map[int64]string, len(authors))
	for _, person := range authors {
		if person.AccountID != nil {
			authorNames[*person.AccountID] = strings.TrimSpace(person.FirstName + " " + person.LastName)
		}
	}
	out := make([]peopledirectory.StudentNoteTeamNotice, 0, len(notices))
	for _, notice := range notices {
		child, ok := children[notice.StudentID]
		if !ok {
			continue // the child left or its person row is gone; nothing to link to
		}
		entry := peopledirectory.StudentNoteTeamNotice{
			NoteID: notice.NoteID, StudentID: notice.StudentID,
			StudentFirstName: child.FirstName, StudentLastName: child.LastName,
			InformedAt: notice.InformedAt,
		}
		if notice.AuthorAccountID != nil {
			entry.AuthorName = authorNames[*notice.AuthorAccountID]
		}
		out = append(out, entry)
	}
	return out
}
