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

// TestTraegerSharedDocumentsReachOnlyTheReleasedAudience pins #4692 on the
// shared documents of a Träger: under the request role a Träger reads and
// writes only its own documents; a school transaction reads no row of the
// tables and reaches a document only through an active release that covers
// it; revoking the release ends that at once; opening a document records the
// read; a release never names a school of another Träger, is only revoked,
// once, and a document never changes.
func TestTraegerSharedDocumentsReachOnlyTheReleasedAudience(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-dokument-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerDocuments(t, db, organization)
		return organization
	}
	own, foreign := newTraeger("Träger Dokument Eigen"), newTraeger("Träger Dokument Fremd")
	released := testpkg.CreateTestSchoolInOrganization(t, db, own, "Schule mit Freigabe")
	sibling := testpkg.CreateTestSchoolInOrganization(t, db, own, "Schule ohne Freigabe")
	foreignSchool := testpkg.CreateTestSchoolInOrganization(t, db, foreign, "Fremde Schule")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("dokument-%d@example.org", testpkg.UniqueTestTenantID(t)))

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
	insertDocument := func(organization int64, title string) (int64, error) {
		var id int64
		err := asRequestRole("app.current_org_id", organization, func(tx bun.Tx) error {
			return tx.NewRaw(`INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
				VALUES (?, ?, 'hausordnung.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea) RETURNING id`, organization, title).Scan(ctx, &id)
		})
		return id, err
	}
	document, err := insertDocument(own, "Hausordnung")
	require.NoError(t, err, "the request role adds a document of its Träger")
	assert.Error(t, asRequestRole("app.current_org_id", foreign, func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
			VALUES (?, 'Fremd', 'f.pdf', 'application/pdf', 8, '%PDF-1.4'::bytea)`, own)
		return err
	}), "another Träger adds no document for this one")

	release := func(organization int64, audience string, school any) (int64, error) {
		var id int64
		err := asRequestRole("app.current_org_id", organization, func(tx bun.Tx) error {
			return tx.NewRaw(`INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience, school_id, released_by)
				VALUES (?, ?, ?, ?, ?) RETURNING id`, organization, document, audience, school, account.ID).Scan(ctx, &id)
		})
		return id, err
	}
	_, err = release(own, "schools", foreignSchool)
	assert.ErrorContains(t, err, "does not belong", "a release never names a school of another Träger")
	schoolRelease, err := release(own, "schools", released)
	require.NoError(t, err)
	_, err = release(own, "schools", released)
	assert.Error(t, err, "one active release exists per document and audience")

	countRows := func(setting string, value int64, table string) int {
		var n int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM `+table+` WHERE organization_id = ?`, own).Scan(ctx, &n)
		}))
		return n
	}
	assert.Equal(t, 1, countRows("app.current_org_id", own, "platform.traeger_documents"))
	assert.Zero(t, countRows("app.current_org_id", foreign, "platform.traeger_documents"), "another Träger sees nothing")
	assert.Zero(t, countRows("app.current_tenant_id", released, "platform.traeger_documents"), "a school reads no row of the table")
	assert.Zero(t, countRows("app.current_tenant_id", released, "platform.traeger_document_releases"), "a school reads no release row")

	visible := func(school int64) []int64 {
		var ids []int64
		require.NoError(t, asRequestRole("app.current_tenant_id", school, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT document_id FROM platform.traeger_school_document_releases()`).Scan(ctx, &ids)
		}))
		return ids
	}
	assert.Equal(t, []int64{document}, visible(released), "the released school sees the document")
	assert.Empty(t, visible(sibling), "another school of the Träger does not")
	assert.Empty(t, visible(foreignSchool), "a school of another Träger does not")
	var outside []int64
	require.NoError(t, asRequestRole("app.current_org_id", own, func(tx bun.Tx) error {
		return tx.NewRaw(`SELECT document_id FROM platform.traeger_school_document_releases()`).Scan(ctx, &outside)
	}))
	assert.Empty(t, outside, "the functions answer nothing outside a school transaction")

	open := func(school int64) (string, bool) {
		var name string
		var found bool
		require.NoError(t, asRequestRole("app.current_tenant_id", school, func(tx bun.Tx) error {
			rows, err := tx.QueryContext(ctx, `SELECT file_name FROM platform.open_traeger_school_document(?, ?)`, document, account.ID)
			if err != nil {
				return err
			}
			defer func() { _ = rows.Close() }()
			if rows.Next() {
				found = true
				if err := rows.Scan(&name); err != nil {
					return err
				}
			}
			return rows.Err()
		}))
		return name, found
	}
	name, found := open(released)
	assert.True(t, found)
	assert.Equal(t, "hausordnung.pdf", name)
	_, found = open(sibling)
	assert.False(t, found, "an uncovered school opens nothing")
	assert.Equal(t, 1, countRows("app.current_org_id", own, "platform.traeger_document_reads"), "only the covered opening is recorded")

	// A school's opening deletes its Träger's reads older than three years,
	// also where no parent ever opens a document; another Träger's stay.
	foreignDocument, err := insertDocument(foreign, "Fremde Hausordnung")
	require.NoError(t, err)
	for _, old := range [][2]int64{{own, document}, {foreign, foreignDocument}} {
		_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_document_reads (organization_id, document_id, reader, read_at)
			VALUES (?, ?, 'school', NOW() - INTERVAL '3 years 1 day')`, old[0], old[1])
		require.NoError(t, err)
	}
	_, found = open(released)
	require.True(t, found)
	oldReads := func(organization int64) int {
		var n int
		require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.traeger_document_reads
			WHERE organization_id = ? AND read_at < NOW() - INTERVAL '3 years'`, organization).Scan(ctx, &n))
		return n
	}
	assert.Zero(t, oldReads(own), "a school's opening prunes the Träger's old reads")
	assert.Equal(t, 1, oldReads(foreign), "and never another Träger's")
	assert.Equal(t, 2, countRows("app.current_org_id", own, "platform.traeger_document_reads"))

	// A release for the parents of all schools covers every school of the
	// Träger, and the school sees it too.
	parentsRelease, err := release(own, "parents", nil)
	require.NoError(t, err)
	assert.Equal(t, []int64{document}, visible(sibling))

	revoke := func(id int64) error {
		return asRequestRole("app.current_org_id", own, func(tx bun.Tx) error {
			_, err := tx.ExecContext(ctx, `UPDATE platform.traeger_document_releases SET revoked_at = NOW(), revoked_by = ? WHERE id = ?`, account.ID, id)
			return err
		})
	}
	require.NoError(t, revoke(parentsRelease))
	require.NoError(t, revoke(schoolRelease))
	assert.Empty(t, visible(released), "revoking ends the access at once")
	_, found = open(released)
	assert.False(t, found)
	assert.ErrorContains(t, revoke(schoolRelease), "only revoked, once")
	_, err = release(own, "schools", released)
	assert.NoError(t, err, "after the revocation the document can be released again")

	_, err = db.ExecContext(ctx, `UPDATE platform.traeger_documents SET title = 'anders' WHERE id = ?`, document)
	assert.ErrorContains(t, err, "never changes", "not even the migration role changes a document")
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience, group_id, group_name)
		VALUES (?, ?, 'parents', 7, 'Gruppe')`, own, document)
	assert.Error(t, err, "a group release names its school")
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content)
		VALUES (?, 'Skript', 'a.html', 'text/html', 4, 'html'::bytea)`, own)
	assert.Error(t, err, "only the admitted file types are stored")
}
