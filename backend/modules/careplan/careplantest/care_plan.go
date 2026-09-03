package careplantest

import (
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
		StudentLock: students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
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
	students, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	if err != nil {
		panic("compose test People Directory: " + err.Error())
	}
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
		StudentLock: students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
	})
	if err != nil {
		panic("compose test Care Plan: " + err.Error())
	}
	return carePlanLegacy.NewCareOfferingRepository(capability)
}

func newStudentDirectory(tb TB, db *bun.DB) peopledirectory.Capability {
	tb.Helper()
	students, err := peopleCompose.New(peopleCompose.Dependencies{
		DB: db, Observe: func(peopleCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test People Directory: %v", err)
	}
	return students
}
