package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/uptrace/bun"
)

// NewGuardianStudentAccess composes the relationship-access commands (#2756)
// from the database alone: the relationship's unit of work in People
// Directory needs them before the full Identity & Access graph exists. Every
// command joins the caller's tenant or administrative transaction.
func NewGuardianStudentAccess(db *bun.DB, observe func(Observation)) (identityaccess.GuardianStudentAccess, error) {
	if db == nil || observe == nil {
		return nil, errors.New("identity access guardian student access: database and observer are required")
	}
	return application.NewGuardianStudentAccessCommands(retainedGuardianStudentAccess{GuardianStudentAccessStore: postgres.NewStore(db), Observe: observe}, newCommandTransactions(nil)), nil
}
