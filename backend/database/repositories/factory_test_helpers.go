package repositories

import (
	"time"

	"github.com/uptrace/bun"
)

// NewFactoryWithPeopleDirectory builds the repository factory with the
// People Directory and Timetable capability already bound, so repository tests
// read the same person-enriched rows and execute the same care-exit booking
// writes as the service graph does.
func NewFactoryWithPeopleDirectory(db *bun.DB, dependencies TimetableDependencies, clocks ...func() time.Time) (*Factory, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	factory := NewFactory(db, dependencies, clocks...)
	factory.BindPeopleDirectory(persons)
	return factory, nil
}

// NewStudentScheduleRepositories composes only the student schedule adapters
// needed by legacy service integration tests.
func NewStudentScheduleRepositories(db *bun.DB) StudentScheduleRepositories {
	repositories := NewFactory(db, NewUnobservedTimetableDependencies(db))
	return StudentScheduleRepositories{
		ArrivalSchedule:  repositories.StudentArrivalSchedule,
		ArrivalException: repositories.StudentArrivalException,
		ArrivalNote:      repositories.StudentArrivalNote,
		PickupSchedule:   repositories.StudentPickupSchedule,
		PickupException:  repositories.StudentPickupException,
		PickupNote:       repositories.StudentPickupNote,
		StatusDay:        repositories.StudentStatusDay,
	}
}
