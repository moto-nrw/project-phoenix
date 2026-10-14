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
		ArrivalSchedule:  services.NewArrivalScheduleRepository(carePlan),
		ArrivalException: services.NewArrivalExceptionRepository(carePlan),
		ArrivalNote:      NewArrivalNoteRepository(carePlan),
		PickupSchedule:   services.NewPickupScheduleRepository(carePlan),
		PickupException:  services.NewPickupExceptionRepository(carePlan),
		PickupNote:       NewPickupNoteRepository(carePlan),
		StatusDay:        services.NewStudentStatusDayRepository(carePlan),
	}
}
