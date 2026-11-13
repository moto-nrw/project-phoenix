package compose

import (
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/timetable/internal/application"
	"github.com/uptrace/bun"
)

// storeService composes one observed Timetable service over the owner's
// store and transaction; name labels the missing-dependency error.
func storeService[T any](db *bun.DB, observe func(Observation), name string, build func(*postgres.Store) T) (T, error) {
	if db == nil || observe == nil {
		var zero T
		return zero, errors.New(name + ": database and observer are required")
	}
	return build(postgres.New(postgres.DatabaseRuntime(db))), nil
}

func NewClassArrivals(db *bun.DB, observe func(Observation)) (timetable.ClassArrivals, error) {
	return storeService(db, observe, "class arrival exceptions", func(store *postgres.Store) timetable.ClassArrivals {
		return application.NewClassArrivals(store, transaction{}, observe)
	})
}

func NewClassArrivalQueries(db *bun.DB, observe func(Observation)) (timetable.ClassArrivalQuery, error) {
	return storeService(db, observe, "class arrival queries", func(store *postgres.Store) timetable.ClassArrivalQuery {
		return application.NewClassArrivalQueries(store, transaction{}, observe)
	})
}

func NewCourseGroupQueries(db *bun.DB, observe func(Observation)) (timetable.CourseGroupQuery, error) {
	return storeService(db, observe, "course group queries", func(store *postgres.Store) timetable.CourseGroupQuery {
		return application.NewCourseGroupQueries(store, transaction{}, observe)
	})
}

// NewRoomReferenceQueries counts the Timetable rows that use a room, for the
// Facilities room removal (#6845).
func NewRoomReferenceQueries(db *bun.DB, observe func(Observation)) (timetable.RoomReferenceQuery, error) {
	return storeService(db, observe, "room reference queries", func(store *postgres.Store) timetable.RoomReferenceQuery {
		return application.NewRoomReferenceQueries(store, observe)
	})
}
