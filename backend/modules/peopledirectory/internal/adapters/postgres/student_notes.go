package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	SchoolClass        *string        `bun:"school_class"`
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

func (s *StudentNoteStore) tenantDatabase(ctx context.Context) (bun.IDB, TenantScope, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, TenantScope{}, err
	}
	if scope.TenantID <= 0 {
		return nil, TenantScope{}, errors.New("people directory: student notes require a tenant")
	}
	return db, scope, nil
}

// List returns one child's visible notes, newest first. The audience predicate
// is built from the filter the caller resolved; this store never decides who
// may read what, it only turns the answer into SQL.
func (s *StudentNoteStore) List(ctx context.Context, filter domain.StudentNoteFilter) ([]domain.StudentNote, domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []studentNoteRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(studentNotesTable+" AS note").
		Where(`"note".tenant_id = ?`, scope.TenantID).
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
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.StudentNote{}, false, domain.OperationStats{}, err
	}
	var row studentNoteRow
	query := db.NewSelect().
		Model(&row).
		ModelTableExpr(studentNotesTable+" AS note").
		Where(`"note".tenant_id = ?`, scope.TenantID).
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
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.StudentNote{}, domain.OperationStats{}, err
	}
	author := create.AuthorAccountID
	row := studentNoteRow{
		TenantID: scope.TenantID, StudentID: create.StudentID, AuthorAccountID: &author,
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

// Update rewrites the editable columns and the described day of one note. Its
// group reference stays fixed, so a correction cannot move a note to another
// activity or group.
func (s *StudentNoteStore) Update(ctx context.Context, update domain.UpdateStudentNote) (domain.StudentNote, domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
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
		Set("subject_date = ?", update.SubjectDate).
		Where(`"note".tenant_id = ?`, scope.TenantID).
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
	db, scope, err := s.tenantDatabase(ctx)
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
		Where(`"note".tenant_id = ?`, scope.TenantID).
		Where(`"note".id = ?`, deletion.ID).
		Where(`"note".deleted_at IS NULL`).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("delete student note: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("delete student note: %w", err)
	}
	stats.Rows = affected
	if affected == 0 {
		return stats, domain.ErrStudentNoteNotFound
	}
	return stats, nil
}

// ImportPermanentHint writes an imported „Betreuernotizen" text to the
// carried-over permanent hint: it replaces the live one, or creates one when
// the child never had one. A hint the school deliberately deleted stays
// deleted, the same rule the 1.15.438 carry-over follows.
func (s *StudentNoteStore) ImportPermanentHint(
	ctx context.Context, studentID int64, body string,
) (domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	body = strings.TrimSpace(body)
	if studentID <= 0 || body == "" {
		return domain.OperationStats{}, domain.ErrStudentNoteInvalid
	}
	updated, stats, err := s.updateLivePermanentHint(ctx, db, scope.TenantID, studentID, body)
	if err != nil || updated {
		return stats, err
	}
	insertStats, err := s.insertPermanentHint(ctx, db, scope.TenantID, studentID, body)
	stats.Add(insertStats)
	return stats, err
}

func (s *StudentNoteStore) updateLivePermanentHint(
	ctx context.Context, db bun.IDB, tenantID, studentID int64, body string,
) (bool, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewUpdate().
		Model((*studentNoteRow)(nil)).
		ModelTableExpr(studentNotesTable+" AS note").
		Set("body = ?", body).
		Where(`"note".tenant_id = ?`, tenantID).
		Where(`"note".student_id = ?`, studentID).
		Where(`"note".origin = ?`, domain.StudentNoteOriginMasterData).
		Where(`"note".deleted_at IS NULL`).
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return false, stats, fmt.Errorf("import permanent hint: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, stats, fmt.Errorf("import permanent hint: %w", err)
	}
	stats.Rows = affected
	return affected > 0, stats, nil
}

func (s *StudentNoteStore) insertPermanentHint(
	ctx context.Context, db bun.IDB, tenantID, studentID int64, body string,
) (domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewRaw(`INSERT INTO users.student_notes
		(tenant_id, student_id, origin, kind, visibility, body)
		SELECT ?, ?, 'master_data', 'permanent', 'all_staff', ?
		WHERE NOT EXISTS (
			SELECT 1 FROM users.student_notes AS note
			WHERE note.tenant_id = ? AND note.student_id = ? AND note.origin = 'master_data'
		)`, tenantID, studentID, body, tenantID, studentID).Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("import permanent hint: %w", err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return stats, fmt.Errorf("import permanent hint: %w", err)
	}
	return stats, nil
}

// schoolDayHintConflict is the arbiter of uniq_student_notes_school_day_hint
// (1.15.514): one live hint per author, child, day and target, the class
// compared the way every class is (trimmed, case-insensitive).
const schoolDayHintConflict = `CONFLICT (tenant_id, student_id, subject_date, author_account_id, ` +
	`(LOWER(BTRIM(COALESCE(school_class, ''))))) ` +
	`WHERE origin = 'school' AND deleted_at IS NULL DO UPDATE`

// ListSchoolDayHints reads the live day hints of one day for the named
// children. Oldest first, so a class hint and a later hint for one child read
// in the order they were written.
func (s *StudentNoteStore) ListSchoolDayHints(
	ctx context.Context, filter domain.SchoolDayHintFilter,
) ([]domain.StudentNote, domain.OperationStats, error) {
	if len(filter.StudentIDs) == 0 {
		return nil, domain.OperationStats{}, nil
	}
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []studentNoteRow
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().
		Model(&rows).
		ModelTableExpr(studentNotesTable+" AS note").
		Where(`"note".tenant_id = ?`, scope.TenantID).
		Where(`"note".origin = ?`, domain.StudentNoteOriginSchool).
		Where(`"note".subject_date = ?`, filter.Date).
		Where(`"note".student_id IN (?)`, bun.List(filter.StudentIDs)).
		Where(`"note".deleted_at IS NULL`).
		OrderExpr(`"note".created_at ASC, "note".id ASC`).
		Scan(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("list school day hints: %w", err)
	}
	stats.Rows = int64(len(rows))
	notes := make([]domain.StudentNote, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, row.toDomain())
	}
	return notes, stats, nil
}

// UpsertSchoolDayHints writes one row per named child in one statement. A
// live hint of the same author for the same child, day and target takes the
// new wording; updated_at moves through the table's trigger.
func (s *StudentNoteStore) UpsertSchoolDayHints(
	ctx context.Context, hint domain.SetSchoolDayHint,
) (domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return domain.OperationStats{}, err
	}
	author := hint.AuthorAccountID
	date := hint.Date
	rows := make([]studentNoteRow, 0, len(hint.StudentIDs))
	for _, studentID := range hint.StudentIDs {
		rows = append(rows, studentNoteRow{
			TenantID: scope.TenantID, StudentID: studentID, AuthorAccountID: &author,
			Origin: domain.StudentNoteOriginSchool, Kind: domain.StudentNoteKindJournal,
			Visibility: domain.StudentNoteVisibilityAllStaff, Body: hint.Body,
			SubjectDate: &date, SchoolClass: optionalText(hint.SchoolClass),
		})
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr(studentNotesTable).
		ExcludeColumn("id", "created_at", "updated_at", "deleted_at", "deleted_by_account_id").
		On(schoolDayHintConflict).
		Set("body = EXCLUDED.body").
		Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return stats, fmt.Errorf("upsert school day hints: %w", err)
	}
	if stats.Rows, err = result.RowsAffected(); err != nil {
		return stats, fmt.Errorf("upsert school day hints: %w", err)
	}
	return stats, nil
}

// RetireSchoolDayHints soft-deletes the author's live hints of one target and
// returns the rows it hid, so the caller can record each removal.
func (s *StudentNoteStore) RetireSchoolDayHints(
	ctx context.Context, target domain.ClearSchoolDayHint, keep []int64,
) ([]domain.StudentNote, domain.OperationStats, error) {
	db, scope, err := s.tenantDatabase(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	var rows []studentNoteRow
	query := db.NewUpdate().
		Model((*studentNoteRow)(nil)).
		ModelTableExpr(studentNotesTable+" AS note").
		Set("deleted_at = clock_timestamp()").
		Set("deleted_by_account_id = ?", target.AuthorAccountID).
		Where(`"note".tenant_id = ?`, scope.TenantID).
		Where(`"note".origin = ?`, domain.StudentNoteOriginSchool).
		Where(`"note".author_account_id = ?`, target.AuthorAccountID).
		Where(`"note".subject_date = ?`, target.Date).
		Where(`"note".deleted_at IS NULL`)
	if target.SchoolClass != "" {
		query = query.Where(`LOWER(BTRIM("note".school_class)) = LOWER(BTRIM(?))`, target.SchoolClass)
		if len(keep) > 0 {
			query = query.Where(`"note".student_id NOT IN (?)`, bun.List(keep))
		}
	} else {
		query = query.Where(`"note".school_class IS NULL`).
			Where(`"note".student_id = ?`, target.StudentID)
	}
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Returning(`"note".*`).Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, stats, fmt.Errorf("retire school day hints: %w", err)
	}
	stats.Rows = int64(len(rows))
	notes := make([]domain.StudentNote, 0, len(rows))
	for _, row := range rows {
		notes = append(notes, row.toDomain())
	}
	return notes, stats, nil
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
	if r.SchoolClass != nil {
		note.Subject.SchoolClass = *r.SchoolClass
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
