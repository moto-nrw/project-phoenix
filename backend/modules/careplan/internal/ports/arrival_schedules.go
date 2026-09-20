package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ArrivalScheduleRepository interface {
	EffectiveScheduleRepository[*careplan.ArrivalSchedule]
	FindByID(context.Context, any) (*careplan.ArrivalSchedule, error)
}

type ArrivalBulkStudent struct {
	careplan.ScheduleStudent
	PersonID    int64
	SchoolClass string
	GroupID     *int64
	Alumnus     bool
	CareEnded   bool
}

type ArrivalBulkStudents interface {
	ByClass(context.Context, string, calendar.Date) ([]ArrivalBulkStudent, error)
	ByGroup(context.Context, int64, calendar.Date) ([]ArrivalBulkStudent, error)
	ByIDs(context.Context, []int64, calendar.Date) (map[int64]ArrivalBulkStudent, error)
	LockByID(context.Context, int64, calendar.Date) (ArrivalBulkStudent, error)
	LockByIDs(context.Context, []int64, calendar.Date) (map[int64]ArrivalBulkStudent, error)
	Name(context.Context, ArrivalBulkStudent) (string, error)
}

type ArrivalClassPlan struct {
	ID           int64
	TenantID     int64
	CreatedAt    time.Time
	UpdatedAt    time.Time
	SchoolClass  string
	ArrivalTimes map[string]string
	UpdatedBy    *int64
}

// ArrivalClassPlans belongs to Timetable. Care Plan never writes its table
// directly; the owner supplies locking and persistence through this port.
type ArrivalClassPlans interface {
	FindByClasses(context.Context, []string) ([]*ArrivalClassPlan, error)
	LockClass(context.Context, string) error
	Upsert(context.Context, *ArrivalClassPlan) error
}

// ArrivalScheduleRules provides the student lock and current class timetable
// used to avoid persisting a manual deviation identical to the class time.
type ArrivalScheduleRules interface {
	LockStudent(context.Context, int64) error
	ClassTimesForStudent(context.Context, int64) (map[int]time.Time, error)
}

type ArrivalRuleStudents interface {
	LockStudent(context.Context, int64) error
	SchoolClass(context.Context, int64) (string, error)
}
