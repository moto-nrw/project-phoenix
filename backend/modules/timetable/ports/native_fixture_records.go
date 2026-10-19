//go:build testhooks

package ports

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type NativeGroupRecords interface {
	Create(context.Context, *timetable.Group) error
	Update(context.Context, *timetable.Group) error
	FindByID(context.Context, any) (*timetable.Group, error)
}
type NativeScheduleRecords interface {
	Create(context.Context, *timetable.Schedule) error
	Update(context.Context, *timetable.Schedule) error
	FindByID(context.Context, any) (*timetable.Schedule, error)
	FindByGroupID(context.Context, int64) ([]*timetable.Schedule, error)
}
type NativeEnrollmentRecords interface {
	Create(context.Context, *timetable.StudentEnrollment) error
	Update(context.Context, *timetable.StudentEnrollment) error
	FindByID(context.Context, any) (*timetable.StudentEnrollment, error)
	FindActiveByStudentIDs(context.Context, []int64, string) ([]*timetable.StudentEnrollment, error)
}
type NativeExceptionRecords interface {
	Create(context.Context, *timetable.ActivityException) error
	Update(context.Context, *timetable.ActivityException) error
	FindByID(context.Context, any) (*timetable.ActivityException, error)
}
type NativeTimeframeRecords interface {
	Create(context.Context, *timetable.Timeframe) error
	Update(context.Context, *timetable.Timeframe) error
	FindByID(context.Context, any) (*timetable.Timeframe, error)
}
