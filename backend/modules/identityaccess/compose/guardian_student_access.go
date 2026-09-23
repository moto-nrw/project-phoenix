package compose

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
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
	return guardianStudentAccess{store: newStore(db), observe: observe}, nil
}

type guardianStudentAccess struct {
	store   *postgres.Store
	observe func(Observation)
}

func (a guardianStudentAccess) GrantGuardianStudentAccess(ctx context.Context, grant identityaccess.GuardianStudentAccessGrant) error {
	started := time.Now()
	stats, err := a.store.GrantGuardianStudentAccess(ctx, postgres.GuardianStudentAccessRecord(grant))
	err = mapError(err)
	a.observe(Observation{Operation: "grant_guardian_student_access", Duration: time.Since(started), Stats: stats, Err: err})
	return err
}

func (a guardianStudentAccess) SetGuardianStudentPermissions(ctx context.Context, tenantID, relationshipID int64, permissions json.RawMessage) (bool, error) {
	started := time.Now()
	found, stats, err := a.store.SetGuardianStudentPermissions(ctx, tenantID, relationshipID, permissions)
	err = mapError(err)
	a.observe(Observation{Operation: "set_guardian_student_permissions", Duration: time.Since(started), Stats: stats, Err: err})
	return found, err
}
