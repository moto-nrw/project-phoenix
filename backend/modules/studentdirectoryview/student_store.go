package studentdirectoryview

import (
	"context"
	"errors"
	"fmt"
	"time"

	calendar "github.com/moto-nrw/project-phoenix/internal/timezone"
	domain "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/uptrace/bun"
)

type studentRow struct {
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

// Projection joins the current student owners using the caller's database runtime.
type Projection struct{ database Database }

func New(database Database) *Projection {
	if database == nil {
		panic("student directory projection: database runtime is required")
	}
	return &Projection{database: database}
}

func (s *Projection) ListByIDs(ctx context.Context, ids []int64) ([]domain.Student, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).Where(`"student".id IN (?)`, bun.List(ids)), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students by id")
}

func (s *Projection) ListNamesByIDs(ctx context.Context, ids []int64) ([]domain.StudentName, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	rows := []domain.StudentName{}
	query := withStudentTenant(db.NewSelect().TableExpr(studentSource).
		ColumnExpr(`"student".id AS student_id, "person".first_name, "person".last_name`).
		Join(`JOIN users.persons AS "person" ON "person".tenant_id = "student".tenant_id AND "person".id = "student".person_id`).
		Where(`"student".id IN (?)`, bun.List(ids)).
		Where(`"person".deleted_at IS NULL`), tenantID).
		OrderExpr(`"student".id ASC`)
	stats := OperationStats{Queries: 1}
	started := time.Now()
	err = query.Scan(ctx, &rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: list student names by id: %w", err)
	}
	stats.Rows = int64(len(rows))
	return rows, stats, nil
}

func (s *Projection) ListByClasses(ctx context.Context, classes []string) ([]domain.Student, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).
		Where(`"student".school_class IN (?)`, bun.List(classes)).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".school_class ASC, "student".id ASC`), "list students by class")
}

func (s *Projection) ListByPersonIDs(ctx context.Context, personIDs []int64) ([]domain.Student, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).Where(`"student".person_id IN (?)`, bun.List(personIDs)), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students by person")
}

func (s *Projection) ListEnrolled(ctx context.Context) ([]domain.Student, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, OperationStats{}, errors.New("student directory projection: tenant is required to list enrolled students")
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID)
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list enrolled students")
}

func (s *Projection) ListClasses(ctx context.Context) ([]string, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, OperationStats{}, errors.New("student directory projection: tenant is required to list classes")
	}
	var classes []string
	query := withStudentTenant(db.NewSelect().TableExpr(studentSource).
		ColumnExpr(`DISTINCT "student".school_class`).
		Where(`"student".school_class IS NOT NULL AND "student".school_class <> ''`).
		Where(`"student".status <> ?`, domain.StudentStatusAlumnus), tenantID).
		OrderExpr(`"student".school_class ASC`)
	stats := OperationStats{Queries: 1}
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

func (s *Projection) ListByStatusFlag(ctx context.Context, status string) ([]domain.Student, OperationStats, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, OperationStats{}, err
	}
	if tenantID <= 0 {
		return nil, OperationStats{}, errors.New("student directory projection: tenant is required to list students with a status flag")
	}
	rows := []studentRow{}
	query := withStudentTenant(studentSelect(db, &rows), tenantID)
	switch status {
	case "sick":
		query = query.Where(`"student".sick = TRUE`)
	case "excused":
		query = query.Where(`"student".excused = TRUE`)
	default:
		return nil, OperationStats{}, fmt.Errorf("student directory projection: unsupported student status flag %q", status)
	}
	return scanStudents(ctx, &rows, query.OrderExpr(`"student".id ASC`), "list students with status flag")
}

func scanStudents(ctx context.Context, rows *[]studentRow, query *bun.SelectQuery, operation string) ([]domain.Student, OperationStats, error) {
	stats := OperationStats{Queries: 1}
	started := time.Now()
	err := query.Scan(ctx, rows)
	stats.StatementDuration = time.Since(started)
	if err != nil {
		return nil, stats, fmt.Errorf("student directory projection: %s: %w", operation, err)
	}
	result := make([]domain.Student, 0, len(*rows))
	for _, row := range *rows {
		result = append(result, toStudent(row))
	}
	return result, stats, nil
}

func studentSelect(db bun.IDB, model any) *bun.SelectQuery {
	return db.NewSelect().Model(model).ModelTableExpr(studentSource)
}

func withStudentTenant[Q interface{ Where(string, ...any) Q }](query Q, tenantID int64) Q {
	if tenantID > 0 {
		return query.Where(`"student".tenant_id = ?`, tenantID)
	}
	return query
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
