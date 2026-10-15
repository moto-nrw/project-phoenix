package api

import (
	"fmt"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/services"
)

// NewStudentScheduleRepositories composes only the student schedule adapters
// needed by legacy service integration tests, over an unobserved Care Plan.
func NewStudentScheduleRepositories(db transactionruntime.Database, dependencies services.TimetableDependencies) StudentScheduleRepositories {
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
