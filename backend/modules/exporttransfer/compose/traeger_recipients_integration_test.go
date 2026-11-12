package compose

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// Several recipients of one Träger, one recipient per format (#6911, ADR
// 0221). Each test names the rule it proves; removing that rule turns it
// red.

// twoRecipients enters the Kämmerei and the tax office of org, each with a
// directory of its own, and sends the Sollstellung and the debit file to the
// Kämmerei, the DATEV file to the tax office.
func (f traegerFixture) twoRecipients(t *testing.T, org int64) (kaemmerei, steuerbuero int64, kaemmereiDir, steuerbueroDir string) {
	t.Helper()
	kaemmereiDir, steuerbueroDir = t.TempDir(), t.TempDir()
	input := f.readyInput()
	input.RemoteDirectory = kaemmereiDir
	kaemmerei = f.add(t, org, input)
	input = f.readyInput()
	input.Name, input.RemoteDirectory = "Steuerbüro", steuerbueroDir
	steuerbuero = f.add(t, org, input)
	f.assign(t, org, map[string]int64{"sollstellung": kaemmerei, "datev": steuerbuero, "pain008": kaemmerei})
	return kaemmerei, steuerbuero, kaemmereiDir, steuerbueroDir
}

func formatRequest(org int64, format, filename string, destinations *[]exporttransfer.Destination) exporttransfer.TraegerTransferRequest {
	return exporttransfer.TraegerTransferRequest{OrganizationID: org, Kind: exporttransfer.KindParentBillingFinance, Format: format,
		PrepareFile: func(_ context.Context, destination exporttransfer.Destination) (string, []byte, error) {
			*destinations = append(*destinations, destination)
			return filename, []byte(format), nil
		},
		Settle: func(context.Context, exporttransfer.Outcome) error { return nil },
	}
}

// Each format goes to the recipient the Träger assigned, and the producer
// learns which one; the journal names it.
func TestTraegerFormatsGoToTheirOwnRecipient(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	kaemmerei, steuerbuero, kaemmereiDir, steuerbueroDir := f.twoRecipients(t, f.org)
	var destinations []exporttransfer.Destination
	for format, filename := range map[string]string{"sollstellung": "soll.csv", "datev": "datev.csv"} {
		outcome, err := f.transfers.Transfer(f.ctx, formatRequest(f.org, format, filename, &destinations))
		require.NoError(t, err)
		require.True(t, outcome.Transferred, format)
	}
	_, err := os.Stat(filepath.Join(kaemmereiDir, "soll.csv"))
	require.NoError(t, err, "the Sollstellung lands at the Kämmerei")
	_, err = os.Stat(filepath.Join(steuerbueroDir, "datev.csv"))
	require.NoError(t, err, "the DATEV file lands at the tax office")
	_, err = os.Stat(filepath.Join(kaemmereiDir, "datev.csv"))
	assert.True(t, os.IsNotExist(err), "the DATEV file never reaches the Kämmerei")
	require.Len(t, destinations, 2)
	for _, destination := range destinations {
		assert.Contains(t, []int64{kaemmerei, steuerbuero}, destination.RecipientID)
		assert.NotEmpty(t, destination.RecipientName)
	}

	page, err := f.transfers.Recipient(f.ctx, f.org, steuerbuero, 10)
	require.NoError(t, err)
	require.Len(t, page.Handovers, 1, "the journal of a recipient holds only its own handovers")
	assert.Equal(t, "datev.csv", page.Handovers[0].Filename)
	assert.Equal(t, "Steuerbüro", page.Handovers[0].RecipientName)
	assert.Equal(t, []string{"datev"}, page.Recipient.Formats)
	all, err := f.transfers.Recipients(f.ctx, f.org, 10)
	require.NoError(t, err)
	assert.Len(t, all.Handovers, 2)
	assert.Equal(t, []exporttransfer.TraegerAssignment{
		{Format: "sollstellung", RecipientID: &kaemmerei}, {Format: "datev", RecipientID: &steuerbuero}, {Format: "pain008", RecipientID: &kaemmerei},
	}, all.Assignments)
}

// The error state belongs to a recipient: one that failed shows it, the
// other one keeps its own.
func TestTraegerFailureStaysWithItsRecipient(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	kaemmerei, steuerbuero, _, _ := f.twoRecipients(t, f.org)
	input := f.readyInput()
	input.Name, input.HostKeyFingerprint = "Steuerbüro", f.server.FingerprintECDSA
	_, err := f.transfers.SaveRecipient(f.ctx, f.org, 0, steuerbuero, input)
	require.NoError(t, err)
	var destinations []exporttransfer.Destination
	failed, err := f.transfers.Transfer(f.ctx, formatRequest(f.org, "datev", "datev.csv", &destinations))
	require.NoError(t, err)
	require.False(t, failed.Transferred)
	arrived, err := f.transfers.Transfer(f.ctx, formatRequest(f.org, "pain008", "pain.xml", &destinations))
	require.NoError(t, err)
	require.True(t, arrived.Transferred)
	page, err := f.transfers.Recipients(f.ctx, f.org, 10)
	require.NoError(t, err)
	states := map[int64]string{}
	for _, recipient := range page.Recipients {
		require.NotNil(t, recipient.LastHandover)
		states[recipient.ID] = recipient.LastHandover.State
	}
	assert.Equal(t, map[int64]string{kaemmerei: exporttransfer.TraegerTransferred, steuerbuero: exporttransfer.TraegerFailed}, states)
}

// Removing a recipient sends its formats back to the download and keeps its
// handovers in the journal with the name they had.
func TestTraegerRemovingARecipientReturnsItsFormatsToTheDownload(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	kaemmerei, steuerbuero, _, _ := f.twoRecipients(t, f.org)
	var destinations []exporttransfer.Destination
	_, err := f.transfers.Transfer(f.ctx, formatRequest(f.org, "sollstellung", "soll.csv", &destinations))
	require.NoError(t, err)
	page, err := f.transfers.DeleteRecipient(f.ctx, f.org, kaemmerei)
	require.NoError(t, err)
	assert.Equal(t, []exporttransfer.TraegerAssignment{
		{Format: "sollstellung"}, {Format: "datev", RecipientID: &steuerbuero}, {Format: "pain008"},
	}, page.Assignments, "the formats of the removed recipient go back to the download, the others stay")
	require.Len(t, page.Handovers, 1, "the handovers of a removed recipient stay in the journal")
	assert.Nil(t, page.Handovers[0].RecipientID)
	assert.Equal(t, "Kämmerei", page.Handovers[0].RecipientName)
	prepared := 0
	request := formatRequest(f.org, "sollstellung", "soll-2.csv", &destinations)
	request.PrepareFile = func(context.Context, exporttransfer.Destination) (string, []byte, error) {
		prepared++
		return "soll-2.csv", nil, nil
	}
	outcome, err := f.transfers.Transfer(f.ctx, request)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.ReasonNotConfigured, outcome.Reason)
	assert.Zero(t, prepared, "a format without a recipient makes no file")
}

// Two recipients of one Träger never share a name, in any case; another
// Träger may use it.
func TestTraegerRecipientNamesStayApart(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	first := f.add(t, f.org, f.readyInput())
	input := f.readyInput()
	input.Name = " KÄMMEREI "
	_, err := f.transfers.CreateRecipient(f.ctx, f.org, 0, input)
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNameTaken)
	input.Name = "Steuerbüro"
	second := f.add(t, f.org, input)
	input.Name = "kämmerei"
	_, err = f.transfers.SaveRecipient(f.ctx, f.org, 0, second, input)
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNameTaken, "a rename never takes the name of another recipient")
	f.add(t, f.other, f.readyInput())
	input.Name = ""
	_, err = f.transfers.SaveRecipient(f.ctx, f.org, 0, first, input)
	var invalid *exporttransfer.RecipientError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, []string{exporttransfer.RecipientFieldName}, invalid.Fields)
}

// An assignment names a format of the list and a recipient of the own
// Träger, or nothing changes.
func TestTraegerAssignmentRefusesForeignRecipientsAndUnknownFormats(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	own := f.save(t, f.org)
	foreign := f.add(t, f.other, f.readyInput())
	_, err := f.transfers.AssignFormats(f.ctx, f.org, 0, []exporttransfer.TraegerAssignment{
		{Format: "pain008"}, {Format: "datev", RecipientID: &foreign},
	})
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNotFound)
	for name, assignments := range map[string][]exporttransfer.TraegerAssignment{
		"unknown format": {{Format: "cip", RecipientID: &own}},
		"format twice":   {{Format: "datev", RecipientID: &own}, {Format: "datev"}},
	} {
		_, err := f.transfers.AssignFormats(f.ctx, f.org, 0, assignments)
		require.ErrorIs(t, err, exporttransfer.ErrAssignmentInvalid, name)
	}
	page, err := f.transfers.Recipient(f.ctx, f.org, own, 10)
	require.NoError(t, err)
	assert.Equal(t, exporttransfer.TraegerFormats, page.Recipient.Formats, "a refused assignment changes no format, also not the valid part")
	_, err = f.transfers.Recipient(f.ctx, f.org, foreign, 10)
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNotFound, "another Träger's recipient is not readable by its id")
	_, err = f.transfers.SaveRecipient(f.ctx, f.org, 0, foreign, f.readyInput())
	require.ErrorIs(t, err, exporttransfer.ErrRecipientNotFound, "nor writable")
}

// The assignment policy hides another Träger's rows from an unfiltered
// query of the request role, and the composite reference refuses a row
// that points at another Träger's recipient even without the store's filter.
func TestTraegerAssignmentPolicyAndReferenceKeepTheTraeger(t *testing.T) {
	t.Parallel()
	f := newTraegerFixture(t, nil)
	f.save(t, f.other)
	foreign := f.add(t, f.other, func() exporttransfer.TraegerRecipientInput {
		input := f.readyInput()
		input.Name = "Steuerbüro"
		return input
	}())
	require.NoError(t, tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		var visible int
		require.NoError(t, requestTx(t, ctx).NewRaw(`SELECT count(*) FROM config.traeger_export_recipient_formats`).Scan(ctx, &visible))
		assert.Zero(t, visible, "another Träger's assignments stay hidden")
		return nil
	}))
	err := tenant.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		_, err := requestTx(t, ctx).ExecContext(ctx, `INSERT INTO config.traeger_export_recipient_formats
			(organization_id, export_format, recipient_id) VALUES (?, 'datev', ?)`, f.org, foreign)
		return err
	})
	require.Error(t, err, "a format of one Träger never points at a recipient of another")
}
