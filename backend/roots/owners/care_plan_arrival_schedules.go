package owners

import (
	"errors"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
)

// ArrivalScheduleBinding binds the Care Plan arrival schedules to the People
// Directory and the supplied class plans and class exceptions. The caller
// hands the rules and dependencies with its database to
// careplanCompose.NewArrivalSchedules.
func ArrivalScheduleBinding(people peopledirectory.Capability, classes careplanCompose.ArrivalClassPlans, exceptions careplan.ClassArrivalExceptions, logger *slog.Logger) (careplanCompose.ArrivalScheduleRules, careplanCompose.ArrivalScheduleDependencies, error) {
	if people == nil {
		return nil, careplanCompose.ArrivalScheduleDependencies{}, errors.New("arrival schedules: people directory is required")
	}
	students := arrivalPlanStudents{people}
	return careplanCompose.NewArrivalScheduleRules(students, classes), careplanCompose.ArrivalScheduleDependencies{
		Students: students, Classes: classes, ClassExceptions: exceptions, Logger: logger,
	}, nil
}
