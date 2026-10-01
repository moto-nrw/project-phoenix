package test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// TransactionProbe writes one throwaway guardian profile through the tenant
// transaction of a request context, so a test can tell afterwards whether
// that transaction committed or rolled back.
type TransactionProbe struct {
	db *bun.DB
	id int64
}

// NewTransactionProbe returns a probe that has written nothing yet; a written
// row is removed at cleanup.
func NewTransactionProbe(tb testing.TB, db *bun.DB) *TransactionProbe {
	tb.Helper()
	probe := &TransactionProbe{db: db}
	tb.Cleanup(func() {
		if probe.id > 0 {
			_, _ = db.NewDelete().TableExpr(`users.guardian_profiles`).Where("id = ?", probe.id).Exec(context.Background())
		}
	})
	return probe
}

// Write inserts the probe row through the tenant transaction on ctx.
func (p *TransactionProbe) Write(ctx context.Context) error {
	raw, _ := tenant.TransactionFromContext(ctx)
	tx, ok := raw.(bun.IDB)
	if !ok {
		return errors.New("transaction probe: the tenant transaction is required")
	}
	email := fmt.Sprintf(testEmailFormat, "transaction-probe", uniqueFixtureSuffix())
	profile := &users.GuardianProfile{
		FirstName:              "Transaction",
		LastName:               "Probe",
		Email:                  &email,
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	profile.SetTenantID(tenant.FromContext(ctx))
	if err := tx.NewInsert().Model(profile).ModelTableExpr(`users.guardian_profiles`).Scan(ctx); err != nil {
		return fmt.Errorf("transaction probe: %w", err)
	}
	p.id = profile.ID
	return nil
}

// Written reports whether Write inserted a row inside its transaction.
func (p *TransactionProbe) Written() bool { return p.id > 0 }

// Committed reports whether the written row survived its transaction.
func (p *TransactionProbe) Committed(tb testing.TB) bool {
	tb.Helper()
	if p.id == 0 {
		return false
	}
	exists, err := p.db.NewSelect().TableExpr(`users.guardian_profiles`).Where("id = ?", p.id).Exists(context.Background())
	require.NoError(tb, err)
	return exists
}
