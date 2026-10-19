package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	directory "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

type directoryStudentRow struct {
	bun.BaseModel `bun:"alias:student"`
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

// StudentDirectory is People Directory's reader port onto the student
// directory projection (ADR 0058); the composition root binds it. Each row
// source is aliased "student", joins the three student owners and runs in
// the caller's statement and transaction. Cross-school sources are explicit;
// the school-specific methods reject an absent school.
type StudentDirectory interface {
	StudentRows(db bun.IDB, tenantID int64) *bun.SelectQuery
	AllSchoolsStudentRows(db bun.IDB) *bun.SelectQuery
	StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
	AllSchoolsStudentModelRows(db bun.IDB, model any) *bun.SelectQuery
}

// studentDirectoryReads are the student reads over the projection's row
// source, using the caller's database runtime.
type studentDirectoryReads struct {
	database Database
	rows     StudentDirectory
}

func newStudentDirectoryReads(database Database, rows StudentDirectory) *studentDirectoryReads {
	if database == nil {
		panic("student directory projection: database runtime is required")
	}
	if rows == nil {
		panic("people directory postgres: student directory projection is required")
	}
	return &studentDirectoryReads{database: database, rows: rows}
}

// directoryStats is what one directory read cost.
type directoryStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

// directoryFilter narrows the staff directory page.
type directoryFilter struct {
	IDs           []int64
	SchoolClasses []string
	GradeLevels   []string
	KeepAlumni    []int64
	CareStatus    string
	CareStatusOn  string
	Page          int
	PageSize      int
}

func directoryDateParam(value string) *calendar.Date {
	if value == "" {
		return nil
	}
	day, err := calendar.ParseDate(value)
	if err != nil {
		return nil
	}
	return &day
}

func isLockNotAvailable(err error) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.Field('C') == "55P03"
}

func (s *studentDirectoryReads) ListByIDs(ctx context.Context, ids []int64) ([]directory.Student, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryStudentRow{}
	query := withStudentTenant(s.studentModelRows(db, scope, &rows).Where(`"student".id IN (?)`, bun.List(ids)), scope)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students by id")
}

func (s *studentDirectoryReads) ListNamesByIDs(ctx context.Context, ids []int64) ([]directory.StudentName, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directory.StudentName{}
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name`).
		Join(`JOIN users.persons AS "person" ON "person".tenant_id = "student".tenant_id AND "person".id = "student".person_id`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`), scope).
		OrderExpr(`"student".id ASC`)
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list student names by id: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func (s *studentDirectoryReads) ListByClasses(ctx context.Context, classes []string) ([]directory.Student, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryStudentRow{}
	query := withStudentTenant(s.studentModelRows(db, scope, &rows).
		Where(`"student".school_class IN (?)`, bun.List(classes)).
		Where(`"student".status <> ?`, directory.StudentStatusAlumnus), scope)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".school_class ASC, "student".id ASC`), "list students by class")
}

func (s *studentDirectoryReads) ListByPersonIDs(ctx context.Context, personIDs []int64) ([]directory.Student, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	rows := []directoryStudentRow{}
	query := withStudentTenant(s.studentModelRows(db, scope, &rows).Where(`"student".person_id IN (?)`, bun.List(personIDs)), scope)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students by person")
}

func (s *studentDirectoryReads) ListEnrolled(ctx context.Context) ([]directory.Student, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, directoryStats{}, errors.New("student directory projection: tenant is required to list enrolled students")
	}
	rows := []directoryStudentRow{}
	query := withStudentTenant(s.studentModelRows(db, scope, &rows).
		Where(`"student".status <> ?`, directory.StudentStatusAlumnus), scope)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list enrolled students")
}

func (s *studentDirectoryReads) ListClasses(ctx context.Context) ([]string, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, directoryStats{}, errors.New("student directory projection: tenant is required to list classes")
	}
	var classes []string
	query := withStudentTenant(s.studentRows(db, scope).
		ColumnExpr(`DISTINCT "student".school_class`).
		Where(`"student".school_class IS NOT NULL AND "student".school_class <> ''`).
		Where(`"student".status <> ?`, directory.StudentStatusAlumnus), scope).
		OrderExpr(`"student".school_class ASC`)
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &classes)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list classes: %w", err)
	}
	if classes == nil {
		classes = []string{}
	}
	return classes, stats, nil
}

func (s *studentDirectoryReads) ListByStatusFlag(ctx context.Context, status string) ([]directory.Student, directoryStats, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, directoryStats{}, err
	}
	if scope.TenantID <= 0 {
		return nil, directoryStats{}, errors.New("student directory projection: tenant is required to list students with a status flag")
	}
	rows := []directoryStudentRow{}
	query := withStudentTenant(s.studentModelRows(db, scope, &rows), scope)
	switch status {
	case "sick":
		query = query.Where(`"student".sick = TRUE`)
	case "excused":
		query = query.Where(`"student".excused = TRUE`)
	default:
		return nil, directoryStats{}, fmt.Errorf("student directory projection: unsupported student status flag %q", status)
	}
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students with status flag")
}

func scanStudents(ctx context.Context, rows *[]directoryStudentRow, query *bun.SelectQuery, operation string) ([]directory.Student, directoryStats, error) {
	stats := directoryStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: %s: %w", operation, err)
	}
	result := make([]directory.Student, 0, len(*rows))
	for _, row := range *rows {
		result = append(result, toDirectoryStudent(row))
	}
	return result, stats, nil
}

func withStudentTenant[Q interface {
	Where(string, ...any) Q
	Err(error) Q
}](query Q, scope TenantScope) Q {
	if err := scope.Validate(); err != nil {
		return query.Err(err)
	}
	if scope.AcrossSchools {
		return query
	}
	return query.Where(`"student".tenant_id = ?`, scope.TenantID)
}

func toDirectoryStudent(row directoryStudentRow) directory.Student {
	student := directory.Student{
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

func (s *studentDirectoryReads) studentRows(db bun.IDB, scope TenantScope) *bun.SelectQuery {
	if scope.AcrossSchools {
		return s.rows.AllSchoolsStudentRows(db)
	}
	return s.rows.StudentRows(db, scope.TenantID)
}
func (s *studentDirectoryReads) studentModelRows(db bun.IDB, scope TenantScope, model any) *bun.SelectQuery {
	if scope.AcrossSchools {
		return s.rows.AllSchoolsStudentModelRows(db, model)
	}
	return s.rows.StudentModelRows(db, scope.TenantID, model)
}
