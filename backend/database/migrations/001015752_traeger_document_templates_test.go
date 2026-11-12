package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestTraegerDocumentTemplateVersionsNeverChange pins ADR 0152 on
// platform.traeger_document_templates: under the request role a Träger reads
// and adds only its own versions and a school transaction sees none; no
// role changes a version, the request role deletes none; a version number
// exists once per kind; only the kinds of the contract are stored; deleting
// the saving account keeps the version.
func TestTraegerDocumentTemplateVersionsNeverChange(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	newTraeger := func(name string) int64 {
		slug := fmt.Sprintf("traeger-vorlage-%d", testpkg.UniqueTestTenantID(t))
		organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: name, Slug: slug}).ID
		testpkg.OwnTraegerMasterData(t, db, organization)
		return organization
	}
	own, foreign := newTraeger("Träger Vorlage Eigen"), newTraeger("Träger Vorlage Fremd")

	asRequestRole := func(setting, value string, fn func(bun.Tx) error) error {
		return db.RunInTx(ctx, &sql.TxOptions{}, func(ctx context.Context, tx bun.Tx) error {
			if _, err := tx.ExecContext(ctx, `SET LOCAL ROLE phoenix_tenant`); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `SELECT set_config(?, ?, true)`, setting, value); err != nil {
				return err
			}
			return fn(tx)
		})
	}
	insert := `INSERT INTO platform.traeger_document_templates (organization_id, kind, version, subject, body) VALUES (?, ?, ?, ?, ?)`
	require.NoError(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, insert, own, "rechnung", 1, "Rechnung {{rechnungsnummer}}", "Text")
		return err
	}), "the request role adds a version of its Träger")
	assert.Error(t, asRequestRole("app.current_org_id", fmt.Sprint(foreign), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, insert, own, "rechnung", 2, "Fremd", "Text")
		return err
	}), "another Träger adds no version for this one")

	count := func(setting, value string) int {
		var n int
		require.NoError(t, asRequestRole(setting, value, func(tx bun.Tx) error {
			return tx.NewRaw(`SELECT COUNT(*) FROM platform.traeger_document_templates WHERE organization_id = ?`, own).Scan(ctx, &n)
		}))
		return n
	}
	assert.Equal(t, 1, count("app.current_org_id", fmt.Sprint(own)))
	assert.Zero(t, count("app.current_org_id", fmt.Sprint(foreign)), "another Träger sees nothing")
	assert.Zero(t, count("app.current_tenant_id", "1"), "a school transaction sees nothing")

	_, err := db.ExecContext(ctx, `UPDATE platform.traeger_document_templates SET body = 'anders' WHERE organization_id = ?`, own)
	assert.ErrorContains(t, err, "never changes", "not even the migration role changes a version")
	assert.Error(t, asRequestRole("app.current_org_id", fmt.Sprint(own), func(tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, `DELETE FROM platform.traeger_document_templates WHERE organization_id = ?`, own)
		return err
	}), "the request role deletes no version")

	_, err = db.ExecContext(ctx, insert, own, "rechnung", 1, "Doppelt", "Text")
	assert.Error(t, err, "a version number exists once per kind")
	_, err = db.ExecContext(ctx, insert, own, "brief", 1, "Brief", "Text")
	assert.Error(t, err, "only the kinds of the contract are stored")

	// Deleting the account that saved a version sets it NULL and keeps the
	// version.
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("vorlage-%d@example.org", testpkg.UniqueTestTenantID(t)))
	_, err = db.ExecContext(ctx, `INSERT INTO platform.traeger_document_templates (organization_id, kind, version, subject, body, created_by)
		VALUES (?, 'mahnung', 1, 'Erinnerung', 'Text', ?)`, own, account.ID)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err)
	var kept int
	require.NoError(t, db.NewRaw(`SELECT COUNT(*) FROM platform.traeger_document_templates
		WHERE organization_id = ? AND kind = 'mahnung' AND created_by IS NULL`, own).Scan(ctx, &kept))
	assert.Equal(t, 1, kept)
}
