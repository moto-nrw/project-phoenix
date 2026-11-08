package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestCareLettersRespectInvoiceRecipientChanges(t *testing.T) {
	t.Parallel()
	for _, bulk := range []bool{false, true} {
		for _, changed := range []bool{false, true} {
			t.Run(fmt.Sprintf("bulk=%t/changed=%t", bulk, changed), func(t *testing.T) {
				f := newCareBillingFixture(t, "Träger geänderter Rechnungsempfänger")
				brandCareTraeger(t, f)
				run, contracts := amendableRun(t, f, "Mia")
				for _, email := range []string{"vater@example.org", "rechnung@example.org"} {
					code, _ := f.amend(t, run.ID, contracts["Mia"], map[string]any{
						"kind": "recipient", "reason": "Rechnung an den Vater",
						"recipient": map[string]any{"first_name": "Jan", "last_name": "Beispiel", "email": email},
					})
					require.Equal(t, http.StatusOK, code)
				}
				review := f.run(t, run.ID)
				rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review))
				require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
				invoice := invoiceOfContract(t, f.run(t, run.ID), contracts["Mia"])
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/letters"), careLettersInput())
				require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
				if changed {
					_, err := f.db.ExecContext(context.Background(), `UPDATE care_contract.contract_parties SET email = 'neu@example.org' WHERE contract_id = ?`, contracts["Mia"])
					require.NoError(t, err)
				}
				path := fmt.Sprintf("/invoices/%d/mail", invoice.ID)
				if bulk {
					path = "/letters/mail"
				}
				rec = traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, path), nil)
				if changed && !bulk {
					require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
					assert.Contains(t, rec.Body.String(), "parent_billing.care_recipient_changed")
				} else {
					require.Equal(t, http.StatusAccepted, rec.Code, rec.Body.String())
				}
				if bulk {
					want := careSeriesBody{Queued: 1}
					if changed {
						want = careSeriesBody{RecipientChanged: 1}
					}
					assert.Equal(t, want, decodeSchoolPortalData[careSeriesBody](t, rec.Body.Bytes()))
				}
				mails := careLetterMails(t, f, run.ID)
				if changed {
					assert.Empty(t, mails)
				} else {
					require.Len(t, mails, 1)
					var email string
					require.NoError(t, f.db.NewRaw(`SELECT recipient->>'address' FROM platform.email_outbox WHERE id = ?`, mails[0].ID).Scan(context.Background(), &email))
					assert.Equal(t, "rechnung@example.org", email)
				}
			})
		}
	}
}

func TestCareAmendmentsOutliveTheirAccount(t *testing.T) {
	t.Parallel()
	f := newCareBillingFixture(t, "Träger Änderung Konto gelöscht")
	ctx := context.Background()
	run, contracts := amendableRun(t, f, "Mia")
	account := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("amendment-%d@example.org", testpkg.UniqueTestTenantID(t)))
	_, err := f.db.ExecContext(ctx, `INSERT INTO parent_billing.care_run_amendments
		(organization_id, run_id, care_contract_id, service, kind, amount_cents, label, reason, account_id)
		VALUES (?, ?, ?, 'care', 'add_position', 100, 'Ausflug', 'Elternbrief', ?)`, f.office.organization, run.ID, contracts["Mia"], account.ID)
	require.NoError(t, err)
	review := f.run(t, run.ID)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, careRunPath(run.ID, "/release"), careRelease(review))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	for _, table := range []string{"care_run_amendments", "care_invoice_corrections"} {
		_, err = f.db.ExecContext(ctx, `UPDATE parent_billing.`+table+` SET account_id = NULL, reason = 'Anders' WHERE account_id = ?`, account.ID)
		require.Error(t, err, "clearing the account must not permit other changes")
	}
	_, err = f.db.ExecContext(ctx, `DELETE FROM auth.accounts WHERE id = ?`, account.ID)
	require.NoError(t, err, "both references must allow ON DELETE SET NULL after release")
	for _, table := range []string{"care_run_amendments", "care_invoice_corrections"} {
		var row struct {
			AccountID   *int64
			Reason      string
			AmountCents int64
		}
		require.NoError(t, f.db.NewRaw(`SELECT account_id, reason, amount_cents FROM parent_billing.`+table+` WHERE organization_id = ?`, f.office.organization).Scan(ctx, &row))
		assert.Nil(t, row.AccountID)
		assert.Equal(t, "Elternbrief", row.Reason)
		assert.Equal(t, int64(100), row.AmountCents)
	}
}
