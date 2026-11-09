package parentportal_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	parentportalcompose "github.com/moto-nrw/project-phoenix/workflows/parentportal/compose"
)

const mandateTestIBAN = "DE89370400440532013000"

type mandateFixture struct {
	portal       *parentService.Portal
	db           *bun.DB
	settings     *owners.ParentMandateTestSettings
	chain        testpkg.ParentChain
	organization int64
	link         int64
}

func newMandateFixture(t *testing.T) mandateFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	cipher, err := api.BankDataCipher(testpkg.TestBankDataKeys, "")
	require.NoError(t, err)
	settings := &owners.ParentMandateTestSettings{}
	deps := documentInboxDependencies(t, db, &owners.CareContractTestMails{})
	deps.SEPAMandates = owners.NewParentSEPAMandatesForTests(settings, cipher)
	f := mandateFixture{portal: parentportalcompose.New(deps), db: db, settings: settings, chain: testpkg.CreateTestParentGuardianChain(t, db)}
	ctx := context.Background()
	require.NoError(t, db.NewRaw(`SELECT organization_id FROM platform.schools WHERE id = ?`, f.chain.TenantID).Scan(ctx, &f.organization))
	require.NoError(t, db.NewRaw(`SELECT id FROM users.student_guardian_relationships WHERE student_id = ? AND guardian_profile_id = ?`,
		f.chain.StudentID, f.chain.GuardianProfileID).Scan(ctx, &f.link))
	testpkg.OwnTraegerCareContracts(t, db, f.organization)
	_, err = db.ExecContext(ctx, `INSERT INTO parent_billing.sepa_creditors (organization_id, creditor_id) VALUES (?, 'DE98ZZZ09999999999')`, f.organization)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := db.ExecContext(context.Background(), `DELETE FROM parent_billing.sepa_creditors WHERE organization_id = ?`, f.organization)
		require.NoError(t, err, "remove the creditor of the test Träger")
	})
	settings.Set(f.organization, true, "Jugendring Muster e.V.\nHauptstraße 1\n75175 Pforzheim")
	return f
}

func mandateInput() parentService.SEPAMandateInput {
	return parentService.SEPAMandateInput{AccountHolder: "Eva Muster", IBAN: "DE89 3704 0044 0532 0130 00", Street: "Gartenweg 2",
		PostalCode: "75175", City: "Pforzheim", HolderConfirmed: true}
}

// grant previews the text and confirms exactly that text.
func (f mandateFixture) grant(ctx context.Context, contract int64) (parentService.SEPAMandateView, error) {
	input := mandateInput()
	text, err := f.portal.PreviewSEPAMandate(ctx, f.chain.AccountID, f.organization, contract, input)
	if err != nil {
		return parentService.SEPAMandateView{}, err
	}
	input.Reference, input.TextSHA256 = text.Reference, text.TextSHA256
	return f.portal.GrantSEPAMandate(ctx, f.chain.AccountID, f.organization, contract, input)
}

// Parents give, read and revoke the mandate of their family only on a
// contract sent to their own address, with the right
// parent_portal.enrollment.submit for its child, while the Träger allows
// online mandates (#4696). The mandate and its proof land in the Träger's
// rows with the account; the answer carries the mask, never the IBAN.
func TestParentsGiveTheSEPAMandateOnlyForTheirOwnFamily(t *testing.T) {
	t.Parallel()
	f := newMandateFixture(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	own := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Felix", f.chain.Email)
	otherFamily := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Tom", "andere.familie@example.org")

	_, err := f.grant(ctx, otherFamily)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound, "another family's contract")
	_, err = f.portal.SEPAMandate(ctx, f.chain.AccountID, f.organization, otherFamily)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, f.db, f.link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true,
	}))
	read, err := f.portal.SEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	require.NoError(t, err, "the inbox right reads")
	assert.True(t, read.Enabled)
	assert.False(t, read.CanManage, "but does not give a mandate")
	_, err = f.grant(ctx, own)
	require.ErrorIs(t, err, parentService.ErrGuardianPermissionDenied, "reading the inbox gives no mandate")

	require.NoError(t, testpkg.SetTestStudentGuardianPermissions(ctx, f.db, f.link, map[string]interface{}{
		"parent_portal.access": true, "parent_portal.enrollments.view": true, "parent_portal.enrollment.submit": true,
	}))
	granted, err := f.grant(ctx, own)
	require.NoError(t, err)
	require.NotNil(t, granted.Mandate)
	assert.True(t, granted.CanManage)
	assert.Equal(t, "parent", granted.Mandate.Source)
	assert.Equal(t, "DE•• •••• •••• •••• ••30 00", granted.Mandate.IBANMasked)
	require.NotNil(t, granted.Mandate.Proof)
	assert.Contains(t, granted.Mandate.Proof.Text, "Gläubiger-Identifikationsnummer: DE98ZZZ09999999999")
	assert.NotContains(t, granted.Mandate.Proof.Text, mandateTestIBAN)

	_, err = f.grant(ctx, own)
	assert.ErrorIs(t, err, parentService.ErrSEPAMandateInForce, "one mandate in force per family")

	var stored struct {
		Source  string `bun:"source"`
		Sealed  []byte `bun:"iban_sealed"`
		Account int64  `bun:"account_id"`
		Hash    string `bun:"text_sha256"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT mandate.source, mandate.iban_sealed, consent.account_id, consent.text_sha256
		FROM parent_billing.sepa_mandates AS mandate JOIN parent_billing.sepa_mandate_consents AS consent
		  ON consent.organization_id = mandate.organization_id AND consent.mandate_id = mandate.id
		WHERE mandate.organization_id = ? AND mandate.care_contract_id = ?`, f.organization, own).Scan(ctx, &stored))
	assert.Equal(t, "parent", stored.Source)
	assert.NotContains(t, string(stored.Sealed), mandateTestIBAN, "the IBAN is sealed at rest")
	assert.Equal(t, f.chain.AccountID, stored.Account, "the proof names the account")
	assert.Equal(t, granted.Mandate.Proof.TextSHA256, stored.Hash)

	revoked, err := f.portal.RevokeSEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	require.NoError(t, err)
	require.NotNil(t, revoked.Mandate)
	assert.NotNil(t, revoked.Mandate.RevokedOn, "the revocation ends the mandate for the next run")
	_, err = f.portal.RevokeSEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	assert.ErrorIs(t, err, parentService.ErrSEPAMandateNotFound)

	var trail []string
	require.NoError(t, f.db.NewRaw(`SELECT action FROM parent_billing.sepa_mandate_events
		WHERE organization_id = ? AND care_contract_id = ? AND source = 'parent' AND actor_account_id = ? ORDER BY id`,
		f.organization, own, f.chain.AccountID).Scan(ctx, &trail))
	assert.Equal(t, []string{"create", "revoke"}, trail, "the grant and the revocation, both with the account")
}

// The mandate belongs to the payer: a contract mailed to the account whose
// billing party is another address gives no mandate (Konto-Mail =
// Vertragspartei), even with every relationship right.
func TestParentsGiveNoSEPAMandateForAnotherPayer(t *testing.T) {
	t.Parallel()
	f := newMandateFixture(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	own := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Felix", f.chain.Email)
	_, err := f.db.ExecContext(ctx, `UPDATE care_contract.contract_parties SET email = 'zahler@example.org' WHERE contract_id = ?`, own)
	require.NoError(t, err)

	_, err = f.grant(ctx, own)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)
	_, err = f.portal.SEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	assert.ErrorIs(t, err, parentService.ErrBillingDocumentNotFound)
}

// With the Träger's switch off no route of the mandate does anything: the
// read answers only that it is off, every write is refused and nothing is
// stored or recorded.
func TestSEPAMandateSwitchedOffDoesNothing(t *testing.T) {
	t.Parallel()
	f := newMandateFixture(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	own := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Felix", f.chain.Email)
	_, err := f.grant(ctx, own)
	require.NoError(t, err)
	f.settings.Set(f.organization, false, "")

	read, err := f.portal.SEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	require.NoError(t, err)
	assert.Equal(t, parentService.SEPAMandateView{}, read, "off answers nothing but enabled=false")
	_, err = f.portal.PreviewSEPAMandate(ctx, f.chain.AccountID, f.organization, own, mandateInput())
	assert.ErrorIs(t, err, parentService.ErrOnlineMandateDisabled)
	_, err = f.portal.GrantSEPAMandate(ctx, f.chain.AccountID, f.organization, own, mandateInput())
	assert.ErrorIs(t, err, parentService.ErrOnlineMandateDisabled)
	_, err = f.portal.RevokeSEPAMandate(ctx, f.chain.AccountID, f.organization, own)
	assert.ErrorIs(t, err, parentService.ErrOnlineMandateDisabled)

	var revoked, events int
	require.NoError(t, f.db.NewRaw(`SELECT
		(SELECT count(*) FROM parent_billing.sepa_mandates WHERE organization_id = ? AND revoked_on IS NOT NULL),
		(SELECT count(*) FROM parent_billing.sepa_mandate_events WHERE organization_id = ? AND action <> 'create')`,
		f.organization, f.organization).Scan(ctx, &revoked, &events))
	assert.Zero(t, revoked, "the mandate stays in force")
	assert.Zero(t, events, "no read or change is recorded")
}

func TestParentInboxMandateStatusesReadNoProofOrBankData(t *testing.T) {
	t.Parallel()
	f := newMandateFixture(t)
	ctx := testpkg.WithPackageTenantRuntime(context.Background())
	own := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Felix", f.chain.Email)
	otherPayer := activeInboxContract(t, f.db, f.organization, f.chain.TenantID, f.chain.StudentID, "Mia", f.chain.Email)
	_, err := f.db.ExecContext(ctx, `UPDATE care_contract.contract_parties SET email = 'zahler@example.org' WHERE contract_id = ?`, otherPayer)
	require.NoError(t, err)
	_, err = f.grant(ctx, own)
	require.NoError(t, err)
	queries := testpkg.CaptureQueriesForContext(t, f.db)
	documents, err := f.portal.ListBillingDocuments(queries.Context(ctx), f.chain.AccountID, parentService.DocumentInboxContracts)
	require.NoError(t, err)
	require.Len(t, documents, 2)
	for _, document := range documents {
		if document.ID == own {
			require.NotNil(t, document.SEPAMandate)
			assert.True(t, document.SEPAMandate.Enabled)
			assert.True(t, document.SEPAMandate.CollectionEligible)
		} else {
			assert.Nil(t, document.SEPAMandate, "the mailbox address is not the payer")
		}
	}
	assert.Empty(t, queries.Selects("sepa_mandate_consents"))
	for _, query := range queries.Queries() {
		assert.NotContains(t, query, "sepa_mandate_events", "no mandate-detail audit reads or writes")
	}
	assert.Len(t, queries.Selects("sepa_mandates"), 1, "one family status query, independent of contract count")
}
