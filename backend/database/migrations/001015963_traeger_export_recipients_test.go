package migrations

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The one recipient of a Träger before 1.15.963 becomes the first of its
// recipients without losing anything: its values and sealed password stay,
// it receives every format it served so far, and the journal and the
// handovers of the export protocol name it (#6911, ADR 0222). The way back
// refuses while a Träger has more than one recipient.
func TestTraegerExportRecipientsMigrationKeepsTheExistingRecipient(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	unique := testpkg.UniqueTestTenantID(t)
	org := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger Migration", Slug: fmt.Sprintf("recipients-%d", unique)}).ID
	bare := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{Name: "Träger ohne", Slug: fmt.Sprintf("recipients-bare-%d", unique)}).ID

	require.NoError(t, traegerExportRecipientsDown(ctx, db))
	_, err := db.ExecContext(ctx, `INSERT INTO config.traeger_export_recipients
		(organization_id, enabled, host, port, username, password_sealed, remote_directory, host_key_fingerprint)
		VALUES (?, true, 'dateien.kaemmerei.example.org', 2222, 'kaemmerei', '\x0102'::bytea, '/eingang', 'SHA256:abc')`, org)
	require.NoError(t, err)
	_, err = db.ExecContext(ctx, `INSERT INTO audit.traeger_export_transfers (organization_id, export_kind, export_format, filename, byte_size,
		target_host, target_port, target_directory, status, failure_reason, dispatch_state)
		VALUES (?, 'parent_billing_finance', 'datev', 'EXTF_1.csv', 10, 'dateien.kaemmerei.example.org', 2222, '/eingang', 'success', '', 'terminal')`, org)
	require.NoError(t, err)
	var file int64
	require.NoError(t, db.NewRaw(`INSERT INTO parent_billing.export_files (organization_id, file_number, format, run_kind, run_id, file_name,
		content_sha256, invoice_count, total_cents) VALUES (?, 1, 'datev', 'care', 1, 'EXTF_1.csv', repeat('a', 64), 1, 100) RETURNING id`, org).
		Scan(ctx, &file))
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.export_file_transfers (organization_id, export_file_id, target_host, target_directory,
		state, settled_at) VALUES (?, ?, 'dateien.kaemmerei.example.org', '/eingang', 'transferred', NOW())`, org, file)
	require.NoError(t, err)

	require.NoError(t, traegerExportRecipientsUp(ctx, db))
	var recipient struct {
		ID       int64  `bun:"id"`
		Name     string `bun:"name"`
		Host     string `bun:"host"`
		Port     int    `bun:"port"`
		Username string `bun:"username"`
		Sealed   []byte `bun:"password_sealed"`
	}
	require.NoError(t, db.NewRaw(`SELECT id, name, host, port, username, password_sealed FROM config.traeger_export_recipients
		WHERE organization_id = ?`, org).Scan(ctx, &recipient))
	assert.Equal(t, "dateien.kaemmerei.example.org", recipient.Name, "the existing recipient is named after its address")
	assert.Equal(t, 2222, recipient.Port)
	assert.Equal(t, "kaemmerei", recipient.Username)
	assert.Equal(t, []byte{1, 2}, recipient.Sealed, "the sealed password stays as it was")

	var formats []string
	require.NoError(t, db.NewRaw(`SELECT export_format FROM config.traeger_export_recipient_formats
		WHERE organization_id = ? AND recipient_id = ? ORDER BY export_format`, org, recipient.ID).Scan(ctx, &formats))
	assert.Equal(t, []string{"datev", "pain008", "sollstellung"}, formats, "it receives every format it served")
	var bareFormats int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM config.traeger_export_recipient_formats WHERE organization_id = ?`, bare).Scan(ctx, &bareFormats))
	assert.Zero(t, bareFormats, "a Träger without a recipient downloads every format")

	var journal struct {
		RecipientID   *int64 `bun:"recipient_id"`
		RecipientName string `bun:"recipient_name"`
	}
	require.NoError(t, db.NewRaw(`SELECT recipient_id, recipient_name FROM audit.traeger_export_transfers WHERE organization_id = ?`, org).
		Scan(ctx, &journal))
	require.NotNil(t, journal.RecipientID)
	assert.Equal(t, recipient.ID, *journal.RecipientID, "the journal names the recipient of its handovers")
	assert.Equal(t, recipient.Name, journal.RecipientName)
	var handover struct {
		RecipientID   *int64 `bun:"recipient_id"`
		RecipientName string `bun:"recipient_name"`
		State         string `bun:"state"`
	}
	require.NoError(t, db.NewRaw(`SELECT recipient_id, recipient_name, state FROM parent_billing.export_file_transfers WHERE organization_id = ?`, org).
		Scan(ctx, &handover))
	require.NotNil(t, handover.RecipientID)
	assert.Equal(t, recipient.ID, *handover.RecipientID, "a settled handover names its recipient too")
	assert.Equal(t, "transferred", handover.State)

	_, err = db.ExecContext(ctx, `INSERT INTO config.traeger_export_recipients (organization_id, name) VALUES (?, 'Steuerbüro')`, org)
	require.NoError(t, err)
	require.Error(t, traegerExportRecipientsDown(ctx, db), "the way back never drops a second recipient")
	_, err = db.ExecContext(ctx, `DELETE FROM config.traeger_export_recipients WHERE organization_id = ? AND name = 'Steuerbüro'`, org)
	require.NoError(t, err)
	require.NoError(t, traegerExportRecipientsDown(ctx, db))
	require.NoError(t, traegerExportRecipientsUp(ctx, db))
}
