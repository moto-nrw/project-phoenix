package groups_test

import "github.com/moto-nrw/project-phoenix/api"

// The suites of the retained School Structure services name the service's
// vocabulary through test support (#2742); these local names keep the suites
// readable.

type (
	Service                         = api.EducationSuiteService
	EducationError                  = api.EducationSuiteError
	SubstitutionModule              = api.SubstitutionSuiteModule
	SubstitutionDependencies        = api.SubstitutionSuiteDependencies
	GroupStore                      = api.SubstitutionSuiteGroupStore
	SubstitutionCaller              = api.SubstitutionSuiteCaller
	OverviewQuery                   = api.SubstitutionSuiteOverviewQuery
	OverviewResult                  = api.SubstitutionSuiteOverviewResult
	RunningSupervision              = api.SubstitutionSuiteRunningSupervision
	GroupHandover                   = api.SubstitutionSuiteGroupHandover
	GroupRef                        = api.SubstitutionSuiteGroupRef
	StaffRef                        = api.SubstitutionSuiteStaffRef
	Assignment                      = api.SubstitutionSuiteAssignment
	AssignmentResult                = api.SubstitutionSuiteAssignmentResult
	GroupHandoverAssignment         = api.SubstitutionSuiteGroupHandoverAssignment
	AdditionalSupervisionAssignment = api.SubstitutionSuiteAdditionalSupervisionAssign
	EndRequest                      = api.SubstitutionSuiteEndRequest
)

const (
	TargetGroupHandover         = api.SubstitutionSuiteTargetGroupHandover
	TargetAdditionalSupervision = api.SubstitutionSuiteTargetAdditionalSupervision
)

var (
	ErrGroupNotFound    = api.EducationSuiteErrGroupNotFound
	ErrGroupHasHandover = api.EducationSuiteErrGroupHasHandover
	ErrStaffNotFound    = api.EducationSuiteErrStaffNotFound
	ErrEmptySchoolClass = api.EducationSuiteErrEmptySchoolClass
	ErrNotFound         = api.SubstitutionSuiteErrNotFound
	ErrForbidden        = api.SubstitutionSuiteErrForbidden
	ErrInvalidTarget    = api.SubstitutionSuiteErrInvalidTarget
	ErrInvalidPeriod    = api.SubstitutionSuiteErrInvalidPeriod
	ErrNotRunning       = api.SubstitutionSuiteErrNotRunning
	ErrAlreadyAssigned  = api.SubstitutionSuiteErrAlreadyAssigned
	ErrSelfAssignment   = api.SubstitutionSuiteErrSelfAssignment
)
