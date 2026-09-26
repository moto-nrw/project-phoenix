package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

type studentNoteRow struct {
	bun.BaseModel      `bun:"table:student_notes,alias:note"`
	ID                 int64          `bun:"id,pk,autoincrement"`
	TenantID           int64          `bun:"tenant_id,notnull"`
	StudentID          int64          `bun:"student_id,notnull"`
	AuthorAccountID    *int64         `bun:"author_account_id"`
	Origin             string         `bun:"origin,notnull"`
	Kind               string         `bun:"kind,notnull"`
	Visibility         string         `bun:"visibility,notnull"`
	Category           *string        `bun:"category"`
	Body               string         `bun:"body,notnull"`
	SubjectDate        *calendar.Date `bun:"subject_date,type:date"`
	ActivityGroupID    *int64         `bun:"activity_group_id"`
	EducationGroupID   *int64         `bun:"education_group_id"`
	DeletedAt          *time.Time     `bun:"deleted_at"`
	DeletedByAccountID *int64         `bun:"deleted_by_account_id"`
	CreatedAt          time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt          time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
}

const studentNotesTable = "users.student_notes"

// StudentNoteStore is the persistence port over users.student_notes. It shares
// the database runtime with the person and student stores.
type StudentNoteStore struct{ database Database }

func NewStudentNoteStore(database Database) *StudentNoteStore {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &StudentNoteStore{database: database}
}

func (s *StudentNoteStore) tenantDatabase(ctx context.Context) (bun.IDB, int64, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if tenantID <= 0 {
		return nil, 0, errors.New("people directory: student notes require a tenant")
	}
	return db, tenantID, nil
}

// List returns one child's visible notes, newest first. The audience predicate
// is built from the filter the caller resolved; this store never decides who
// may read what, it only turns the answer into SQL.
func (s *StudentNoteStore) List(ctx context.Context, filter domain.StudentNoteFilter) ([]domain.StudentNote, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []studentNoteRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(studentNotesTable+" AS note").
		Where(`"note".tenant_id = ?`, tenantID).
		Where(`"note".student_id = ?`, filter.StudentID).
		Where(`"note".deleted_at IS NULL`)
	if filter.Kind != "" {
		query = query.Where(`"note".kind = ?`, filter.Kind)
	}
	if filter.NoteID > 0 {
		query = query.Where(`"note".id = ?`, filter.NoteID)
	}
	query = applyAudience(query, filter)
	// created_at then id: two notes written in the same transaction share a
	// timestamp, and a timeline that reorders itself between two reads is a
	// bug report waiting to happen.
	query = query.OrderExpr(`"note".created_at DESC, "note".id DESC`)

	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list student notes: %w", err)
	}
	stats.Rows = int64(len(rows))
	notes := make([]domain.StudentNote, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, row.toDomain())
	}
	return notes, stats, nil
}

// applyAudience adds the OR of the ways a note can reach this reader as one
// bracketed group, so it cannot combine with the tenant and child predicates
// into something wider than intended.
//
// A reader in no audience at all still sees their own notes. The group is
// never empty: a caller with no account and no audience is not a reader this
// store serves, and an empty group would silently produce "this child has no
// notes" instead of an error.
func applyAudience(query *bun.SelectQuery, filter domain.StudentNoteFilter) *bun.SelectQuery {
	return query.WhereGroup(" AND ", func(group *bun.SelectQuery) *bun.SelectQuery {
		if len(filter.Visibilities) > 0 {
			group = group.WhereOr(`"note".visibility IN (?)`, bun.List(filter.Visibilities))
		}
		if filter.ReaderAccountID > 0 {
			group = group.WhereOr(`"note".author_account_id = ?`, filter.ReaderAccountID)
		}
		if len(filter.LedActivityGroupIDs) > 0 {
			group = group.WhereOr(
				`("note".visibility = ? AND "note".activity_group_id IN (?))`,
				domain.StudentNoteVisibilityGroupLeads, bun.List(filter.LedActivityGroupIDs),
			)
		}
		if len(filter.LedEducationGroupIDs) > 0 {
			group = group.WhereOr(
				`("note".visibility = ? AND "note".education_group_id IN (?))`,
				domain.StudentNoteVisibilityGroupLeads, bun.List(filter.LedEducationGroupIDs),
			)
		}
		return group
	})
}

// FindByID reads one note of the current tenant. lock is passed through to
// SELECT ... FOR <lock> so a writer can re-check authorship under the same
// lock it updates with.
func (s *StudentNoteStore) FindByID(ctx context.Context, id int64, lock string) (domain.StudentNote, bool, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.StudentNote{}, false, domain.OperationStats{}, err
	}
	var row studentNoteRow
	query := db.NewSelect().
		Model(&row).
		ModelTableExpr(studentNotesTable+" AS note").
		Where(`"note".tenant_id = ?`, tenantID).
		Where(`"note".id = ?`, id).
		Where(`"note".deleted_at IS NULL`)
	if lock != "" {
		query = query.For(lock)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StudentNote{}, false, stats, nil
	}
	if err != nil {
		return domain.StudentNote{}, false, stats, fmt.Errorf("find student note: %w", err)
	}
	stats.Rows = 1
	return row.toDomain(), true, stats, nil
}

// Insert writes one new note and returns it as stored, so the caller renders
// the server's timestamps rather than guessing them.
func (s *StudentNoteStore) Insert(ctx context.Context, create domain.CreateStudentNote) (domain.StudentNote, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.StudentNote{}, domain.OperationStats{}, err
	}
	author := create.AuthorAccountID
	row := studentNoteRow{
		TenantID: tenantID, StudentID: create.StudentID, AuthorAccountID: &author,
		Origin: domain.StudentNoteOriginStaff, Kind: create.Kind, Visibility: create.Visibility,
		Category: optionalText(create.Category), Body: create.Body,
		SubjectDate:      create.Subject.Date,
		ActivityGroupID:  create.Subject.ActivityGroupID,
		EducationGroupID: create.Subject.EducationGroupID,
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = db.NewInsert().
		Model(&row).
		ModelTableExpr(studentNotesTable).
		Returning("*").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return domain.StudentNote{}, stats, fmt.Errorf("insert student note: %w", err)
	}
	stats.Rows = 1
	return row.toDomain(), stats, nil
}

// Update rewrites the editable columns of one note. The subject stays as
// stored: an entry that moves to another day or another activity is a
// different entry, not a correction of this one.
func (s *StudentNoteStore) Update(ctx context.Context, update domain.UpdateStudentNote) (domain.StudentNote, domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.StudentNote{}, domain.OperationStats{}, err
	}
	var row studentNoteRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewUpdate().
		Model(&row).
		ModelTableExpr(studentNotesTable+" AS note").
		Set("kind = ?", update.Kind).
		Set("visibility = ?", update.Visibility).
		Set("category = ?", optionalText(update.Category)).
		Set("body = ?", update.Body).
		Where(`"note".tenant_id = ?`, tenantID).
		Where(`"note".id = ?`, update.ID).
		Where(`"note".deleted_at IS NULL`).
		Returning("*").
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.StudentNote{}, stats, domain.ErrStudentNoteNotFound
	}
	if err != nil {
		return domain.StudentNote{}, stats, fmt.Errorf("update student note: %w", err)
	}
	stats.Rows = 1
	return row.toDomain(), stats, nil
}

// SoftDelete hides one note and records who hid it. The row stays: a note
// about a child is documentation, and a deletion that leaves no trace is
// indistinguishable from a note that was never written.
func (s *StudentNoteStore) SoftDelete(ctx context.Context, deletion domain.DeleteStudentNote) (domain.OperationStats, error) {
	db, tenantID, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().
		Model((*studentNoteRow)(nil)).
		ModelTableExpr(studentNotesTable+" AS note").
		Set("deleted_at = clock_timestamp()").
		Set("deleted_by_account_id = ?", deletion.ActorAccountID).
		Where(`"note".tenant_id = ?`, tenantID).
		Where(`"note".id = ?`, deletion.ID).
		Where(`"note".deleted_at IS NULL`).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("delete student note: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil {
		stats.Rows = affected
		if affected == 0 {
			return stats, domain.ErrStudentNoteNotFound
		}
	}
	return stats, nil
}

func (r studentNoteRow) toDomain() domain.StudentNote {
	note := domain.StudentNote{
		ID: r.ID, TenantID: r.TenantID, StudentID: r.StudentID,
		AuthorAccountID: r.AuthorAccountID, Origin: r.Origin, Kind: r.Kind,
		Visibility: r.Visibility, Body: r.Body,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		Subject: domain.StudentNoteSubject{
			ActivityGroupID:  r.ActivityGroupID,
			EducationGroupID: r.EducationGroupID,
		},
	}
	if r.Category != nil {
		note.Category = *r.Category
	}
	note.Subject.Date = r.SubjectDate
	return note
}

func optionalText(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
