package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// The extraordinary termination (#4682, § 312k Abs. 2 BGB) and the receipt
// with its time (§ 312k Abs. 4 BGB) on the real store, and the Träger
// filter of the declarations beneath RLS.

// errRollback ends a transaction a test only looked into.
var errRollback = errors.New("test: roll back")

// endsWithin reads the end of A's declaration and of A's contract in the
// transaction of txCtx, so a test sees what a statement wrote before the
// rollback hides it. The caller has dropped the request role.
func (f noticeFixture) endsWithin(t *testing.T, txCtx context.Context) (notice, contract *calendar.Date) {
	t.Helper()
	raw, ok := testpkg.TransactionFromContext(txCtx)
	require.True(t, ok, "a transaction is required")
	tx, ok := raw.(bun.Tx)
	require.True(t, ok, "a bun transaction is required")
	var row struct {
		Notice   *calendar.Date `bun:"notice_end,type:date"`
		Contract *calendar.Date `bun:"contract_end,type:date"`
	}
	require.NoError(t, tx.NewRaw(`SELECT n.ends_on AS notice_end, c.ends_on AS contract_end
		FROM care_contract.contracts AS c JOIN care_contract.contract_notices AS n ON n.contract_id = c.id
		WHERE c.id = ?`, f.contract).Scan(txCtx, &row))
	return row.Notice, row.Contract
}

// setEnd is the store's end the Träger set on day.
func setEnd(day calendar.Date) ports.NoticeEnd {
	return ports.NoticeEnd{EndsOn: day, Decision: carecontract.EndDecisionSet, DecidedAt: noticeToday, Text: "Der Vertrag endet."}
}

var extraordinary = carecontract.NoticeInput{Kind: carecontract.NoticeTermination,
	TerminationKind: carecontract.TerminationExtraordinary, Reason: "  Wir ziehen nach Hamburg.  "}

func (f noticeFixture) parentDeclare(input carecontract.NoticeInput) (carecontract.NoticeOptions, error) {
	var options carecontract.NoticeOptions
	err := testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		input.ReceivedOn = f.notices.today()
		var err error
		options, err = f.notices.declare(txCtx, declaration{organizationID: f.org, accountID: f.reviewer.AccountID,
			channel: carecontract.NoticeChannelParents, contractID: f.contract, input: input, recipient: "erika@example.org",
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
	// Each statement of SetNoticeEnd shows on its own row, read in the same
	// transaction before its rollback: the declaration's end and the
	// contract's end of A stay open.
	err = beneath(func(txCtx context.Context) error {
		assert.ErrorIs(t, store.SetNoticeEnd(txCtx, other, f.contract, setEnd(end)), carecontract.ErrNoticeEndNotOpen, "B ends no declaration of A")
		notice, contract := f.endsWithin(t, txCtx)
		assert.Nil(t, notice, "the declaration's statement keeps to its Träger")
		assert.Nil(t, contract, "the contract's statement keeps to its Träger")
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
	require.NoError(t, beneath(func(txCtx context.Context) error {
		return store.MarkWithdrawalInformed(txCtx, other, f.contract, f.conclusion.At.AddDate(0, 0, -1))
	}))
	require.NoError(t, beneath(func(txCtx context.Context) error {
		var notice int64
		if err := f.db.NewRaw(`SELECT id FROM care_contract.contract_notices WHERE contract_id = ?`, f.contract).
			Scan(txCtx, &notice); err != nil {
			return err
		}
		if err := store.SetEndNoticeOutbox(txCtx, other, notice, 998); err != nil {
			return err
		}
		return store.SetNoticeOutbox(txCtx, other, notice, 999)
	}))
	// The refusal of the reason (#4682) shares SetNoticeEnd: B decides no
	// end of A, neither by setting it nor by refusing the reason.
	err = beneath(func(txCtx context.Context) error {
		refused := ports.NoticeEnd{EndsOn: end, Decision: carecontract.EndDecisionReinterpreted, DecidedAt: noticeToday, Text: "abgelehnt"}
		assert.ErrorIs(t, store.SetNoticeEnd(txCtx, other, f.contract, refused), carecontract.ErrNoticeEndNotOpen, "B refuses no reason of A")
		notice, contract := f.endsWithin(t, txCtx)
		assert.Nil(t, notice)
		assert.Nil(t, contract)
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)

	var row struct {
		EndsOn   *calendar.Date `bun:"ends_on,type:date"`
		Informed bool           `bun:"informed"`
		Outbox   *int64         `bun:"outbox_id"`
		EndMail  *int64         `bun:"end_notice_outbox_id"`
		Decision *string        `bun:"end_decision"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT c.ends_on, c.withdrawal_informed_at = ? AS informed, n.outbox_id, n.end_notice_outbox_id, n.end_decision
		FROM care_contract.contracts AS c JOIN care_contract.contract_notices AS n ON n.contract_id = c.id WHERE c.id = ?`,
		f.conclusion.At, f.contract).Scan(context.Background(), &row))
	assert.Nil(t, row.EndsOn, "A's open end stays open")
	assert.True(t, row.Informed, "A's notice provision stays")
	require.NotNil(t, row.Outbox)
	assert.NotEqual(t, int64(999), *row.Outbox, "A's confirmation mail stays")
	assert.Nil(t, row.EndMail, "B links no end notice to A's declaration")
	assert.Nil(t, row.Decision, "A's end stays undecided")
}

// The end of an extraordinary termination is set once (#4682). In the
// store, each statement keeps its own guard (ends_on IS NULL): a second end
// overwrites neither the declaration's nor the contract's day, each read in
// the transaction before its rollback. Red when either guard is dropped.
func TestNoticeEndIsSetOnceInTheStore(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Ende einmal", noticeToday)
	_, err := f.parentDeclare(extraordinary)
	require.NoError(t, err)
	store := postgres.NewStore()
	first, second := calendar.NewDate(2026, 12, 31), calendar.NewDate(2027, 1, 31)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		return store.SetNoticeEnd(txCtx, f.org, f.contract, setEnd(first))
	}))

	err = testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		assert.ErrorIs(t, store.SetNoticeEnd(txCtx, f.org, f.contract, setEnd(second)), carecontract.ErrNoticeEndNotOpen, "the end is set once")
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		notice, contract := f.endsWithin(t, txCtx)
		assert.Equal(t, &first, notice, "the declaration keeps its first end")
		assert.Equal(t, &first, contract, "the contract keeps its first end")
		return errRollback
	})
	require.ErrorIs(t, err, errRollback)
}

// The application refuses a second end on its own (openEnd), before the
// store is asked; red when that guard alone is dropped. It also refuses an
// end for any declaration but an extraordinary termination and an end
// before the day the declaration arrived.
func TestOpenEndRefusesAnEndAlreadySet(t *testing.T) {
	t.Parallel()
	received := calendar.NewDate(2026, 11, 10)
	end := calendar.NewDate(2026, 12, 31)
	terminated := ContractRecord{Summary: carecontract.ContractSummary{Status: carecontract.StatusTerminated}}
	open := carecontract.Notice{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationExtraordinary,
		ReceivedOn: received}

	notice, err := openEnd(terminated, &open, end)
	require.NoError(t, err)
	assert.Equal(t, &open, notice)

	set := open
	set.EndsOn = &end
	_, err = openEnd(terminated, &set, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an end already set is not set again")
	ordinary := open
	ordinary.TerminationKind = carecontract.TerminationOrdinary
	_, err = openEnd(terminated, &ordinary, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an ordinary termination has its end")
	_, err = openEnd(ContractRecord{Summary: carecontract.ContractSummary{Status: carecontract.StatusActive}}, &open, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "an active contract has no termination")
	_, err = openEnd(terminated, nil, end)
	assert.ErrorIs(t, err, carecontract.ErrNoticeEndNotOpen, "no declaration, no end")
	var invalid *carecontract.ValidationError
	_, err = openEnd(terminated, &open, received.AddDays(-1))
	assert.ErrorAs(t, err, &invalid, "no end before the receipt")
}
