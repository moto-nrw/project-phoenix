package postgres

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The log of the mandate imports keeps a Träger to its own imports also
// below row level security (#6198): the Träger transaction drops its
// request role, so the assertion holds only because SEPAMandateImportEvents
// names the Träger in its own filter. The database refuses an import
// origin on any entry but the creation of an imported mandate.
func TestSEPAMandateImportLogKeepsTheTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	own := newMandateScope(t, db, "Träger Import A")
	other := newMandateScope(t, db, "Träger Import B")
	store := NewStore()
	rollback := errors.New("roll the imports back")
	ownFile, otherFile := "ogs-connect-a.csv", "ogs-connect-b.csv"

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, db))
	err := testpkg.WithinTraeger(ctx, own.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		tx, err := traegerDB(ctx, own.organization)
		require.NoError(t, err)
		own = seedMandateScope(t, ctx, tx, own, "DE98ZZZ09999999999", "SJR-IA-1")
		other = seedMandateScope(t, ctx, tx, other, "DE98ABC09999999999", "SJR-IB-1")
		_, err = tx.ExecContext(ctx, `SET LOCAL session_replication_role = replica`)
		require.NoError(t, err)
		require.NoError(t, store.InsertSEPAMandateImportEvents(ctx, own.organization, 0, ownFile,
			[]parentbilling.ImportedSEPAMandate{{Mandate: parentbilling.SEPAMandate{ID: own.mandate, ContractID: 1}, Line: 2}}))
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_events (organization_id, mandate_id, care_contract_id,
			action, source, import_file, import_line) VALUES (?, ?, 1, 'create', 'import', ?, 2)`, other.organization, other.mandate, otherFile)
		require.NoError(t, err)

		log, err := store.SEPAMandateImportEvents(ctx, own.organization)
		require.NoError(t, err)
		require.Len(t, log, 1, "SEPAMandateImportEvents reads only the Träger's own import")
		assert.Equal(t, ownFile, log[0].File)

		_, err = tx.ExecContext(ctx, `SAVEPOINT origin`)
		require.NoError(t, err)
		_, err = tx.ExecContext(ctx, `INSERT INTO parent_billing.sepa_mandate_events (organization_id, mandate_id, care_contract_id,
			action, source, import_file, import_line) VALUES (?, ?, 1, 'read', 'office', ?, 2)`, own.organization, own.mandate, ownFile)
		require.ErrorContains(t, err, "chk_sepa_mandate_events_import", "only an imported creation carries a file")
		_, err = tx.ExecContext(ctx, `ROLLBACK TO SAVEPOINT origin`)
		require.NoError(t, err)
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}
