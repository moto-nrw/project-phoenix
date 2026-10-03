package education_test

import "github.com/moto-nrw/project-phoenix/api/testutil"

// The suites of the retained School Structure services name the service's
// vocabulary through test support (#2742); these local names keep the suites
// readable.

type (
	Service                         = testutil.EducationSuiteService
	EducationError                  = testutil.EducationSuiteError
	SubstitutionModule              = testutil.SubstitutionSuiteModule
	SubstitutionDependencies        = testutil.SubstitutionSuiteDependencies
	GroupStore                      = testutil.SubstitutionSuiteGroupStore
	SubstitutionCaller              = testutil.SubstitutionSuiteCaller
	OverviewQuery                   = testutil.SubstitutionSuiteOverviewQuery
	OverviewResult                  = testutil.SubstitutionSuiteOverviewResult
	RunningSupervision              = testutil.SubstitutionSuiteRunningSupervision
	GroupHandover                   = testutil.SubstitutionSuiteGroupHandover
	GroupRef                        = testutil.SubstitutionSuiteGroupRef
	StaffRef                        = testutil.SubstitutionSuiteStaffRef
	Assignment                      = testutil.SubstitutionSuiteAssignment
	AssignmentResult                = testutil.SubstitutionSuiteAssignmentResult
	GroupHandoverAssignment         = testutil.SubstitutionSuiteGroupHandoverAssignment
	AdditionalSupervisionAssignment = testutil.SubstitutionSuiteAdditionalSupervisionAssign
	EndRequest                      = testutil.SubstitutionSuiteEndRequest
)

const (
	TargetGroupHandover         = testutil.SubstitutionSuiteTargetGroupHandover
	TargetAdditionalSupervision = testutil.SubstitutionSuiteTargetAdditionalSupervision
)

var (
	ErrGroupNotFound    = testutil.EducationSuiteErrGroupNotFound
	ErrGroupHasHandover = testutil.EducationSuiteErrGroupHasHandover
	ErrStaffNotFound    = testutil.EducationSuiteErrStaffNotFound
	ErrEmptySchoolClass = testutil.EducationSuiteErrEmptySchoolClass
	ErrNotFound         = testutil.SubstitutionSuiteErrNotFound
	ErrForbidden        = testutil.SubstitutionSuiteErrForbidden
	ErrInvalidTarget    = testutil.SubstitutionSuiteErrInvalidTarget
	ErrInvalidPeriod    = testutil.SubstitutionSuiteErrInvalidPeriod
	ErrNotRunning       = testutil.SubstitutionSuiteErrNotRunning
	ErrAlreadyAssigned  = testutil.SubstitutionSuiteErrAlreadyAssigned
	ErrSelfAssignment   = testutil.SubstitutionSuiteErrSelfAssignment
)
