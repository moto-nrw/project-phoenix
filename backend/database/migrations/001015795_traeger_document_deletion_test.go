package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestTraegerDocumentDeletionKeepsTheRowAndEndsTheAccess pins migration
// 1.15.795 (#5993): under the request role the Träger deletes its document
// once, by removing the file and marking who deleted it when; a deleted
// document keeps no file, changes no further and takes no new release; the
// schools' function answers for no deleted document even while a release
// still names it; the rows that name the document stay.
func TestTraegerDocumentDeletionKeepsTheRowAndEndsTheAccess(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	slug := fmt.Sprintf("traeger-loeschen-%d", testpkg.UniqueTestTenantID(t))
	own := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Löschen", Slug: slug}).ID
	testpkg.OwnTraegerDocuments(t, db, own)
	school := testpkg.CreateTestSchoolInOrganization(t, db, own, "Schule Löschen")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("loeschen-%d@example.org", testpkg.UniqueTestTenantID(t)))

	asRequestRole := func(setting string, value int64, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, fmt.Sprint(value)); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	asTraeger := func(statement string, args ...any) error {
		return asRequestRole("app.current_org_id", own, func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, statement, args...)
			return err
		})
	}
	var document int64
	require.NoError(t, asRequestRole("app.current_org_id", own, func(tx bun.Tx) error {
		return tx.NewRaw(`INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content, created_by)
			VALUES (?, 'Hausordnung', 'hausordnung.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea, ?) RETURNING id`, own, account.ID).Scan(ctx, &document)
	}))
	require.NoError(t, asTraeger(`INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience) VALUES (?, ?, 'schools')`, own, document))
	visible := func() []int64 {
		var ids []int64
		require.NoError(t, asRequestRole("app.current_tenant_id", school, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT document_id FROM platform.traeger_school_document_releases()`).Scan(ctx, &ids)
		}))
		return ids
	}
	require.Equal(t, []int64{document}, visible())

	assert.ErrorContains(t, asTraeger(`UPDATE platform.traeger_documents SET deleted_at = NOW(), deleted_by = ? WHERE id = ?`, account.ID, document),
		"never changes", "a deletion always removes the file")
	assert.ErrorContains(t, asTraeger(`UPDATE platform.traeger_documents SET content = '%PDF-1.5'::bytea WHERE id = ?`, document),
		"never changes", "the file of a document is never replaced")
	assert.Equal(t, "42501", testpkg.PostgresSQLState(asTraeger(`UPDATE platform.traeger_documents SET content = NULL, deleted_at = NOW(), title = 'anders' WHERE id = ?`, document)), "the request role changes nothing but the deletion's columns")
	_, err := db.ExecContext(ctx, `UPDATE platform.traeger_documents SET content = NULL, deleted_at = NOW(), title = 'anders' WHERE id = ?`, document)
	assert.ErrorContains(t, err, "never changes", "not even the migration role changes more than the deletion")
	require.NoError(t, asTraeger(`UPDATE platform.traeger_documents SET content = NULL, deleted_at = NOW(), deleted_by = ? WHERE id = ?`, account.ID, document),
		"the request role deletes its Träger's document")

	assert.Empty(t, visible(), "a school sees no deleted document, even through a release not yet revoked")
	assert.ErrorContains(t, asTraeger(`UPDATE platform.traeger_documents SET deleted_at = NOW() WHERE id = ?`, document),
		"never changes", "a document is deleted once")
	assert.ErrorContains(t, asTraeger(`INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience) VALUES (?, ?, 'parents')`, own, document),
		"released for nobody", "a deleted document takes no new release")

	var kept struct {
		Title    string `bun:"title"`
		Deleted  bool   `bun:"deleted"`
		Releases int    `bun:"releases"`
	}
	require.NoError(t, db.NewRaw(`SELECT title, deleted_at IS NOT NULL AND deleted_by = created_by AS deleted,
		(SELECT COUNT(*) FROM platform.traeger_document_releases WHERE document_id = d.id) AS releases
		FROM platform.traeger_documents AS d WHERE id = ?`, document).Scan(ctx, &kept))
	assert.Equal(t, "Hausordnung", kept.Title, "the row stays as the referent of the audit")
	assert.True(t, kept.Deleted, "who deleted it when stays with the row")
	assert.Equal(t, 1, kept.Releases, "no cascade removes the release")

	_, err = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err, "deleting the account sets the uploading and the deleting account to NULL")
}
