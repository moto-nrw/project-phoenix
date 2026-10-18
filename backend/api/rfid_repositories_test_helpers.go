//go:build testhooks

package api

import (
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// NewStudentLookupTestRepository is the retained student repository for suites
// that only look children up.
func NewStudentLookupTestRepository(db transactionruntime.Database) peoplePorts.StudentRepository {
	return NewUnboundStudentRepository(db)
}

// NewGuardianProfileTestRepository is the guardian profile repository with the
// Identity & Access reachability queries bound.
func NewGuardianProfileTestRepository(db transactionruntime.Database) peoplePorts.GuardianProfileRepository {
	return NewGuardianProfileRepository(db)
}
