package testutil

import (
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/services"
)

// The composition the retained School Structure suites (the tests of
// database/repositories/education) drive (#2742), composed in services so the
// suites import neither the retained repositories nor the legacy composition.

// NewSchoolStructureRepositorySuiteFactory is the legacy repository factory
// over the unobserved timetable dependencies.
func NewSchoolStructureRepositorySuiteFactory(db *bun.DB) *services.PeopleRepositorySuiteFactory {
	return services.NewPeopleRepositorySuiteFactory(db)
}

// NewSchoolStructureRepositorySuiteGraph is the same factory together with
// a second set of the owner capabilities it reads through, for suites that
// write through an owner directly.
func NewSchoolStructureRepositorySuiteGraph(db *bun.DB) (*services.PeopleRepositorySuiteFactory, services.SchoolStructureRepositorySuiteDependencies) {
	return services.NewSchoolStructureRepositorySuiteGraph(db)
}

// The retained School Structure service suites (services/education) name the
// service's vocabulary and compose it through these entries (#2742).

type (
	EducationSuiteService                        = services.EducationSuiteService
	EducationSuiteError                          = services.EducationSuiteError
	EducationSuiteClassAssignmentAudit           = services.EducationSuiteClassAssignmentAudit
	SubstitutionSuiteModule                      = services.SubstitutionSuiteModule
	SubstitutionSuiteDependencies                = services.SubstitutionSuiteDependencies
	SubstitutionSuiteGroupStore                  = services.SubstitutionSuiteGroupStore
	SubstitutionSuiteCaller                      = services.SubstitutionSuiteCaller
	SubstitutionSuiteOverviewQuery               = services.SubstitutionSuiteOverviewQuery
	SubstitutionSuiteOverviewResult              = services.SubstitutionSuiteOverviewResult
	SubstitutionSuiteRunningSupervision          = services.SubstitutionSuiteRunningSupervision
	SubstitutionSuiteGroupHandover               = services.SubstitutionSuiteGroupHandover
	SubstitutionSuiteGroupRef                    = services.SubstitutionSuiteGroupRef
	SubstitutionSuiteStaffRef                    = services.SubstitutionSuiteStaffRef
	SubstitutionSuiteAssignment                  = services.SubstitutionSuiteAssignment
	SubstitutionSuiteAssignmentResult            = services.SubstitutionSuiteAssignmentResult
	SubstitutionSuiteGroupHandoverAssignment     = services.SubstitutionSuiteGroupHandoverAssignment
	SubstitutionSuiteAdditionalSupervisionAssign = services.SubstitutionSuiteAdditionalSupervisionAssign
	SubstitutionSuiteEndRequest                  = services.SubstitutionSuiteEndRequest
)

const (
	SubstitutionSuiteTargetGroupHandover         = services.SubstitutionSuiteTargetGroupHandover
	SubstitutionSuiteTargetAdditionalSupervision = services.SubstitutionSuiteTargetAdditionalSupervision
)

var (
	EducationSuiteErrGroupTeacherNotFound = services.EducationSuiteErrGroupTeacherNotFound
	EducationSuiteErrDuplicateGroup       = services.EducationSuiteErrDuplicateGroup
	EducationSuiteErrGroupHasStudents     = services.EducationSuiteErrGroupHasStudents
	EducationSuiteErrRoomNotFound         = services.EducationSuiteErrRoomNotFound
	EducationSuiteErrTeacherNotFound      = services.EducationSuiteErrTeacherNotFound
	EducationSuiteErrGroupNotFound        = services.EducationSuiteErrGroupNotFound
	EducationSuiteErrGroupHasHandover     = services.EducationSuiteErrGroupHasHandover
	EducationSuiteErrStaffNotFound        = services.EducationSuiteErrStaffNotFound
	EducationSuiteErrEmptySchoolClass     = services.EducationSuiteErrEmptySchoolClass
	SubstitutionSuiteErrNotFound          = services.SubstitutionSuiteErrNotFound
	SubstitutionSuiteErrForbidden         = services.SubstitutionSuiteErrForbidden
	SubstitutionSuiteErrInvalidTarget     = services.SubstitutionSuiteErrInvalidTarget
	SubstitutionSuiteErrInvalidPeriod     = services.SubstitutionSuiteErrInvalidPeriod
	SubstitutionSuiteErrNotRunning        = services.SubstitutionSuiteErrNotRunning
	SubstitutionSuiteErrAlreadyAssigned   = services.SubstitutionSuiteErrAlreadyAssigned
	SubstitutionSuiteErrSelfAssignment    = services.SubstitutionSuiteErrSelfAssignment
)

// NewEducationSuiteService composes the group service over the repositories.
func NewEducationSuiteService(repos *services.PeopleRepositorySuiteFactory, db *bun.DB) services.EducationSuiteService {
	return services.NewEducationSuiteService(repos, db)
}

// NewSubstitutionSuiteModule composes the substitution module; unset stores,
// audit trail and runtime bind to the repositories.
func NewSubstitutionSuiteModule(repos *services.PeopleRepositorySuiteFactory, db *bun.DB, deps services.SubstitutionSuiteDependencies) services.SubstitutionSuiteModule {
	return services.NewSubstitutionSuiteModule(repos, db, deps)
}

// NewSchoolStructurePeopleSuiteFactory is the repository factory with the
// People Directory bound.
func NewSchoolStructurePeopleSuiteFactory(db *bun.DB) (*services.PeopleRepositorySuiteFactory, error) {
	return services.NewSchoolStructurePeopleSuiteFactory(db)
}

// GradeTransitionSuiteOwners are the owner compositions the grade transition
// workflow suites assemble the workflow over.
type GradeTransitionSuiteOwners = services.GradeTransitionSuiteOwners

// NewGradeTransitionSuiteOwners composes those owners on the given clock.
func NewGradeTransitionSuiteOwners(db *bun.DB, clock func() time.Time) (GradeTransitionSuiteOwners, error) {
	return services.NewGradeTransitionSuiteOwners(db, clock)
}

// SetEducationSuiteBroadcaster swaps the group service's broadcaster for the
// recording one and reports whether the service accepts one.
func SetEducationSuiteBroadcaster(service EducationSuiteService, broadcaster *testpkg.RecordingBroadcaster) bool {
	return services.SetEducationSuiteBroadcaster(service, broadcaster)
}
