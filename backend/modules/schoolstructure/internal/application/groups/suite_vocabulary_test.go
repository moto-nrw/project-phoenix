package groups_test

import (
	schoolStructureCompose "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
)

// The suites of the retained School Structure services name the service's
// vocabulary through School Structure's composition (#2742, #4005); these
// local names keep the suites readable.

type (
	Service                         = schoolStructureCompose.Service
	EducationError                  = schoolStructureCompose.EducationError
	SubstitutionModule              = schoolStructureCompose.SubstitutionModule
	SubstitutionDependencies        = schoolStructureCompose.SubstitutionDependencies
	GroupStore                      = schoolStructureCompose.GroupStore
	SubstitutionCaller              = schoolStructureCompose.Caller
	OverviewQuery                   = schoolStructureCompose.OverviewQuery
	OverviewResult                  = schoolStructureCompose.OverviewResult
	RunningSupervision              = schoolStructureCompose.RunningSupervision
	GroupHandover                   = schoolStructureCompose.GroupHandover
	GroupRef                        = schoolStructureCompose.GroupRef
	StaffRef                        = schoolStructureCompose.StaffRef
	Assignment                      = schoolStructureCompose.Assignment
	AssignmentResult                = schoolStructureCompose.AssignmentResult
	GroupHandoverAssignment         = schoolStructureCompose.GroupHandoverAssignment
	AdditionalSupervisionAssignment = schoolStructureCompose.AdditionalSupervisionAssignment
	EndRequest                      = schoolStructureCompose.EndRequest
)

const (
	TargetGroupHandover         = schoolStructureCompose.TargetGroupHandover
	TargetAdditionalSupervision = schoolStructureCompose.TargetAdditionalSupervision
)

var (
	ErrGroupNotFound    = schoolStructureCompose.ErrGroupNotFound
	ErrGroupHasHandover = schoolStructureCompose.ErrGroupHasHandover
	ErrStaffNotFound    = schoolStructureCompose.ErrStaffNotFound
	ErrEmptySchoolClass = schoolStructureCompose.ErrEmptySchoolClass
	ErrNotFound         = schoolStructureCompose.ErrNotFound
	ErrForbidden        = schoolStructureCompose.ErrForbidden
	ErrInvalidTarget    = schoolStructureCompose.ErrInvalidTarget
	ErrInvalidPeriod    = schoolStructureCompose.ErrInvalidPeriod
	ErrNotRunning       = schoolStructureCompose.ErrNotRunning
	ErrAlreadyAssigned  = schoolStructureCompose.ErrAlreadyAssigned
	ErrSelfAssignment   = schoolStructureCompose.ErrSelfAssignment
)
