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

type studentRow struct {
	bun.BaseModel `bun:"table:students,alias:student"`
	ID            int64          `bun:"id,pk,autoincrement"`
	CreatedAt     time.Time      `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time      `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	TenantID      int64          `bun:"tenant_id,notnull"`
	PersonID      int64          `bun:"person_id,notnull"`
	SchoolClass   string         `bun:"school_class,notnull"`
	GroupID       *int64         `bun:"group_id"`
	Status        string         `bun:"status,notnull"`
	EnrolledFrom  *calendar.Date `bun:"enrolled_from,type:date"`
	EnrolledUntil *calendar.Date `bun:"enrolled_until,type:date"`
	Sick          *bool          `bun:"sick"`
	SickSince     *time.Time     `bun:"sick_since"`
	Excused       *bool          `bun:"excused"`
	ExcusedSince  *time.Time     `bun:"excused_since"`
	PhotoPath     *string        `bun:"photo_path"`
}

// StudentStore is the users.students adapter; it shares the database
// runtime with the person store.
type StudentStore struct{ database Database }

func NewStudentStore(database Database) *StudentStore {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &StudentStore{database: database}
}

func (s *StudentStore) ListByIDs(ctx context.Context, ids []int64) ([]domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).Where(`"student".id IN (?)`, bun.List(ids)), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students by id")
}

func (s *StudentStore) ListNamesByIDs(ctx context.Context, ids []int64) ([]domain.StudentName, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []domain.StudentName{}
	query := withStudentTenant(db.NewSelect().TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name`).
		Join(`JOIN users.persons AS "person" ON "person".tenant_id = "student".tenant_id AND "person".id = "student".person_id`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`), tenantID).
		OrderExpr(`"student".id ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list student names by id: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func (s *StudentStore) ListByClasses(ctx context.Context, classes []string) ([]domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).
		Where(`"student".school_class IN (?)`, bun.List(classes)).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".school_class ASC, "student".id ASC`), "list students by class")
}

func (s *StudentStore) ListEnrolled(ctx context.Context) ([]domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to list enrolled students")
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list enrolled students")
}

func (s *StudentStore) ListClasses(ctx context.Context) ([]string, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to list classes")
	}
	var classes []string
	query := withStudentTenant(db.NewSelect().TableExpr(`users.students AS "student"`).
		ColumnExpr(`DISTINCT "student".school_class`).
		Where(`"student".school_class IS NOT NULL AND "student".school_class <> ''`).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID).
		OrderExpr(`"student".school_class ASC`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &classes)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: list classes: %w", err)
	}
	if classes == nil {
		classes = []string{}
	}
	return classes, stats, nil
}

func (s *StudentStore) ListByStatusFlag(ctx context.Context, status string) ([]domain.Student, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to list students with a status flag")
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows), tenantID)
	switch status {
	case "sick":
		query = query.Where(`"student".sick = TRUE`)
	case "excused":
		query = query.Where(`"student".excused = TRUE`)
	default:
		return nil, domain.OperationStats{}, fmt.Errorf("people directory postgres: unsupported student status flag %q", status)
	}
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students with status flag")
}

// Lock takes the student row FOR UPDATE. It is the first lock every care-day
// writer acquires, so the statement stays a bare row lock without joins.
func (s *StudentStore) Lock(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return false, domain.OperationStats{}, errors.New("people directory postgres: tenant is required to lock a student")
	}
	var lockedID int64
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err = db.NewSelect().TableExpr(`users.students AS "student"`).
		ColumnExpr(`"student".id`).
		Where(`"student".id = ?`, id).
		Where(`"student".tenant_id = ?`, tenantID).
		For("UPDATE").
		Scan(ctx, &lockedID)
	stats.StatementDuration = time.Since(started)
	if errors.Is(err, sql.ErrNoRows) {
		return false, stats, nil
	}
	if err != nil {
		return false, stats, fmt.Errorf("people directory postgres: lock student: %w", err)
	}
	stats.Rows = 1
	return true, stats, nil
}

func (s *StudentStore) Promote(ctx context.Context, ids []int64, fromClass, toClass string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`school_class = ?`, toClass).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".school_class = ?`, fromClass).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return execStudents(ctx, query, "promote students")
}

func (s *StudentStore) RevertClass(ctx context.Context, id int64, fromClass, toClass string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`school_class = ?`, fromClass).
		Where(`"student".id = ?`, id).
		Where(`"student".school_class = ?`, toClass), tenantID)
	return execStudents(ctx, query, "revert student class")
}

func (s *StudentStore) GraduateByClasses(ctx context.Context, classes []string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, domain.StudentStatusAlumnus).
		Where(`"student".school_class IN (?)`, bun.List(classes)).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return execStudents(ctx, query, "graduate students by class")
}

func (s *StudentStore) GraduateByIDs(ctx context.Context, ids []int64) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, domain.StudentStatusAlumnus).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return execStudents(ctx, query, "graduate students by id")
}

func (s *StudentStore) Reactivate(ctx context.Context, ids []int64, status string) ([]int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return nil, domain.OperationStats{}, err
	}
	type idRow struct {
		ID int64 `bun:"id"`
	}
	var rows []idRow
	query := withStudentTenant(studentUpdate(db).
		Set(`status = ?`, status).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".status = ?`, domain.StudentStatusAlumnus), tenantID).
		Returning(`"student".id`)
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	_, err = query.Exec(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: reactivate students: %w", err)
	}
	stats.Rows = int64(len(rows))
	result := make([]int64, 0, len(rows))
	for _, row := range rows {
		result = append(result, row.ID)
	}
	return result, stats, nil
}

func (s *StudentStore) ClearStatusFlags(ctx context.Context, ids []int64, status string) (int64, domain.OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return 0, domain.OperationStats{}, err
	}
	if err := requireStudentWriteTenant(tenantID); err != nil {
		return 0, domain.OperationStats{}, err
	}
	query := studentUpdate(db).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"student".tenant_id = ?`, tenantID)
	switch status {
	case "sick":
		query = query.Set(`sick = FALSE`).Set(`sick_since = NULL`).Where(`"student".sick = TRUE`)
	case "excused":
		query = query.Set(`excused = FALSE`).Set(`excused_since = NULL`).Where(`"student".excused = TRUE`)
	default:
		return 0, domain.OperationStats{}, fmt.Errorf("people directory postgres: unsupported student status flag %q", status)
	}
	return execStudents(ctx, query, "clear student status flags")
}

func execStudents(ctx context.Context, query *bun.UpdateQuery, operation string) (int64, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	result, err := query.Exec(ctx)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	stats.Rows, err = result.RowsAffected()
	if err != nil {
		return 0, stats, fmt.Errorf("people directory postgres: %s: count rows: %w", operation, err)
	}
	return stats.Rows, stats, nil
}

func scanStudents(ctx context.Context, rows *[]studentRow, query *bun.SelectQuery, operation string) ([]domain.Student, domain.OperationStats, error) {
	stats := domain.OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("people directory postgres: %s: %w", operation, err)
	}
	result := make([]domain.Student, 0, len(*rows))
	for _, row := range *rows {
		result = append(result, toStudent(row))
	}
	return result, stats, nil
}

func studentSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(`users.students AS "student"`)
}

func studentUpdate(db bun.IDB) *bun.UpdateQuery {
	return db.NewUpdate().Model((*studentRow)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set(`updated_at = NOW()`)
}

func withStudentTenant[Q interface{ Where(string, ...any) Q }](query Q, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"student".tenant_id = ?`, tenantID)
	}
	return query
}

func requireStudentWriteTenant(tenantID int64) error {
	if tenantID <= 0 {
		return errors.New("people directory postgres: tenant is required to write students")
	}
	return nil
}

func toStudent(row studentRow) domain.Student {
	student := domain.Student{
		ID: row.ID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, TenantID: row.TenantID,
		PersonID: row.PersonID, SchoolClass: row.SchoolClass, GroupID: row.GroupID, Status: row.Status,
		Sick: row.Sick, SickSince: row.SickSince, Excused: row.Excused, ExcusedSince: row.ExcusedSince, PhotoPath: row.PhotoPath,
	}
	if row.EnrolledFrom != nil {
		student.EnrolledFrom = row.EnrolledFrom.String()
	}
	if row.EnrolledUntil != nil {
		student.EnrolledUntil = row.EnrolledUntil.String()
	}
	return student
}
