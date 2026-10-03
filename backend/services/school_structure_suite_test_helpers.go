package services

import (
	"context"
	"log/slog"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/modules/delivery/application/realtimeevents"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolStructure "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/services/education"
)

// The composition the retained School Structure repository suites
// (database/repositories/education) drive (#2742), composed here so the
// suites import neither the retained repositories nor the legacy composition.

// SchoolStructureRepositorySuiteDependencies are the owner capabilities the
// repository factory reads through.
type SchoolStructureRepositorySuiteDependencies = repositories.TimetableDependencies

// NewSchoolStructureRepositorySuiteGraph is the legacy repository factory
// over the unobserved timetable dependencies, with a second set of those
// owner capabilities for the suites that write through an owner directly.
func NewSchoolStructureRepositorySuiteGraph(db *bun.DB) (*PeopleRepositorySuiteFactory, SchoolStructureRepositorySuiteDependencies) {
	return repositories.NewRetainedRepositoryFactory(db), repositories.NewUnobservedTimetableDependencies(db)
}

// The retained School Structure service suites (services/education) may
// import neither the service package they drive nor the legacy composition
// (#2742). They name the service's vocabulary and compose it through these
// entries, behind api/testutil. Every entry goes with the last suite that
// uses it.

type (
	EducationSuiteService                        = education.Service
	EducationSuiteError                          = education.EducationError
	EducationSuiteClassAssignmentAudit           = education.ClassAssignmentAudit
	SubstitutionSuiteModule                      = education.SubstitutionModule
	SubstitutionSuiteDependencies                = education.SubstitutionDependencies
	SubstitutionSuiteGroupStore                  = education.GroupStore
	SubstitutionSuiteCaller                      = education.Caller
	SubstitutionSuiteOverviewQuery               = education.OverviewQuery
	SubstitutionSuiteOverviewResult              = education.OverviewResult
	SubstitutionSuiteRunningSupervision          = education.RunningSupervision
	SubstitutionSuiteGroupHandover               = education.GroupHandover
	SubstitutionSuiteGroupRef                    = education.GroupRef
	SubstitutionSuiteStaffRef                    = education.StaffRef
	SubstitutionSuiteAssignment                  = education.Assignment
	SubstitutionSuiteAssignmentResult            = education.AssignmentResult
	SubstitutionSuiteGroupHandoverAssignment     = education.GroupHandoverAssignment
	SubstitutionSuiteAdditionalSupervisionAssign = education.AdditionalSupervisionAssignment
	SubstitutionSuiteEndRequest                  = education.EndRequest
)

const (
	SubstitutionSuiteTargetGroupHandover         = education.TargetGroupHandover
	SubstitutionSuiteTargetAdditionalSupervision = education.TargetAdditionalSupervision
)

var (
	EducationSuiteErrGroupNotFound        = education.ErrGroupNotFound
	EducationSuiteErrGroupHasHandover     = education.ErrGroupHasHandover
	EducationSuiteErrStaffNotFound        = education.ErrStaffNotFound
	EducationSuiteErrEmptySchoolClass     = education.ErrEmptySchoolClass
	EducationSuiteErrGroupTeacherNotFound = education.ErrGroupTeacherNotFound
	EducationSuiteErrDuplicateGroup       = education.ErrDuplicateGroup
	EducationSuiteErrGroupHasStudents     = education.ErrGroupHasStudents
	EducationSuiteErrRoomNotFound         = education.ErrRoomNotFound
	EducationSuiteErrTeacherNotFound      = education.ErrTeacherNotFound
	SubstitutionSuiteErrNotFound          = education.ErrNotFound
	SubstitutionSuiteErrForbidden         = education.ErrForbidden
	SubstitutionSuiteErrInvalidTarget     = education.ErrInvalidTarget
	SubstitutionSuiteErrInvalidPeriod     = education.ErrInvalidPeriod
	SubstitutionSuiteErrNotRunning        = education.ErrNotRunning
	SubstitutionSuiteErrAlreadyAssigned   = education.ErrAlreadyAssigned
	SubstitutionSuiteErrSelfAssignment    = education.ErrSelfAssignment
)

// NewEducationSuiteService composes the group service over the repositories
// the way the service factory does, its class assignment audit trail
// included.
func NewEducationSuiteService(repos *PeopleRepositorySuiteFactory, db *bun.DB) education.Service {
	service := education.NewService(repos.Group, repos.GroupTeacher, repos.ClassTeacher,
		repositories.NewEducationRooms(repos.Room), NewEducationTeachers(repos.Teacher),
		repositories.NewEducationStaff(repos.Staff), repos.Student, repos.GroupSubstitution,
		schoolStructure.NewLegacyRepositoryRuntime(db))
	if auditAware, ok := service.(interface {
		SetMasterDataAudit(education.ClassAssignmentAudit)
	}); ok {
		auditAware.SetMasterDataAudit(repositories.NewEducationClassAssignmentAudit(repos.StaffMasterDataChange))
	}
	return service
}

// NewSubstitutionSuiteModule composes the substitution module: the stores,
// the audit trail and the runtime the suite left unset bind to the
// repositories the way the service factory binds them.
func NewSubstitutionSuiteModule(repos *PeopleRepositorySuiteFactory, db *bun.DB, deps education.SubstitutionDependencies) education.SubstitutionModule {
	if deps.Groups == nil {
		deps.Groups = repos.Group
	}
	if deps.Substitutions == nil {
		deps.Substitutions = repositories.NewEducationHandovers(repos.GroupSubstitution)
	}
	if deps.Teachers == nil {
		deps.Teachers = repositories.NewEducationCaregivers(repos.Teacher)
	}
	if deps.Staff == nil {
		deps.Staff = repositories.NewEducationStaff(repos.Staff)
	}
	if deps.Audit == nil {
		deps.Audit = repositories.NewEducationSubstitutionAudit(repos.SubstitutionChange)
	}
	if deps.Runtime == nil {
		deps.Runtime = schoolStructure.NewLegacyRepositoryRuntime(db)
	}
	return education.NewSubstitutionModule(deps)
}

// NewSchoolStructurePeopleSuiteFactory is the repository factory with the
// People Directory bound, so the suites read person-enriched rows like the
// service graph does.
func NewSchoolStructurePeopleSuiteFactory(db *bun.DB) (*PeopleRepositorySuiteFactory, error) {
	return repositories.NewFactoryWithPeopleDirectory(db, repositories.NewUnobservedTimetableDependencies(db))
}

// GradeTransitionSuiteOwners are the owner compositions the grade transition
// workflow suites (services/education, #2711) assemble the workflow over.
type GradeTransitionSuiteOwners struct {
	People               peopledirectory.Capability
	Membership           schoolmembership.Capability
	Rosters              timetable.RosterMaintenance
	LockRecurrenceWrites func(context.Context) error
}

// NewGradeTransitionSuiteOwners composes the owners the production root binds
// the workflow to, with the roster maintenance on the given clock.
func NewGradeTransitionSuiteOwners(db *bun.DB, clock func() time.Time) (GradeTransitionSuiteOwners, error) {
	rows, err := repositories.NewTimetableTestRepositories(db, clock)
	if err != nil {
		return GradeTransitionSuiteOwners{}, err
	}
	people, err := repositories.NewPeopleDirectory(db)
	if err != nil {
		return GradeTransitionSuiteOwners{}, err
	}
	membership, err := repositories.NewSchoolMembership(db)
	if err != nil {
		return GradeTransitionSuiteOwners{}, err
	}
	return GradeTransitionSuiteOwners{
		People: people, Membership: membership,
		Rosters:              rows.RosterMaintenance(slog.Default(), clock),
		LockRecurrenceWrites: repositories.MustNewTimetableRecurrenceLock(db).LockRecurrenceWrites,
	}, nil
}

// SetEducationSuiteBroadcaster swaps the group service's broadcaster, the
// duck-typed wiring the service factory uses, and reports whether the service
// accepts one.
func SetEducationSuiteBroadcaster(service education.Service, broadcaster realtimeevents.Publisher) bool {
	aware, ok := service.(interface {
		SetBroadcaster(realtimeevents.Publisher)
	})
	if ok {
		aware.SetBroadcaster(broadcaster)
	}
	return ok
}
