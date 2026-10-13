package test

import (
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// Suites that may import neither the retained People Directory models nor
// the authorization runtime name the rows, values and errors they arrange and
// assert through this test support. The retained People Directory repository
// suites name the models themselves from the People Directory contract suite
// (#3966). Every entry goes with the last suite that uses it.

type (
	Student         = users.Student
	StudentGuardian = users.StudentGuardian
	StudentStatus   = users.StudentStatus
	Teacher         = users.Teacher
)

const (
	StudentStatusActive                = users.StudentStatusActive
	StudentStatusAlumnus               = users.StudentStatusAlumnus
	StudentStatusInactive              = users.StudentStatusInactive
	StudentStatusPending               = users.StudentStatusPending
	GuardianPermissionEnrollmentSubmit = securityruntime.GuardianPermissionEnrollmentSubmit
	GuardianPermissionPollResponse     = securityruntime.GuardianPermissionPollResponse
	GuardianPermissionPortalAccess     = securityruntime.GuardianPermissionPortalAccess
	GuardianRoleLegalGuardian          = securityruntime.GuardianRoleLegalGuardian
	GuardianRolePickupOnly             = securityruntime.GuardianRolePickupOnly
	GuardianRoleSocialWorker           = securityruntime.GuardianRoleSocialWorker
)

var (
	NewQueryOptions = users.NewQueryOptions
	NewQueryFilter  = users.NewQueryFilter
)
