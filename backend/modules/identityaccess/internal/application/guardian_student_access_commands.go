package application

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type guardianAccessCommands struct {
	identityaccess.GuardianStudentAccess
	tx interface {
		RunWrite(context.Context, func(context.Context) error) error
	}
}

// NewGuardianStudentAccessCommands owns the single-owner access command's unit
// of work. A relationship workflow supplies the ambient transaction.
func NewGuardianStudentAccessCommands(store identityaccess.GuardianStudentAccess, tx interface {
	RunWrite(context.Context, func(context.Context) error) error
}) identityaccess.GuardianStudentAccess {
	return guardianAccessCommands{store, tx}
}
func (c guardianAccessCommands) GrantGuardianStudentAccess(ctx context.Context, grant identityaccess.GuardianStudentAccessGrant) error {
	return c.tx.RunWrite(ctx, func(txCtx context.Context) error {
		return c.GuardianStudentAccess.GrantGuardianStudentAccess(txCtx, grant)
	})
}

// GrantGuardianStudentAccesses writes the access halves of many new
// relationships in the caller's transaction, which the relationship workflow
// requires before it calls; it opens none (#5552, ADR 0067 addendum of
// 2026-11-08, point 3). The data import observes the whole guardian batch.
func (c guardianAccessCommands) GrantGuardianStudentAccesses(ctx context.Context, grants []identityaccess.GuardianStudentAccessGrant) error {
	store, ok := c.GuardianStudentAccess.(guardianAccessBatchStore)
	if !ok {
		return errGuardianAccessBatchesUnbound
	}
	rows := make([]domain.GuardianStudentAccessGrant, len(grants))
	for i, grant := range grants {
		rows[i] = domain.GuardianStudentAccessGrant(grant)
	}
	_, err := store.InsertGuardianStudentAccesses(ctx, rows)
	return MapError(err)
}

// guardianAccessBatchStore is the batch insert of the access store the
// command binds.
type guardianAccessBatchStore interface {
	InsertGuardianStudentAccesses(context.Context, []domain.GuardianStudentAccessGrant) (domain.OperationStats, error)
}

var _ identityaccess.GuardianStudentAccessBatches = guardianAccessCommands{}

var errGuardianAccessBatchesUnbound = errors.New("identity access: the guardian access store has no batch grant")

func (c guardianAccessCommands) SetGuardianStudentPermissions(ctx context.Context, schoolID, relationshipID int64, permissions json.RawMessage) (found bool, err error) {
	err = c.tx.RunWrite(ctx, func(txCtx context.Context) error {
		found, err = c.GuardianStudentAccess.SetGuardianStudentPermissions(txCtx, schoolID, relationshipID, permissions)
		return err
	})
	return found, err
}
