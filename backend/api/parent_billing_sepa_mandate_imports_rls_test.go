package api

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	careContractCompose "github.com/moto-ogs/project-phoenix/modules/carecontract/compose"
	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	parentBillingCompose "github.com/moto-ogs/project-phoenix/modules/parentbilling/compose"
	securityRuntimeCompose "github.com/moto-ogs/project-phoenix/modules/securityruntime/compose"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The payers of a mandate import are the contracts of the Träger only, also
// below row level security (#6198, rule 25): the Träger transaction drops
// its request role, so a contract of another Träger stays unknown only
// because the Betreuungsvertrag's VersionsInForce names the Träger in its
// own filter. The import runs through the root's real binding of both
// owners. Removing that filter turns this test red.
func TestSEPAMandateImportPayersStayWithTheirTraegerBelowRLS(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Mandatsimport RLS")
	foreign := newCareBillingFixture(t, "Träger Mandatsimport RLS Fremd")
	own := careContract(t, f.db, f.office.organization, f.office.schools[0], "Mia", careMonth, sjrServices()...)
	other := careContract(t, foreign.db, foreign.office.organization, foreign.office.schools[0], "Ben", careMonth, sjrServices()...)
	saveCreditor(t, f, map[string]any{"creditor_id": sepaCreditorID, "prenotification_days": 14, "missing_mandate": "transfer"})
	reviewer := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("rls-import-%d@example.org", testpkg.UniqueTestTenantID(t)))

	cipher, err := securityRuntimeCompose.NewBankDataCipher(testpkg.NewTestHealthDataKeys(), "")
	require.NoError(t, err)
	mandates := parentBillingCompose.NewSEPAMandates(
		parentBillingCareContracts{contracts: careContractCompose.NewContractVersions()}, cipher, noDebtorNumberFormat{})
	csv := fmt.Sprintf("Mandatsreferenz;Mandatsdatum;IBAN;Kontoinhaber;Zahler\n"+
		"OGS-R-1;01.09.2026;%[1]s;Eva Beispiel;%[2]d\nOGS-R-2;01.09.2026;%[1]s;Eva Beispiel;%[3]d\n", sepaIBAN, own, other)
	mapping := &parentbilling.SEPAMandateImportMapping{Columns: parentbilling.SEPAMandateImportColumns{
		Reference: "Mandatsreferenz", SignedOn: "Mandatsdatum", IBAN: "IBAN", AccountHolder: "Kontoinhaber", Payer: "Zahler",
	}, PayerKey: parentbilling.PayerByContract, CreditorFromTraeger: true, ConfirmedCreditorID: sepaCreditorID}
	rollback := errors.New("roll the import back")

	ctx := testpkg.ContextWithTenantRuntime(context.Background(), testpkg.TenantRuntime(t, f.db))
	err = testpkg.WithinTraeger(ctx, f.office.organization, func(ctx context.Context) error {
		require.NoError(t, testpkg.WithoutRequestRole(ctx))
		result, err := mandates.ImportSEPAMandates(ctx, parentbilling.HolidayReviewer{
			OrganizationID: f.office.organization, AccountID: reviewer.ID,
		}, parentbilling.SEPAMandateImportRequest{Content: []byte(csv), Mapping: mapping})
		require.NoError(t, err)
		require.Len(t, result.Rows, 2)
		assert.Equal(t, own, result.Rows[0].ContractID, "the Träger's own contract is a payer")
		assert.Zero(t, result.Rows[1].ContractID, "a contract of another Träger is no payer")
		assert.Equal(t, parentbilling.ImportRowInvalid, result.Rows[1].Status)
		assert.Contains(t, result.Rows[1].Problems, parentbilling.SEPAMandateImportProblem{
			Field: parentbilling.ImportFieldPayer, Code: parentbilling.ImportProblemPayerUnknown,
		})
		assert.Empty(t, result.Rows[1].PayerLastName, "and names no one of it")
		return rollback
	})
	require.ErrorIs(t, err, rollback)
}

// noDebtorNumberFormat is a Träger without a format of its debtor numbers
// (#6675); the import of this test maps no debtor number column.
type noDebtorNumberFormat struct{}

func (noDebtorNumberFormat) DebtorNumberFormat(context.Context, int64) (parentbilling.DebtorNumberFormat, error) {
	return parentbilling.DebtorNumberFormat{}, nil
}

// DebtorNumberScope keeps the number per payer, the default (#7007).
func (noDebtorNumberFormat) DebtorNumberScope(context.Context, int64) (string, error) {
	return parentbilling.DebtorNumberScopePayer, nil
}
