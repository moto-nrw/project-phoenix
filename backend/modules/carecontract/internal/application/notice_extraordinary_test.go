package application

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The extraordinary termination (#4682, § 312k Abs. 2 BGB) and the receipt
// with its time (§ 312k Abs. 4 BGB) on the real store, and the Träger
// filter of the declarations beneath RLS.

var extraordinary = carecontract.NoticeInput{Kind: carecontract.NoticeTermination,
	TerminationKind: carecontract.TerminationExtraordinary, Reason: "  Wir ziehen nach Hamburg.  "}

func (f noticeFixture) parentDeclare(input carecontract.NoticeInput) (carecontract.NoticeOptions, error) {
	var options carecontract.NoticeOptions
	err := testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		input.ReceivedOn = f.notices.today()
		var err error
		options, err = f.notices.declare(txCtx, declaration{organizationID: f.org, accountID: f.reviewer.AccountID,
			channel: carecontract.ChannelParents, contractID: f.contract, input: input, recipient: "erika@example.org",
			traeger: "Ganztag im Quartier e.V."})
		return err
	})
	return options, err
}

// An extraordinary termination is received as declared, with the parents'
// reason; its end stays open, so the contract holds on, until the Träger
// sets the end after its check. The confirmation names the kind's reason
// and the receipt with its time, on the screen and in the mail.
func TestExtraordinaryTerminationWaitsForTheTraegersEnd(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger außerordentlich", noticeToday)
	from, to := calendar.NewDate(2026, 12, 1), calendar.NewDate(2027, 1, 31)

	options, err := f.parentDeclare(extraordinary)
	require.NoError(t, err)
	require.NotNil(t, options.Notice)
	assert.Equal(t, carecontract.StatusTerminated, options.Status)
	assert.Equal(t, carecontract.TerminationExtraordinary, options.Notice.TerminationKind)
	assert.Equal(t, "Wir ziehen nach Hamburg.", options.Notice.Reason)
	assert.Nil(t, options.Notice.EndsOn, "the Träger sets the end after its check")
	want := "Außerordentliche Kündigung für Lina Muster, eingegangen am 10.11.2026 um 11:00 Uhr, Grund: Wir ziehen nach Hamburg. Ganztag im Quartier e.V."
	assert.Equal(t, want, options.Notice.ConfirmationText)
	require.Len(t, f.post.mails, 1)
	assert.Equal(t, want, f.post.mails[0].ConfirmationText, "the mail carries the same text")
	payload, err := json.Marshal(f.post.mails[0])
	require.NoError(t, err)
	rendered, err := RenderNoticeMail(t.Context(), payload)
	require.NoError(t, err)
	assert.Contains(t, rendered.Paragraphs[0], "um 11:00 Uhr", "date and time of the receipt in the mail")
	s := f.state(t)
	assert.Nil(t, s.endsOn)
	require.Len(t, f.versionsInForce(t, from, to), 1)
	assert.Equal(t, to, f.versionsInForce(t, from, to)[0].Until, "the contract holds while its end is open")

	_, err = f.parentDeclare(terminate)
	assert.ErrorIs(t, err, carecontract.ErrContractNotEndable, "one declaration per contract")

	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, calendar.NewDate(2026, 11, 9))
	var invalid *carecontract.ValidationError
	require.ErrorAs(t, err, &invalid, "no end before the receipt")
	end := calendar.NewDate(2026, 12, 31)
	options, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
	require.NoError(t, err)
	require.NotNil(t, options.Notice)
	assert.Equal(t, &end, options.Notice.EndsOn)
	assert.Equal(t, &end, f.state(t).endsOn)
	assert.Equal(t, end, f.versionsInForce(t, from, to)[0].Until, "the contract ends on the day the Träger set")
	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "the end is set once")
	var trail int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ? AND action = 'set_end'`,
		f.contract).Scan(context.Background(), &trail))
	assert.Equal(t, 1, trail)
}

// The parents name a reason only with an extraordinary termination, and
// always with it; the Träger sets no end on an ordinary termination.
func TestExtraordinaryTerminationNeedsItsReason(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Grund", noticeToday)
	var invalid *carecontract.ValidationError
	_, err := f.parentDeclare(carecontract.NoticeInput{Kind: carecontract.NoticeTermination,
		TerminationKind: carecontract.TerminationExtraordinary, Reason: "   "})
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "required", invalid.Fields["reason"])
	ordinary := terminate
	ordinary.Reason = "Umzug"
	_, err = f.parentDeclare(ordinary)
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, "not_allowed", invalid.Fields["reason"])

	options, err := f.parentDeclare(terminate)
	require.NoError(t, err)
	assert.Contains(t, options.Notice.ConfirmationText, "eingegangen am 10.11.2026 um 11:00 Uhr",
		"an ordinary termination with the button also names the time")
	_, err = f.notices.SetNoticeEnd(f.ctx, f.reviewer, f.contract, calendar.NewDate(2027, 1, 31))
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an ordinary termination has its end")
}

// TestNoticesOfAnotherTraegerFindNothingBeneathRLS pins the Träger filter of
// the declarations' store beneath RLS, as #5952 does for the paper
// signature: as the owner of the tables, Träger B reads, ends, marks and
// links a mail to A's contract and finds nothing; A's contract stays
// untouched.
// Each step goes red when its statement drops `organization_id = ?`.
func TestNoticesOfAnotherTraegerFindNothingBeneathRLS(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Erklärung A", noticeToday)
	_, err := f.parentDeclare(extraordinary)
	require.NoError(t, err)
	other := testpkg.CreateTestOrganization(t, f.db, &testpkg.Organization{
		Name: "Träger Erklärung B", Slug: fmt.Sprintf("traeger-erklaerung-b-%d", testpkg.UniqueTestTenantID(t)),
	}).ID
	testpkg.OwnTraegerCareContracts(t, f.db, other)
	store := postgres.NewStore()
	end := calendar.NewDate(2026, 12, 31)

	beneath := func(step func(context.Context) error) error {
		return testpkg.WithinTraeger(f.ctx, other, func(txCtx context.Context) error {
			if err := testpkg.WithoutRequestRole(txCtx); err != nil {
				return err
			}
			return step(txCtx)
		})
	}
	err = beneath(func(txCtx context.Context) error {
		_, err := store.NoticeState(txCtx, other, f.contract)
		return err
	})
	assert.ErrorIs(t, err, carecontract.ErrContractNotFound, "B reads no state of A's contract")
	err = beneath(func(txCtx context.Context) error { return store.SetNoticeEnd(txCtx, other, f.contract, end) })
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "B ends no declaration of A")
	require.NoError(t, beneath(func(txCtx context.Context) error {
		return store.MarkWithdrawalInformed(txCtx, other, f.contract, f.conclusion.At.AddDate(0, 0, -1))
	}))
	require.NoError(t, beneath(func(txCtx context.Context) error {
		var notice int64
		if err := f.db.NewRaw(`SELECT id FROM care_contract.contract_notices WHERE contract_id = ?`, f.contract).
			Scan(txCtx, &notice); err != nil {
			return err
		}
		return store.SetNoticeOutbox(txCtx, other, notice, 999)
	}))

	var row struct {
		EndsOn   *calendar.Date `bun:"ends_on,type:date"`
		Informed bool           `bun:"informed"`
		Outbox   *int64         `bun:"outbox_id"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT c.ends_on, c.withdrawal_informed_at = ? AS informed, n.outbox_id
		FROM care_contract.contracts AS c JOIN care_contract.contract_notices AS n ON n.contract_id = c.id WHERE c.id = ?`,
		f.conclusion.At, f.contract).Scan(context.Background(), &row))
	assert.Nil(t, row.EndsOn, "A's open end stays open")
	assert.True(t, row.Informed, "A's notice provision stays")
	require.NotNil(t, row.Outbox)
	assert.NotEqual(t, int64(999), *row.Outbox, "A's confirmation mail stays")
}
