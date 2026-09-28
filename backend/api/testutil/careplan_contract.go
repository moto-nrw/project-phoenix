package testutil

import "github.com/moto-nrw/project-phoenix/services"

// Care Plan's contract suite drives the enrollment intake and decision flow
// (#3565) through these bindings.

// EnrollmentSettingKeys names the tenant setting keys a suite's settings
// double answers.
var EnrollmentSettingKeys = services.CarePlanContractKeys

// NewEnrollmentGuardianAccess composes the Identity & Access capability an
// approval recognises a parent's portal account through.
var NewEnrollmentGuardianAccess = services.NewCarePlanContractGuardianAccess
