package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentdirectoryview"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres/calendar"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/domain"
	"github.com/uptrace/bun"
)

type studentRow struct {
	bun.BaseModel `bun:"table:student_profiles,alias:student"`
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

// StudentStore binds profile writes and Directory reads; it shares the database
// runtime with the person store.
type StudentStore struct {
	database       Database
	reads          *studentdirectoryview.Projection
	classGate      func(context.Context, bool) error
	classGateQuery func(context.Context) (*bun.SelectQuery, error)
}

func NewStudentStore(database Database, classGate func(context.Context, bool) error, classGateQuery func(context.Context) (*bun.SelectQuery, error)) *StudentStore {
	if database == nil {
		panic("people directory postgres: database runtime is required")
	}
	return &StudentStore{database: database, reads: studentdirectoryview.New(studentdirectoryview.Database(database)), classGate: classGate, classGateQuery: classGateQuery}
}

// Lock takes the student row FOR UPDATE. It is the first lock every care-day
// writer acquires, so the statement stays a bare row lock without joins.
func (s *StudentStore) Lock(ctx context.Context, id int64) (bool, domain.OperationStats, error) {
	_, tenantID, err := s.database(ctx)
	if err != nil {
		return false, domain.OperationStats{}, err
	}
	if tenantID <= 0 {
		return false, domain.OperationStats{}, errors.New("people directory: tenant is required to lock a student")
	}
	_, found, stats, err := s.FindRecord(ctx, id, "UPDATE")
	return found, stats, err
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
