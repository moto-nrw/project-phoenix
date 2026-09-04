package careplantest

import (
	"context"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanLegacy "github.com/moto-nrw/project-phoenix/modules/careplan/legacy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

type TB interface {
	Helper()
	Fatalf(string, ...any)
}

// NewCarePlan composes the owner capability for integration tests.
func NewCarePlan(tb TB, db *bun.DB) careplan.Capability {
	tb.Helper()
	students := newStudentDirectory(tb, db)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
		StatusStudents: students,
		People:         studentNameFinder(students),
		StudentLock:    students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
	})
	if err != nil {
		tb.Fatalf("compose test Care Plan: %v", err)
	}
	return capability
}

// NewCareOfferingRepository exposes the legacy contract over the owner module
// for integration tests that have not migrated their service seam yet.
func NewCareOfferingRepository(tb TB, db *bun.DB) enrollmentModels.CareOfferingRepository {
	tb.Helper()
	return carePlanLegacy.NewCareOfferingRepository(NewCarePlan(tb, db))
}

// CareOfferingRepository is the no-TB variant for shared test builders.
func CareOfferingRepository(db *bun.DB) enrollmentModels.CareOfferingRepository {
	return carePlanLegacy.NewCareOfferingRepository(carePlan(db))
}

func StudentStatusDayRepository(db *bun.DB) activeModels.StudentStatusDayRepository {
	return carePlanLegacy.NewStudentStatusDayRepository(carePlan(db))
}

func carePlan(db *bun.DB) careplan.Capability {
	students, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	if err != nil {
		panic("compose test People Directory: " + err.Error())
	}
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
		StatusStudents: students,
		People:         studentNameFinder(students),
		StudentLock:    students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
	})
	if err != nil {
		panic("compose test Care Plan: " + err.Error())
	}
	return capability
}

func newStudentDirectory(tb TB, db *bun.DB) *peopledirectory.Module {
	tb.Helper()
	students, err := peopleCompose.New(peopleCompose.Dependencies{
		DB: db, Observe: func(peopleCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test People Directory: %v", err)
	}
	return students
}

func studentNameFinder(students peopledirectory.Capability) carePlanCompose.StudentNameFinder {
	return carePlanCompose.StudentNameFinderFunc(func(ctx context.Context, ids []int64) ([]carePlanCompose.StudentName, error) {
		values, err := students.ListStudentNamesByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		result := make([]carePlanCompose.StudentName, 0, len(values))
		for _, value := range values {
			result = append(result, carePlanCompose.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
		}
		return result, nil
	})
}
