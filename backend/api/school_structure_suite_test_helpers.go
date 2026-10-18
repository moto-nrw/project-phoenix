//go:build testhooks

package api

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	education "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/workflows/shiftplansync"
)

// The composition the retained School Structure repository suites
// (database/repositories/education) drive (#2742), composed here so the
// suites import neither the retained repositories nor the service graph.

// NewSchoolStructureRepositorySuiteGraph is the legacy repository factory
// over the unobserved owners; the suites that write through an owner
// directly compose it with MustNewSchoolMembership or MustNewWorkforce.
func NewSchoolStructureRepositorySuiteGraph(db transactionruntime.Database) *PeopleRepositorySuiteFactory {
	return NewPeopleRepositorySuiteFactory(db)
}

// The retained School Structure service suites (services/education) may
// import neither the service package they drive nor the service graph
// (#2742). They compose the service through these entries and name its
// vocabulary through School Structure's composition (#4005). Every entry goes
// with the last suite that uses it.

// The School Structure values the suites arrange and assert (#3556): the
// owner's domain values through its composition, and the Workforce row of a
// group substitution through the retained repository contract.
type (
	SchoolStructureSuiteGroupTeacher    = schoolmembership.GroupAssignment
	SchoolStructureSuiteClassTeacher    = schoolmembership.ClassAssignment
	SchoolStructureSuiteSubstitutionRow = GroupSubstitutionRow
)

// NewEducationSuiteService composes the group service over the repositories
// the way the service graph does, its class assignment audit trail
// included.
func NewEducationSuiteService(repos *PeopleRepositorySuiteFactory, db transactionruntime.Database, broadcasters ...deliveryCompose.RealtimePublisher) education.Service {
	var broadcaster deliveryCompose.RealtimePublisher
	if len(broadcasters) > 0 {
		broadcaster = broadcasters[0]
	}
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		panic(fmt.Sprintf("education suite: compose facilities: %v", err))
	}
	structure := NewSchoolStructureSuiteRepositories(db)
	service := education.NewService(structure.Group, structure.GroupTeacher, structure.ClassTeacher,
		NewEducationRooms(rooms), NewEducationTeachers(repos.Teacher),
		NewEducationStaff(repos.Staff), repos.Student, repos.GroupSubstitution,
		education.NewLegacyRepositoryRuntime(db.Pool()), education.GroupServiceOptions{Broadcaster: NewGroupEvents(broadcaster, nil), Audit: NewEducationClassAssignmentAudit(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).StaffMasterDataChange)})

	return service
}

// NewTestGroupHandovers binds the handover writes of the shift-plan-sync
// workflow to an unobserved Workforce owner on db.
func NewTestGroupHandovers(db transactionruntime.Database) shiftplansync.GroupHandovers {
	workTime, err := NewUnobservedWorkforce(db)
	if err != nil {
		panic(fmt.Sprintf("group handovers: compose workforce: %v", err))
	}
	return shiftplansync.NewGroupHandovers(workTime)
}

// NewSubstitutionSuiteModule composes the substitution module: the stores,
// the audit trail and the runtime the suite left unset bind to the
// repositories the way the service graph binds them.
func NewSubstitutionSuiteModule(repos *PeopleRepositorySuiteFactory, db transactionruntime.Database, deps education.SubstitutionDependencies) education.SubstitutionModule {
	if deps.Groups == nil {
		deps.Groups = NewSchoolStructureSuiteRepositories(db).Group
	}
	if deps.Substitutions == nil {
		deps.Substitutions = NewEducationHandovers(repos.GroupSubstitution, NewTestGroupHandovers(db))
	}
	if deps.Teachers == nil {
		deps.Teachers = NewEducationCaregivers(repos.Teacher)
	}
	if deps.Staff == nil {
		deps.Staff = NewEducationStaff(repos.Staff)
	}
	if deps.ExternalCaregivers == nil {
		deps.ExternalCaregivers = NewEducationExternalCaregivers(repos.Staff, repos.Guest)
	}
	if deps.Audit == nil {
		deps.Audit = NewEducationSubstitutionAudit(auditCompose.MustNewPersistence(auditCompose.PersistenceDependencies{DB: db.Pool()}).SubstitutionChange)
	}
	if deps.Runtime == nil {
		deps.Runtime = education.NewLegacyRepositoryRuntime(db.Pool())
	}
	return education.NewSubstitutionModule(deps)
}

// NewSchoolStructurePeopleSuiteFactory is the repository factory with the
// People Directory bound, so the suites read person-enriched rows like the
// service graph does.
func NewSchoolStructurePeopleSuiteFactory(db transactionruntime.Database) (*PeopleRepositorySuiteFactory, error) {
	return NewPeopleRepositorySuiteFactory(db), nil
}

// GradeTransitionSuiteOwners are the Timetable owner's roster maintenance
// and recurrence gate the grade transition workflow suites
// (services/education, #2711) assemble the workflow over.
type GradeTransitionSuiteOwners struct {
	Rosters              timetable.RosterMaintenance
	LockRecurrenceWrites func(context.Context) error
}

// NewGradeTransitionSuiteOwners composes the owners the production root binds
// the workflow to, with the roster maintenance on the given clock: the
// Timetable side, the People Directory and the School Membership owner.
func NewGradeTransitionSuiteOwners(db transactionruntime.Database, clock func() time.Time) (GradeTransitionSuiteOwners, peopledirectory.Capability, schoolmembership.Capability, error) {
	rows, err := NewTimetableTestRepositories(db, clock)
	if err != nil {
		return GradeTransitionSuiteOwners{}, nil, nil, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return GradeTransitionSuiteOwners{}, nil, nil, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return GradeTransitionSuiteOwners{}, nil, nil, err
	}
	return GradeTransitionSuiteOwners{
		Rosters:              rows.RosterMaintenance(slog.Default(), clock),
		LockRecurrenceWrites: MustNewTimetableRecurrenceLock(db).LockRecurrenceWrites,
	}, people, membership, nil
}

// NewScheduleSubstitutionSuiteModule constructs only the module's schedule
// port for the shift-plan-sync integration suites, without the service graph.
func NewScheduleSubstitutionSuiteModule(schedule education.ScheduleAdapter, runtime education.Runtime, logger *slog.Logger) education.SubstitutionModule {
	return education.NewSubstitutionModule(education.SubstitutionDependencies{Schedule: schedule, Runtime: runtime, Logger: logger})
}
