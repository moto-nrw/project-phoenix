package owners

import (
	"fmt"
	"log/slog"
	"time"

	studentDirectoryView "github.com/moto-nrw/project-phoenix/modules/studentdirectoryview/compose"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewStudentPresence composes the Student Presence owner with its attendance
// rules bound to an unobserved Timetable owner over the same database. Every
// root composes its presence module through it so every attendance rule sees
// the same planned roster.
func NewStudentPresence(db transactionruntime.Database, observe func(presenceCompose.Observation)) (*studentpresence.Module, error) {
	pool := db.Pool()
	// The roster reads and the care-day row lock are People Directory reads;
	// they need none of the owner's write owners or audit seams.
	students := peopleCompose.MustNewReads(pool, studentDirectoryView.NewReader())
	rooms, err := facilitiesCompose.NewUnobserved(pool)
	if err != nil {
		return nil, err
	}
	capability, err := NewUnobservedTimetable(db, students, rooms)
	if err != nil {
		return nil, err
	}
	queries, err := carePlanCompose.NewExceptionQueries(pool, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	locks, err := carePlanCompose.NewDayLocks(pool, students.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		return nil, err
	}
	rules := NewPresenceRulesDependencies(capability, queries, locks)
	return presenceCompose.New(presenceCompose.Dependencies{
		DB: pool, Observe: observe, Roster: rules.Roster, CarePlan: rules.CarePlan, CareDays: rules.CareDays,
	})
}

// NewUnloggedStudentPresence composes the Student Presence owner that logs
// only its failed operations.
func NewUnloggedStudentPresence(db transactionruntime.Database) *studentpresence.Module {
	module, err := NewStudentPresence(db, presenceCompose.FailureLogger())
	if err != nil {
		panic(err)
	}
	return module
}

// NewLoggedStudentPresence composes the Student Presence owner with its
// operations logged at debug level.
func NewLoggedStudentPresence(db transactionruntime.Database, logger *slog.Logger) *studentpresence.Module {
	module, err := NewStudentPresence(db, presenceCompose.DebugLogger(logger))
	if err != nil {
		panic(err)
	}
	return module
}

// NewPresenceFacts composes the Student Presence queries Timetable's session
// facts read. They need no roster, which is what keeps the two owners from
// composing each other forever.
func NewPresenceFacts(db transactionruntime.Database) timetable.SessionFacts {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db.Pool(), Observe: presenceCompose.FailureLogger()})
	if err != nil {
		panic(err)
	}
	return timetableCompose.NewPresenceSessionFacts(module)
}

// NewSessionRecords builds the Student Presence session records with every
// owner directory bound at construction (#2743): the supervision clock (nil
// is the Berlin clock), the devices (nil fails device reads loudly), the
// rooms, the activities, the staff memberships and their persons.
func NewSessionRecords(db transactionruntime.Database, now func() time.Time, devices devicefleet.Query, rooms facilitiesModule.Query, activities timetable.Capability, membership schoolmembership.Query, persons peopledirectory.Query) presenceCompose.SessionRecords {
	deps := sessionRecordDependencies(now, devices, rooms, activities, membership, persons)
	deps.DB = db.Pool()
	return ComposePresenceSessionRecords(deps)
}

// MustPresenceReads composes the joined Timetable presence reads.
func MustPresenceReads(db transactionruntime.Database) timetableCompose.PresenceReads {
	reads, err := timetableCompose.NewPresenceReads(NewTimetablePresenceProjection(db))
	if err != nil {
		panic(err)
	}
	return reads
}

// NewPartialAbsencePreview composes the Timetable partial-absence preview over
// the People Directory, the Care Plan pickup exceptions and the joined plan
// reads of the database.
func NewPartialAbsencePreview(db transactionruntime.Database) timetableCompose.PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db.Pool(), func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	return newPartialAbsencePreview(MustPresenceReads(db), peopleCompose.MustNewReads(db.Pool(), studentDirectoryView.NewReader()), exceptions)
}
