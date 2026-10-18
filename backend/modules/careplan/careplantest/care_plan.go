package careplantest

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	carePlanPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

type TB interface {
	Helper()
	Fatalf(string, ...any)
}

// NewStoredPickupBaselines composes fixture reads with booking authority off.
func NewStoredPickupBaselines(records carePlanPorts.PickupBaselineRecords, links careplan.ApprovedBookingReader) careplan.PickupBaselineReader {
	baselines, err := carePlanCompose.NewPickupBaselines(records, links, func(context.Context) (bool, error) { return false, nil })
	if err != nil {
		panic(err)
	}
	return baselines
}

// NewArrivalQueries binds schedule reads for integration fixtures that do not
// exercise arrival mutations. The caller supplies its existing owner records.
func NewArrivalQueries(tb TB, db *bun.DB, records careplan.Capability) careplan.ArrivalScheduleService {
	tb.Helper()
	queries, err := carePlanCompose.NewArrivalSchedules(db, records, nil, nil, carePlanCompose.ArrivalScheduleDependencies{})
	if err != nil {
		tb.Fatalf("compose test arrival queries: %v", err)
	}
	return queries
}

// NewPickupQueries binds schedule reads for fixtures without pickup mutations.
func NewPickupQueries(tb TB, db *bun.DB, records careplan.Capability, baselines careplan.PickupBaselineReader) careplan.PickupScheduleService {
	tb.Helper()
	queries, err := carePlanCompose.NewPickupSchedules(db, records, baselines, nil, nil, nil)
	if err != nil {
		tb.Fatalf("compose test pickup queries: %v", err)
	}
	return queries
}

type CareParticipationResolver = carePlanCompose.CareParticipationResolver

// NewCareDays binds native records and the fixture's existing participation source.
func NewCareDays(records careplan.Capability, baselines careplan.PickupBaselineReader, participation carePlanCompose.CareParticipationResolver) careplan.CareDayQuery {
	careDays, err := carePlanCompose.NewCareDayQuery(carePlanCompose.CareDayDependencies{Records: records, PickupBaselines: baselines, CareParticipation: participation})
	if err != nil {
		panic(err)
	}
	return careDays
}

// NewOfferingBookings composes the effective-booking owner for workflow tests.
func NewOfferingBookings() *careplan.OfferingBookings {
	return carePlanCompose.NewOfferingBookings()
}

// NewCarePlan composes the owner capability for integration tests. The
// caller supplies the student directory reader; test support may not reach
// the projection (ADR 0058).
func NewCarePlan(tb TB, db *bun.DB, directory StudentDirectory) careplan.Capability {
	tb.Helper()
	students := newStudentDirectory(tb, db, directory)
	capability, err := carePlanCompose.New(carePlanCompose.Dependencies{
		DB: db, Observe: func(carePlanCompose.Observation) {}, AmbientDB: carePlanCompose.TenantAmbientDatabase(db),
		StatusStudents: newStatusStudentDirectory(db, students, directory), StatusSlots: emptyStatusSlots{}, StudentDirectory: directory,
		People:      studentNameFinder(students),
		StudentLock: students.LockStudent, StudentNotFound: peopledirectory.ErrStudentNotFound,
	})
	if err != nil {
		tb.Fatalf("compose test Care Plan: %v", err)
	}
	return capability
}

type statusStudentDirectory struct {
	students peopledirectory.Capability
	flags    peopledirectory.StudentStatusFlagCapability
	care     careplan.StudentProfileCommands
}

func newStatusStudentDirectory(db *bun.DB, students peopledirectory.Capability, directory carePlanCompose.StudentDirectory) statusStudentDirectory {
	flags, ok := students.(peopledirectory.StudentStatusFlagCapability)
	if !ok {
		panic("test People Directory does not expose status flags")
	}
	care, err := carePlanCompose.NewStudentProfiles(db, directory, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(err)
	}
	return statusStudentDirectory{students: students, flags: flags, care: care}
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
	return d.care.ClearStudentStatusFlags(ctx, ids, status)
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

// StudentDirectory is the student directory projection both Care Plan and
// People Directory read. Test support may not reach the projection, so the
// caller passes it.
type StudentDirectory interface {
	carePlanCompose.StudentDirectory
	StudentModelRows(db bun.IDB, tenantID int64, model any) *bun.SelectQuery
}

func newStudentDirectory(tb TB, db *bun.DB, directory StudentDirectory) *peopledirectory.Module {
	tb.Helper()
	students, err := peopleCompose.New(peopleCompose.Dependencies{
		StudentDirectory: directory,
		DB:               db, Observe: func(peopleCompose.Observation) {},
	})
	if err != nil {
		tb.Fatalf("compose test People Directory: %v", err)
	}
	return students
}

func studentNameFinder(students peopledirectory.Capability) carePlanPorts.StudentNameFinder {
	return studentNames{students: students}
}

type studentNames struct{ students peopledirectory.Capability }

func (n studentNames) ListStudentNamesByID(ctx context.Context, ids []int64) ([]carePlanPorts.StudentName, error) {
	values, err := n.students.ListStudentNamesByID(ctx, ids)
	if err != nil {
		return nil, err
	}
	result := make([]carePlanPorts.StudentName, 0, len(values))
	for _, value := range values {
		result = append(result, carePlanPorts.StudentName{StudentID: value.StudentID, FirstName: value.FirstName, LastName: value.LastName})
	}
	return result, nil
}

// ArrivalStudentRecords is the People projection arrival baselines read.
type ArrivalStudentRecords interface {
	ListStudentRecordsByID(context.Context, []int64) ([]peopledirectory.StudentRecord, error)
}

// ArrivalStudentClasses projects People's student records onto the school
// class names Care Plan's arrival baselines resolve class plans by.
func ArrivalStudentClasses(students ArrivalStudentRecords) func(context.Context, []int64) (map[int64]string, error) {
	return func(ctx context.Context, ids []int64) (map[int64]string, error) {
		rows, err := students.ListStudentRecordsByID(ctx, ids)
		if err != nil {
			return nil, err
		}
		classes := make(map[int64]string, len(rows))
		for _, row := range rows {
			classes[row.ID] = row.SchoolClass
		}
		return classes, nil
	}
}
