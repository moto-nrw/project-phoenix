package compose_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// The parents' inbox marks a sent contract version that a change on its own
// first day replaced (#4681): the PDF they have holds on no day, and a
// changed version holds from the same day.
func TestInboxMarksTheSentVersionReplacedByAChangeOnItsFirstDay(t *testing.T) {
	t.Parallel()
	f := newTableFixture(t, 0, []tableVersion{{"wirksam", "2099-08-01", []string{"mon"}}})
	student := f.contract * 10
	email := fmt.Sprintf("eltern-%d@example.org", f.contract)
	var version, document int64
	require.NoError(t, f.db.NewRaw(`UPDATE care_contract.contracts SET student_id = ? WHERE id = ? RETURNING id`,
		student, f.contract).Scan(f.ctx, new(int64)))
	require.NoError(t, f.db.NewRaw(`SELECT id FROM care_contract.contract_versions WHERE contract_id = ? AND version_number = 1`,
		f.contract).Scan(f.ctx, &version))
	require.NoError(t, f.db.NewRaw(`INSERT INTO care_contract.contract_documents (organization_id, contract_id, contract_version_id,
			template_version, filename, pdf) VALUES (?, ?, ?, 0, 'vertrag.pdf', ?) RETURNING id`,
		f.organization, f.contract, version, []byte("%PDF-1.4")).Scan(f.ctx, &document))
	_, err := f.db.ExecContext(f.ctx, `INSERT INTO care_contract.contract_mailings (organization_id, contract_id,
			contract_document_id, outbox_id, recipient_email) VALUES (?, ?, ?, ?, ?)`,
		f.organization, f.contract, document, f.contract, email)
	require.NoError(t, err)

	var schoolID int64
	require.NoError(t, f.db.NewRaw(`SELECT school_id FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &schoolID))
	reader := carecontract.InboxReader{AccountID: 1, Email: email, Children: map[int64][]int64{schoolID: {student}}}
	inbox := func() carecontract.InboxContract {
		var found []carecontract.InboxContract
		require.NoError(t, tenant.WithinTraeger(f.ctx, f.organization, func(txCtx context.Context) error {
			var err error
			found, err = compose.NewInbox().InboxContracts(txCtx, f.organization, reader)
			return err
		}))
		require.Len(t, found, 1)
		return found[0]
	}
	assert.False(t, inbox().Replaced, "the sent version holds")

	aug := calendar.NewDate(2099, 8, 1)
	f.step(t, booking(f.short, []string{"mon", "wed"}, &aug, nil))
	assert.Equal(t, []string{"1:ersetzt", "2:wirksam"}, f.statuses(t))
	sent := inbox()
	assert.True(t, sent.Replaced, "the parents see that their version was replaced")
	assert.Equal(t, aug, sent.EffectiveOn, "the changed version holds from the same day")
	download := func() {
		require.NoError(t, tenant.WithinTraeger(f.ctx, f.organization, func(ctx context.Context) error {
			file, err := compose.NewInbox().InboxContractDocument(ctx, f.organization, f.contract, reader)
			require.NoError(t, err)
			assert.Equal(t, []byte("%PDF-1.4"), file.Data)
			return nil
		}))
	}
	var informed *time.Time
	download()
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	assert.Nil(t, informed, "a legacy PDF contains no proven notice")
	_, err = f.db.ExecContext(f.ctx, `UPDATE care_contract.contract_documents SET withdrawal_notice = true WHERE id = ?`, document)
	require.NoError(t, err)
	download()
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	assert.NotNil(t, informed, "downloading a notice-bearing PDF records actual provision")
}

func booking(offering int64, days []string, from, until *calendar.Date) carecontract.Booking {
	return carecontract.Booking{CareOfferingID: offering, Days: days, ValidFrom: from, ValidUntil: until}
}
