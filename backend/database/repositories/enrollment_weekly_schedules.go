package repositories

import (
	"context"
	"time"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

// NewEnrollmentPickupSchedules binds the weekly pickup times an enrollment
// approval writes to the retained schedule repository. A nil repository
// leaves the port unbound, so the approval skips the pickup schedule.
func NewEnrollmentPickupSchedules(repo scheduleModels.StudentPickupScheduleRepository) enrollmentCompose.WeeklyPickupSchedules {
	if repo == nil {
		return nil
	}
	return enrollmentPickupSchedules{repo: repo}
}

// NewEnrollmentArrivalSchedules binds the weekly arrival days an enrollment
// approval writes to the retained schedule repository. A nil repository
// leaves the port unbound, so the approval skips the arrival schedule.
func NewEnrollmentArrivalSchedules(repo scheduleModels.StudentArrivalScheduleRepository) enrollmentCompose.WeeklyArrivalSchedules {
	if repo == nil {
		return nil
	}
	return enrollmentArrivalSchedules{repo: repo}
}

type enrollmentPickupSchedules struct {
	repo scheduleModels.StudentPickupScheduleRepository
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
	repo scheduleModels.StudentArrivalScheduleRepository
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
