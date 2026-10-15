package services

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/uptrace/bun"
)

// The roots below compose Student Presence, its session records and the
// joined Timetable presence reads from the database. Most bind several
// owners' compositions over the same database (Student Presence, Timetable,
// Care Plan, People Directory, Facilities), so no single owner composition
// can host them. The serving root, the CLI roots and other owners' test
// graphs reach them here; the bindings they use stay database-free. They
// leave with the composition root (#2747).

// NewStudentPresence composes the Student Presence owner with its attendance
// rules bound to an unobserved Timetable owner over the same database. The
// serving roots and the retained services compose their presence module
// through it so every attendance rule sees the same planned roster.
func NewStudentPresence(db *bun.DB, observe func(presenceCompose.Observation)) (*studentpresence.Module, error) {
	// The roster reads and the care-day row lock are People Directory reads;
	// they need none of the owner's write owners or audit seams.
	students := peopleCompose.MustNewReads(db)
	rooms, err := facilitiesCompose.NewUnobserved(db)
	if err != nil {
		return nil, err
	}
	capability, err := newPresenceTimetable(db, students, rooms)
	if err != nil {
		return nil, err
	}
	queries, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		return nil, err
	}
	locks, err := carePlanCompose.NewDayLocks(db, students.LockStudent, peopledirectory.ErrStudentNotFound)
	if err != nil {
		return nil, err
	}
	rules := NewPresenceRulesDependencies(capability, queries, locks)
	return presenceCompose.New(presenceCompose.Dependencies{
		DB: db, Observe: observe, Roster: rules.Roster, CarePlan: rules.CarePlan, CareDays: rules.CareDays,
	})
}

// newPresenceTimetable composes the unobserved Timetable owner the
// presence rules read the planned roster from. The session facts come from
// Student Presence over the same database (#2762).
func newPresenceTimetable(db *bun.DB, students peopledirectory.StudentQuery, rooms facilities.Query) (timetable.Capability, error) {
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return nil, err
	}
	return timetableCompose.New(timetableCompose.Dependencies{
		LockStaffAssignment: func(ctx context.Context, staffID int64) error {
			_, err := membership.FindStaffForMutation(ctx, staffID)
			return err
		},
		DB: db,
		Students: timetableCompose.StudentDirectoryFunc(func(ctx context.Context) ([]timetableCompose.TargetStudent, error) {
			values, err := students.ListEnrolledStudents(ctx)
			result := make([]timetableCompose.TargetStudent, 0, len(values))
			for _, value := range values {
				result = append(result, timetableCompose.TargetStudent{
					ID: value.ID, SchoolClass: value.SchoolClass, EducationGroupID: value.GroupID,
					EnrolledUntil: value.EnrolledUntil,
				})
			}
			return result, err
		}),
		Rooms: timetable.RoomDirectoryFunc(func(ctx context.Context, ids []int64) ([]timetable.RoomRef, error) {
			values, err := rooms.LockRoomsByID(ctx, ids)
			result := make([]timetable.RoomRef, 0, len(values))
			for _, value := range values {
				result = append(result, timetable.RoomRef{ID: value.ID, TenantID: value.TenantID})
			}
			return result, err
		}),
		Sessions: NewPresenceFacts(db),
		Observe:  func(timetableCompose.Observation) {},
	})
}

// NewUnloggedStudentPresence composes the bootstrap Student Presence owner
// behind the legacy repository adapters.
func NewUnloggedStudentPresence(db *bun.DB) *studentpresence.Module {
	module, err := NewStudentPresence(db, presenceObserver())
	if err != nil {
		panic(err)
	}
	return module
}

// NewLoggedStudentPresence composes the Student Presence owner with its operations
// logged at debug level.
func NewLoggedStudentPresence(db *bun.DB, logger *slog.Logger) *studentpresence.Module {
	module, err := NewStudentPresence(db, studentPresenceObserver(logger))
	if err != nil {
		panic(err)
	}
	return module
}

// NewPresenceFacts composes the Student Presence queries Timetable's session
// facts read. They need no roster, which is what keeps the two owners from
// composing each other forever.
func NewPresenceFacts(db *bun.DB) timetable.SessionFacts {
	module, err := presenceCompose.New(presenceCompose.Dependencies{DB: db, Observe: presenceObserver()})
	if err != nil {
		panic(err)
	}
	return NewPresenceSessionFacts(module)
}

// NewPresenceSessionRecords builds the Student Presence session records
// without owner directories: the reads that project rooms, activities or
// devices fail loudly, and supervision reads carry no staff members. The
// optional clock fixes the supervision "today".
func NewPresenceSessionRecords(db *bun.DB, clocks ...func() time.Time) *presenceCompose.SessionRecords {
	var now func() time.Time
	if len(clocks) > 0 {
		now = clocks[0]
	}
	return newPresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db, Now: now})
}

// NewSessionRecords builds the Student Presence session records with every
// owner directory bound at construction (#2743).
func NewSessionRecords(db *bun.DB, owners SessionRecordOwners) *presenceCompose.SessionRecords {
	deps := sessionRecordDependencies(owners)
	deps.DB = db
	return newPresenceSessionRecords(deps)
}

func MustPresenceReads(db *bun.DB) timetableCompose.PresenceReads {
	reads, err := timetableCompose.NewPresenceReads(db)
	if err != nil {
		panic(err)
	}
	return reads
}

// NewPartialAbsencePreview composes the Timetable partial-absence preview over
// the People Directory, the Care Plan pickup exceptions and the joined plan
// reads of the database, for the retained services and their tests.
func NewPartialAbsencePreview(db *bun.DB) timetableCompose.PartialAbsencePreview {
	exceptions, err := carePlanCompose.NewExceptionQueries(db, func(carePlanCompose.Observation) {})
	if err != nil {
		panic(fmt.Sprintf("partial absence preview: compose care plan exception queries: %v", err))
	}
	return newPartialAbsencePreview(MustPresenceReads(db), peopleCompose.MustNewReads(db), exceptions)
}

// NewPickupReviewBlocks previews the blocks a pickup change under review
// would excuse over the joined presence reads of the database.
func NewPickupReviewBlocks(db *bun.DB) carePlanCompose.PickupReviewBlocks {
	return PickupReviewBlocks{reads: MustPresenceReads(db)}
}
