// Package compose builds the school search projection (ADR 0227). It reads in
// the caller's school transaction and never opens one of its own.
package compose

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/schoolsearchview/internal/application"
	"github.com/moto-ogs/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// New builds the search over Enrollment's answer search, which the root binds.
func New(answers schoolsearchview.StudentAnswerSearch) (schoolsearchview.Search, error) {
	return application.New(postgres.New(schoolTransaction), answers)
}

// schoolTransaction resolves the tenant transaction of the request and its
// school. A read without one is a wiring error, never an unscoped read.
func schoolTransaction(ctx context.Context) (bun.IDB, int64, error) {
	id, err := tenant.TenantFromContext(ctx)
	if err != nil {
		return nil, 0, err
	}
	raw, ok := tenant.TransactionFromContext(ctx)
	if !ok {
		return nil, 0, errors.New("school search view: school transaction is required")
	}
	tx, ok := raw.(bun.Tx)
	if !ok {
		return nil, 0, fmt.Errorf("school search view: unsupported transaction %T", raw)
	}
	return tx, id.Int64(), nil
}
