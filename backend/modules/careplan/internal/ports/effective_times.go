package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type EffectiveTimeEntity interface {
	Validate() error
	SetTenantID(int64)
}

type EffectiveScheduleRepository[S EffectiveTimeEntity] interface {
	FindByStudentID(context.Context, int64) ([]S, error)
	FindByStudentIDAndWeekday(context.Context, int64, int) (S, error)
	FindByStudentIDsAndWeekday(context.Context, []int64, int) ([]S, error)
	FindByStudentIDs(context.Context, []int64) ([]S, error)
	UpsertSchedule(context.Context, S) error
	Create(context.Context, S) error
	Delete(context.Context, int64) error
	DeleteByStudentID(context.Context, int64) error
}

type EffectiveExceptionRepository[E EffectiveTimeEntity] interface {
	FindByID(context.Context, int64) (E, error)
	FindByIDForUpdate(context.Context, int64) (E, error)
	FindByStudentID(context.Context, int64) ([]E, error)
	FindUpcomingByStudentID(context.Context, int64) ([]E, error)
	FindByStudentIDAndDate(context.Context, int64, calendar.Date) (E, error)
	FindByStudentIDsAndDate(context.Context, []int64, calendar.Date) ([]E, error)
	Create(context.Context, E) error
	Update(context.Context, E) error
	Delete(context.Context, int64) error
	DeleteByStudentID(context.Context, int64) error
}

type EffectiveNoteRepository[N EffectiveTimeEntity] interface {
	FindByID(context.Context, int64) (N, error)
	FindByStudentID(context.Context, int64) ([]N, error)
	FindByStudentIDAndDate(context.Context, int64, calendar.Date) ([]N, error)
	FindByStudentIDsAndDate(context.Context, []int64, calendar.Date) ([]N, error)
	Create(context.Context, N) error
	Update(context.Context, N) error
	Delete(context.Context, int64) error
	DeleteByStudentID(context.Context, int64) error
}

type EffectiveTimeDomain[S EffectiveTimeEntity, E EffectiveTimeEntity, N EffectiveTimeEntity] interface {
	Name() string
	CollisionPolicy() domain.ExceptionCollisionPolicy
	ScheduleFields(S) domain.EffectiveScheduleFields
	SetScheduleStudentID(S, int64)
	ExceptionFields(E) domain.EffectiveExceptionFields
	NewException(domain.EffectiveExceptionFields) E
	AssignException(E, E)
	NoteFields(N) domain.EffectiveNoteFields
}

// EffectiveTimeTransaction keeps database transactions and driver errors at the adapter boundary.
// LockStudentAndExceptionDay must acquire the student row before the care-day lock.
type EffectiveTimeTransaction interface {
	TenantID(context.Context) int64
	WithinTenant(context.Context, func(context.Context) error) error
	LockStudentAndExceptionDay(context.Context, int64, string) error
	IsNotFound(error) bool
}
