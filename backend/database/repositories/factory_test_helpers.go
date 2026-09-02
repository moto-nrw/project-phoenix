package repositories

import (
	"context"
	"time"

	"github.com/uptrace/bun"
)

// NewFactoryWithPeopleDirectory builds the repository factory with the
// People Directory already bound, so repository tests read the same
// person-enriched rows the service graph does.
func NewFactoryWithPeopleDirectory(db *bun.DB, clocks ...func() time.Time) (*Factory, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	factory := NewFactory(db, clocks...)
	factory.BindPeopleDirectory(persons)
	return factory, nil
}

// NewCareStudentLock composes the People Directory's student row lock for
// test graphs that bind timetable services without services.NewFactory
// (#2662). It returns the lock and the sentinel the lock reports for a
// missing child; the directory reads the transaction from the context, so
// any open pool of the test database serves as the composition anchor.
func NewCareStudentLock(db *bun.DB) (lock func(context.Context, int64) error, notFound error, err error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, nil, err
	}
	lock, notFound = CareStudentLock(persons)
	return lock, notFound, nil
}
