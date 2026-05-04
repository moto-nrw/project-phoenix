package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableActiveStudentStatusDays = "active.student_status_days"

const (
	StudentStatusDaySick    = "sick"
	StudentStatusDayExcused = "excused"
)

const (
	StudentStatusSourceManual      = "manual"
	StudentStatusSourceNextCheckin = "next_checkin"
	StudentStatusSourceEndOfDay    = "end_of_day"
)

type StudentStatusDay struct {
	base.Model `bun:"schema:active,table:student_status_days"`
	base.TenantModel
	StudentID  int64      `bun:"student_id,notnull" json:"student_id"`
	Date       time.Time  `bun:"date,notnull,type:date" json:"date"`
	Status     string     `bun:"status,notnull" json:"status"`
	ReportedAt time.Time  `bun:"reported_at,notnull" json:"reported_at"`
	ClearedAt  *time.Time `bun:"cleared_at" json:"cleared_at,omitempty"`
	Source     string     `bun:"source,notnull" json:"source"`
}

func (s *StudentStatusDay) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableActiveStudentStatusDays)
	}
	return nil
}

func (s *StudentStatusDay) GetID() any              { return s.ID }
func (s *StudentStatusDay) GetCreatedAt() time.Time { return s.CreatedAt }
func (s *StudentStatusDay) GetUpdatedAt() time.Time { return s.UpdatedAt }
func (s *StudentStatusDay) TableName() string       { return tableActiveStudentStatusDays }

type StudentStatusDayRepository interface {
	UpsertReported(ctx context.Context, entry *StudentStatusDay) error
	MarkCleared(ctx context.Context, studentID int64, status string, date time.Time, clearedAt time.Time, source string) error
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate time.Time) ([]*StudentStatusDay, error)
}
