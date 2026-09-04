package careplantest

import (
	"context"

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
		StatusStudents: newStatusStudentDirectory(students), StatusSlots: emptyStatusSlots{},
		People:      studentNameFinder(students),
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
	return carePlanLegacy.NewCareOfferingRepository(carePlan(db))
}

func carePlan(db *bun.DB) careplan.Capability {
	students, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	if err != nil {
		panic("compose test People Directory: " + err.Error())
	}
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanLegacy.NewAmbientDatabase(db),
		StatusStudents: newStatusStudentDirectory(students), StatusSlots: emptyStatusSlots{},
		People:      studentNameFinder(students),
		StudentLock: students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
	})
	if err != nil {
		panic("compose test Care Plan: " + err.Error())
	}
	return capability
}

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
}

func newStatusStudentDirectory(students peopledirectory.Capability) statusStudentDirectory {
	flags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		panic("test People Directory does not expose status flags")
	}
	return statusStudentDirectory{students: students, flags: flags}
}

func (d statusStudentDirectory) ListEnrolledStudents(ctx context.Context) ([]carePlanCompose.StatusStudent, error) {
	values, err := d.students.ListEnrolledStudents(ctx)
	return statusStudents(values), err
}

func (d statusStudentDirectory) ListStudentsWithStatusFlag(ctx context.Context, status string) ([]carePlanCompose.StatusStudent, error) {
	values, err := d.flags.ListStudentsWithStatusFlag(ctx, status)
	return statusStudents(values), err
}

func (d statusStudentDirectory) ClearStudentStatusFlags(ctx context.Context, ids []int64, status string) (int64, error) {
	return d.flags.ClearStudentStatusFlags(ctx, ids, status)
}

func (d statusStudentDirectory) LockStudent(ctx context.Context, id int64) error {
	return d.students.LockStudent(ctx, id)
}

func statusStudents(values []peopledirectory.Student) []carePlanCompose.StatusStudent {
	result := make([]carePlanCompose.StatusStudent, 0, len(values))
	for _, value := range values {
		result = append(result, carePlanCompose.StatusStudent{
			ID: value.ID, TenantID: value.TenantID, Status: value.Status,
			Sick: value.Sick, SickSince: value.SickSince, Excused: value.Excused, ExcusedSince: value.ExcusedSince,
		})
	}
	return result
}

type emptyStatusSlots struct{}

func (emptyStatusSlots) ApplyStatusDay(context.Context, int64, careplan.Date, int64, string) (int, error) {
	return 0, nil
}

func (emptyStatusSlots) ReleaseStatusDay(context.Context, int64) (int, error) { return 0, nil }

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
