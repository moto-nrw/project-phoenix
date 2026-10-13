package services

import (
	"fmt"

	"github.com/uptrace/bun"
)

// NewStudentScheduleRepositories composes only the student schedule adapters
// needed by legacy service integration tests, over an unobserved Care Plan.
func NewStudentScheduleRepositories(db *bun.DB, dependencies TimetableDependencies) StudentScheduleRepositories {
	carePlan, err := NewUnobservedCarePlan(db, dependencies)
	if err != nil {
		panic(fmt.Sprintf("student schedule repositories: compose care plan: %v", err))
	}
	return StudentScheduleRepositories{
		ArrivalSchedule:  NewArrivalScheduleRepository(carePlan),
		ArrivalException: NewArrivalExceptionRepository(carePlan),
		ArrivalNote:      NewArrivalNoteRepository(carePlan),
		PickupSchedule:   NewPickupScheduleRepository(carePlan),
		PickupException:  NewPickupExceptionRepository(carePlan),
		PickupNote:       NewPickupNoteRepository(carePlan),
		StatusDay:        NewStudentStatusDayRepository(carePlan),
	}
}
