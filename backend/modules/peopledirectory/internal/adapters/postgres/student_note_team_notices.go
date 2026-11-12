package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const studentNoteTeamNoticesTable = "users.student_note_team_notices"

type studentNoteTeamNoticeRow struct {
	NoteID          int64     `bun:"note_id"`
	StudentID       int64     `bun:"student_id"`
	AuthorAccountID *int64    `bun:"author_account_id"`
	InformedAt      time.Time `bun:"informed_at"`
}

// UpsertTeamNotices marks one note unread for the recipients in one
// statement. A recipient told about the note before is told again: the
// author corrected it and wants it read once more.
func (s *StudentNoteStore) UpsertTeamNotices(ctx context.Context, noteID int64, recipientAccountIDs []int64) (int, domain.OperationStats, error) {
	if len(recipientAccountIDs) == 0 {
		return 0, domain.OperationStats{}, nil
	}
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewRaw(`
		INSERT INTO `+studentNoteTeamNoticesTable+` (tenant_id, note_id, recipient_account_id, informed_at, read_at)
		SELECT ?, ?, recipient, clock_timestamp(), NULL
		FROM unnest(?::bigint[]) AS recipient
		ON CONFLICT (note_id, recipient_account_id)
		DO UPDATE SET informed_at = EXCLUDED.informed_at, read_at = NULL`,
		scope.TenantID, noteID, pgdialect.Array(recipientAccountIDs)).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("store student note team notices: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("store student note team notices: %w", err)
	}
	stats.Rows = affected
	return int(affected), stats, nil
}

// ListUnreadTeamNotices reads the reader's unread markers. The note decides
// whether a marker still points anywhere: a deleted note and one narrowed
// below the whole team drop out here rather than through a second write on
// every correction. Departed children are excluded before the result limit.
func (s *StudentNoteStore) ListUnreadTeamNotices(ctx context.Context, readerAccountID int64, limit int) ([]domain.StudentNoteTeamNotice, domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []studentNoteTeamNoticeRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewRaw(`
		SELECT notice.note_id, note.student_id, note.author_account_id, notice.informed_at
		FROM `+studentNoteTeamNoticesTable+` AS notice
		JOIN `+studentNotesTable+` AS note ON note.id = notice.note_id AND note.tenant_id = notice.tenant_id
		WHERE notice.tenant_id = ?
			AND notice.recipient_account_id = ?
			AND notice.read_at IS NULL
			AND note.deleted_at IS NULL
			AND note.visibility = ?
			AND note.student_id IN (?)
		ORDER BY notice.informed_at DESC, notice.note_id DESC
		LIMIT ?`,
		scope.TenantID, readerAccountID, domain.StudentNoteVisibilityAllStaff,
		s.students.StudentRows(db, scope.TenantID).ColumnExpr(`"student".id`).
			Where(`"student".status <> ?`, domain.StudentStatusAlumnus), limit).
		Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list student note team notices: %w", err)
	}
	stats.Rows = int64(len(rows))
	notices := make([]domain.StudentNoteTeamNotice, 0, len(rows))
	for _, row := range rows {
		notices = append(notices, domain.StudentNoteTeamNotice{
			NoteID: row.NoteID, StudentID: row.StudentID, AuthorAccountID: row.AuthorAccountID, InformedAt: row.InformedAt,
		})
	}
	return notices, stats, nil
}

// MarkTeamNoticesRead clears the reader's markers on every note of the child.
func (s *StudentNoteStore) MarkTeamNoticesRead(ctx context.Context, readerAccountID, studentID int64) (domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewRaw(`
		UPDATE `+studentNoteTeamNoticesTable+` AS notice
		SET read_at = clock_timestamp()
		FROM `+studentNotesTable+` AS note
		WHERE notice.tenant_id = ?
			AND notice.recipient_account_id = ?
			AND notice.read_at IS NULL
			AND note.id = notice.note_id
			AND note.tenant_id = notice.tenant_id
			AND note.student_id = ?`,
		scope.TenantID, readerAccountID, studentID).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("mark student note team notices read: %w", err)
	}
	if stats.Rows, err = result.RowsAffected(); err != nil {
		return stats, fmt.Errorf("mark student note team notices read: %w", err)
	}
	return stats, nil
}
