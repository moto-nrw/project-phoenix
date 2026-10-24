package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
)

// EnrollmentPickupSchedules replaces a student's weekly pickup times; it is
// the shape of Enrollment's weekly pickup port.
type EnrollmentPickupSchedules interface {
	DeletePickupSchedules(ctx context.Context, studentID int64) error
	UpsertPickupSchedule(ctx context.Context, studentID int64, weekday int, pickupTime time.Time, createdBy int64) error
}

// EnrollmentArrivalSchedules replaces a student's weekly arrival days; it is
// the shape of Enrollment's weekly arrival port.
type EnrollmentArrivalSchedules interface {
	DeleteArrivalSchedules(ctx context.Context, studentID int64) error
	CreateArrivalSchedule(ctx context.Context, studentID int64, weekday int, createdBy int64) error
}

// NewEnrollmentPickupSchedules binds the weekly pickup times an enrollment
// approval writes to the retained schedule repository. A nil repository
// leaves the port unbound, so the approval skips the pickup schedule.
func NewEnrollmentPickupSchedules(repo ports.StudentPickupScheduleRecords) EnrollmentPickupSchedules {
	if repo == nil {
		return nil
	}
	return enrollmentPickupSchedules{repo: repo}
}

// NewEnrollmentArrivalSchedules binds the weekly arrival days an enrollment
// approval writes to the retained schedule repository. A nil repository
// leaves the port unbound, so the approval skips the arrival schedule.
func NewEnrollmentArrivalSchedules(repo ports.StudentArrivalScheduleRecords) EnrollmentArrivalSchedules {
	if repo == nil {
		return nil
	}
	return enrollmentArrivalSchedules{repo: repo}
}

type enrollmentPickupSchedules struct {
	repo ports.StudentPickupScheduleRecords
}

func (s enrollmentPickupSchedules) DeletePickupSchedules(ctx context.Context, studentID int64) error {
	return s.repo.DeleteByStudentID(ctx, studentID)
}

func (s enrollmentPickupSchedules) UpsertPickupSchedule(ctx context.Context, studentID int64, weekday int, pickupTime time.Time, createdBy int64) error {
	return s.repo.UpsertSchedule(ctx, &scheduleModels.StudentPickupSchedule{
		StudentID:  studentID,
		Weekday:    weekday,
		PickupTime: pickupTime,
		CreatedBy:  createdBy,
	})
}

type enrollmentArrivalSchedules struct {
	repo ports.StudentArrivalScheduleRecords
}

func (s enrollmentArrivalSchedules) DeleteArrivalSchedules(ctx context.Context, studentID int64) error {
	return s.repo.DeleteByStudentID(ctx, studentID)
}

func (s enrollmentArrivalSchedules) CreateArrivalSchedule(ctx context.Context, studentID int64, weekday int, createdBy int64) error {
	return s.repo.Create(ctx, &scheduleModels.StudentArrivalSchedule{
		StudentID: studentID,
		Weekday:   weekday,
		CreatedBy: createdBy,
	})
}
