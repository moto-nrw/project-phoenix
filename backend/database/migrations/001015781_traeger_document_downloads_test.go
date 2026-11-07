package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestTraegerDocumentDownloadsStayWithTheirTraeger pins the Träger policy of
// audit.traeger_document_downloads (#4692, ADR 0157): under the request role a
// Träger writes and reads only its own downloads; another Träger sees none of
// them and cannot write one in its name.
func TestTraegerDocumentDownloadsStayWithTheirTraeger(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-download-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerDocuments(t, db, organization)
		return organization
	}
	own, other := newTraeger("Träger Download Eigen"), newTraeger("Träger Download Fremd")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("download-%d@example.org", testpkg.UniqueTestTenantID(t)))
	var document int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
		VALUES (?, 'Elternliste', 'liste.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea) RETURNING id`, own).Scan(ctx, &document))

	asTraeger := func(organization int64, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config('app.current_org_id', ?, true)`, fmt.Sprint(organization)); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	record := func(organization, forTraeger int64) error {
		return asTraeger(organization, func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO audit.traeger_document_downloads (organization_id, document_id, actor_account_id, client_ip)
				VALUES (?, ?, ?, '203.0.113.9')`, forTraeger, document, account.ID)
			return err
		})
	}
	visible := func(organization int64) int {
		var n int
		require.NoError(t, asTraeger(organization, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM audit.traeger_document_downloads WHERE organization_id = ?`, own).Scan(ctx, &n)
		}))
		return n
	}

	require.NoError(t, record(own, own), "the Träger records its own download")
	assert.Equal(t, 1, visible(own), "the Träger reads its own download")
	assert.Zero(t, visible(other), "another Träger sees none of them")
	assert.Error(t, record(other, own), "another Träger cannot write a download in this Träger's name")
	assert.Equal(t, 1, visible(own))
}
