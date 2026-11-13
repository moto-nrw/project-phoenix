package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The kind of document per service keeps a Träger to its rows below row
// level security (#4688): the Träger transaction drops its request role, so
// each assertion holds only because the adapter names the Träger in its own
// statement. Removing the filter of DocumentKinds turns the first assertion
// red; the conflict key of SetDocumentKind is the second.
func TestDocumentKindFiltersKeepTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newCareScope(t, db, "Träger Dokumentart A")
	other := newCareScope(t, db, "Träger Dokumentart B")
	store := NewStore()
	rollback := errors.New("roll the document kinds back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.document_kinds (organization_id, service, document_kind)
			VALUES (?, 'holiday', 'fee_notice'), (?, 'care', 'fee_notice')`, other.organization, other.organization)
		require.NoError(t, err)

		kinds, err := store.DocumentKinds(ctx, own.organization)
		require.NoError(t, err)
		assert.Empty(t, kinds, "DocumentKinds reads no choice of another Träger")

		require.NoError(t, store.SetDocumentKind(ctx, own.organization, parentbilling.ServiceHoliday, parentbilling.DocumentKindInvoice))
		kinds, err = store.DocumentKinds(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, kinds, 1)
		assert.Equal(t, parentbilling.DocumentKindInvoice, kinds[parentbilling.ServiceHoliday].DocumentKind)
		assert.NotNil(t, kinds[parentbilling.ServiceHoliday].UpdatedAt)

		var otherKind string
		require.NoError(t, tx.NewRaw(`SELECT document_kind FROM parent_billing.document_kinds WHERE organization_id = ? AND service = 'holiday'`,
			other.organization).Scan(ctx, &otherKind))
		assert.Equal(t, parentbilling.DocumentKindFeeNotice, otherKind, "the other Träger keeps its choice")

		// The schema refuses a kind a service does not issue.
		_, err = tx.ExecContext(ctx, `SAVEPOINT refused`)
		require.NoError(t, err)
		assert.Error(t, store.SetDocumentKind(ctx, own.organization, parentbilling.ServiceCare, parentbilling.DocumentKindConfirmation),
			"the monthly care has no confirmation letter")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT refused`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// TestIssuedDocumentKindNeverChanges: the trigger that refuses every change
// of an issued invoice also holds its kind of document (#4688).
func TestIssuedDocumentKindNeverChanges(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	scope := newCareScope(t, db, "Träger Dokumentart fest")
	rollback := errors.New("roll the invoice back")
	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, scope.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, scope.organization)
		require.NoError(t, err)
		scope = seedCareScope(t, ctx, tx, scope, "Träger Dokumentart fest")
		var kind string
		require.NoError(t, tx.NewRaw(`SELECT document_kind FROM parent_billing.care_invoices WHERE organization_id = ? AND run_id = ?`,
			scope.organization, scope.released).Scan(ctx, &kind))
		assert.Equal(t, parentbilling.DocumentKindInvoice, kind, "an invoice of before keeps its kind")
		_, err = tx.ExecContext(ctx, `SAVEPOINT frozen`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `UPDATE parent_billing.care_invoices SET document_kind = 'fee_notice' WHERE organization_id = ? AND run_id = ?`,
			scope.organization, scope.released)
		assert.Error(t, err, "the kind of an issued invoice never changes")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT frozen`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
