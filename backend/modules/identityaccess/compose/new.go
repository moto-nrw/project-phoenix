// Package compose wires the Identity & Access module over the shared tenant
// runtime and the Bun database.
package compose

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

type Observation = ports.Observation

type Dependencies struct {
	DB      *bun.DB
	Observe func(Observation)
}

// New composes the Identity & Access module. Guardian operations run on the
// caller's ambient transaction when one exists and otherwise open one for
// the tenant in context, so the guardian-access writes commit with the
// approval that requested them. Operator operations join an ambient
// transaction and otherwise run on the root connection: operators and their
// sessions are platform-wide rows without a tenant.
func New(dependencies Dependencies) (*identityaccess.Module, error) {
	if dependencies.DB == nil || dependencies.Observe == nil {
		return nil, errors.New("identity access compose: all dependencies are required")
	}
	store := postgres.New(func(ctx context.Context) (bun.IDB, error) {
		transaction, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return dependencies.DB, nil
		}
		switch tx := transaction.(type) {
		case bun.Tx:
			return tx, nil
		case *bun.Tx:
			if tx != nil {
				return *tx, nil
			}
			return dependencies.DB, nil
		default:
			return nil, fmt.Errorf("identity access postgres: unsupported transaction %T", transaction)
		}
	})
	service := application.New(store, store, transaction{}, tenant.FromContext, func(observation Observation) {
		observation.Err = mapError(observation.Err)
		dependencies.Observe(observation)
	})
	return identityaccess.NewModule(engine{service: service}), nil
}

type transaction struct{}

func (transaction) RunWrite(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err != nil {
		return fmt.Errorf("%w: %w", identityaccess.ErrTenantRequired, err)
	}
	return tenant.WithinCurrentTenant(ctx, callback)
}

func (transaction) RunRead(ctx context.Context, callback func(context.Context) error) error {
	if _, ok := tenant.TransactionFromContext(ctx); ok {
		return callback(ctx)
	}
	if _, err := tenant.TenantFromContext(ctx); err == nil {
		return tenant.WithinCurrentTenant(ctx, callback)
	}
	return tenant.WithinAdmin(ctx, callback)
}

// RunPlatform never opens a transaction: the operator flows open their own
// administrative transaction where several statements must commit together
// and run single statements on the root connection otherwise.
func (transaction) RunPlatform(ctx context.Context, callback func(context.Context) error) error {
	return callback(ctx)
}

type engine struct{ service *application.Service }

func (e engine) FindAccount(ctx context.Context, id int64) (identityaccess.Account, error) {
	value, err := e.service.FindAccount(ctx, id)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) FindAccountByEmail(ctx context.Context, email string) (identityaccess.Account, error) {
	value, err := e.service.FindAccountByEmail(ctx, email)
	return identityaccess.Account(value), mapError(err)
}

func (e engine) GrantGuardianTenantAccess(ctx context.Context, accountID int64) (identityaccess.GuardianTenantAccess, error) {
	value, err := e.service.GrantGuardianTenantAccess(ctx, accountID)
	return identityaccess.GuardianTenantAccess(value), mapError(err)
}

func (e engine) FindOperator(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperator(ctx, id)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) FindOperatorForUpdate(ctx context.Context, id int64) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorForUpdate(ctx, id)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) FindOperatorByEmail(ctx context.Context, email string) (identityaccess.Operator, error) {
	value, err := e.service.FindOperatorByEmail(ctx, email)
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) ListOperators(ctx context.Context) ([]identityaccess.Operator, error) {
	values, err := e.service.ListOperators(ctx)
	if err != nil {
		return nil, mapError(err)
	}
	result := make([]identityaccess.Operator, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.Operator(value))
	}
	return result, nil
}

func (e engine) CreateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.CreateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) UpdateOperator(ctx context.Context, operator identityaccess.Operator) (identityaccess.Operator, error) {
	value, err := e.service.UpdateOperator(ctx, domain.Operator(operator))
	return identityaccess.Operator(value), mapError(err)
}

func (e engine) DeleteOperator(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteOperator(ctx, id))
}

func (e engine) RecordOperatorLogin(ctx context.Context, id int64) error {
	return mapError(e.service.RecordOperatorLogin(ctx, id))
}

func (e engine) IncrementOperatorMFAAttempts(ctx context.Context, id int64, threshold int, lockout time.Duration) (identityaccess.OperatorMFAAttempts, error) {
	value, err := e.service.IncrementOperatorMFAAttempts(ctx, id, threshold, lockout)
	return identityaccess.OperatorMFAAttempts(value), mapError(err)
}

func (e engine) ResetOperatorMFAAttempts(ctx context.Context, id int64) error {
	return mapError(e.service.ResetOperatorMFAAttempts(ctx, id))
}

func (e engine) FindOperatorSessionForUpdate(ctx context.Context, token string) (identityaccess.OperatorSession, error) {
	value, err := e.service.FindOperatorSessionForUpdate(ctx, token)
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) LatestOperatorSessionInFamily(ctx context.Context, familyID string) (identityaccess.OperatorSession, error) {
	value, err := e.service.LatestOperatorSessionInFamily(ctx, familyID)
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) CreateOperatorSession(ctx context.Context, session identityaccess.OperatorSession) (identityaccess.OperatorSession, error) {
	value, err := e.service.CreateOperatorSession(ctx, domain.OperatorSession(session))
	return identityaccess.OperatorSession(value), mapError(err)
}

func (e engine) MarkOperatorSessionRotated(ctx context.Context, id int64, replacementToken string, recoveryProofHash []byte, rotatedAt time.Time) error {
	return mapError(e.service.MarkOperatorSessionRotated(ctx, id, replacementToken, recoveryProofHash, rotatedAt))
}

func (e engine) DeleteExpiredRotatedOperatorSessions(ctx context.Context, familyID string, now time.Time) error {
	return mapError(e.service.DeleteExpiredRotatedOperatorSessions(ctx, familyID, now))
}

func (e engine) DeleteOperatorSession(ctx context.Context, id int64) error {
	return mapError(e.service.DeleteOperatorSession(ctx, id))
}

func (e engine) RevokeOperatorSessions(ctx context.Context, operatorID int64) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessions(ctx, operatorID)
	return operatorSessions(values), mapError(err)
}

func (e engine) RevokeOperatorSessionFamily(ctx context.Context, familyID string) ([]identityaccess.OperatorSession, error) {
	values, err := e.service.RevokeOperatorSessionFamily(ctx, familyID)
	return operatorSessions(values), mapError(err)
}

func (e engine) DeleteExpiredOperatorSessions(ctx context.Context, now time.Time) (int, error) {
	deleted, err := e.service.DeleteExpiredOperatorSessions(ctx, now)
	return deleted, mapError(err)
}

func operatorSessions(values []domain.OperatorSession) []identityaccess.OperatorSession {
	if values == nil {
		return nil
	}
	result := make([]identityaccess.OperatorSession, 0, len(values))
	for _, value := range values {
		result = append(result, identityaccess.OperatorSession(value))
	}
	return result
}

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrAccountNotFound):
		return identityaccess.ErrAccountNotFound
	case errors.Is(err, domain.ErrTenantRequired):
		return identityaccess.ErrTenantRequired
	case errors.Is(err, domain.ErrGuardianRoleMissing):
		return identityaccess.ErrGuardianRoleMissing
	case errors.Is(err, domain.ErrOperatorNotFound):
		return identityaccess.ErrOperatorNotFound
	case errors.Is(err, domain.ErrOperatorSessionNotFound):
		return identityaccess.ErrOperatorSessionNotFound
	case errors.Is(err, domain.ErrOperatorSessionRotated):
		return identityaccess.ErrOperatorSessionRotated
	default:
		return err
	}
}
