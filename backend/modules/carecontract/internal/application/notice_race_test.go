package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Withdrawal and termination on the real store (#4682). The races run two
// real Träger transactions on the same contract; each goes red when the
// declaration (declare) reads the contract without its row lock: both then
// read it as aktiv and both record a declaration and a mail.

// noticeTemplates answers a fixed template per kind.
type noticeTemplates struct{}

func (noticeTemplates) NoticeTemplate(_ context.Context, _ int64, kind string) (string, error) {
	if kind == carecontract.NoticeWithdrawal {
		return "Widerruf für {Kind} am {Eingang}. {Träger}", nil
	}
	if kind == carecontract.NoticeTemplateEndSet {
		return "Ende für {Kind}: letzter Tag {Vertragsende}. {Träger}", nil
	}
	if kind == carecontract.NoticeTemplateReasonRejected {
		// Without {Vertragsende}: moto appends the last day.
		return "Grund nicht anerkannt für {Kind}, eingegangen am {Eingang}. {Träger}", nil
	}
	if kind == carecontract.TerminationExtraordinary {
		return "Außerordentliche Kündigung für {Kind}, eingegangen am {Eingang}, Grund: {Grund} {Träger}", nil
	}
	return "Kündigung für {Kind} zum {Vertragsende}, eingegangen am {Eingang}. {Träger}", nil
}

// noticePost counts the confirmation mails, safe for two transactions.
type noticePost struct {
	mu         sync.Mutex
	next       int64
	mails      []carecontract.NoticeMail
	deliveries []carecontract.NoticeDelivery
}

func (p *noticePost) EnqueueNoticeMail(_ context.Context, mail carecontract.NoticeMail) (int64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.next++
	p.mails = append(p.mails, mail)
	return p.next, nil
}

type noticeFixture struct {
	raceFixture
	notices    *Notices
	post       *noticePost
	addressees *noticeAddressees
}

// noticeAddressees answers the addresses the test grants the enrollment
// submission, safe for two transactions.
type noticeAddressees struct {
	mu        sync.Mutex
	addresses []string
}

func (a *noticeAddressees) PermittedAddresses(context.Context, int64, []int64) ([]string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.addresses...), nil
}

func (a *noticeAddressees) set(addresses ...string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.addresses = addresses
}

// noticeToday is the day the parents act: four days after the conclusion
// of 6 November 2026, inside the withdrawal period.
var noticeToday = time.Date(2026, 11, 10, 10, 0, 0, 0, time.UTC)

// newNoticeFixture is an active contract from 1 September 2026, one month
// notice to the month end, concluded on 6 November 2026 with the PDF that
// carries the withdrawal notice, and its declarations on day now.
func newNoticeFixture(t *testing.T, name string, now time.Time) noticeFixture {
	t.Helper()
	testpkg.SetupTestDB(t)
	race := newRaceFixture(t, name)
	race.contracts.clock = Clock{Now: func() time.Time { return race.conclusion.At }}
	require.NoError(t, testpkg.WithinTraeger(race.ctx, race.org, func(txCtx context.Context) error {
		if _, err := race.db.NewUpdate().Table("care_contract.contract_versions").Set("effective_from = ?", calendar.NewDate(2026, 9, 1)).
			Where("contract_id = ?", race.contract).Exec(txCtx); err != nil {
			return err
		}
		return race.conclude(txCtx)
	}))
	store, post := postgres.NewStore(), &noticePost{next: testpkg.UniqueTestTenantID(t)}
	require.NoError(t, testpkg.WithinTraeger(race.ctx, race.org, func(ctx context.Context) error {
		if _, err := race.db.NewUpdate().Table("care_contract.contracts").Set("student_id = ?", testpkg.UniqueTestTenantID(t)).
			Where("id = ?", race.contract).Exec(ctx); err != nil {
			return err
		}
		return store.MarkWithdrawalInformed(ctx, race.org, race.contract, race.conclusion.At)
	}))
	// The contract party still holds the enrollment submission for the child.
	addressees := &noticeAddressees{addresses: []string{"Erika@Example.org"}}
	notices := NewNotices(store, store, NoticePorts{Holidays: schoolHolidays{}, Templates: noticeTemplates{}, Post: post, Addressees: addressees},
		Clock{Now: func() time.Time { return now }}, race.contracts)
	return noticeFixture{raceFixture: race, notices: notices, post: post, addressees: addressees}
}

func (f noticeFixture) declare(input carecontract.NoticeInput) func(context.Context) error {
	return func(txCtx context.Context) error {
		_, err := f.notices.declare(txCtx, declaration{
			organizationID: f.org, accountID: f.reviewer.AccountID, channel: carecontract.NoticeChannelOffice,
			contractID: f.contract, input: input, traeger: "Ganztag im Quartier e.V.",
		})
		return err
	}
}

var (
	terminate = carecontract.NoticeInput{Kind: carecontract.NoticeTermination, TerminationKind: carecontract.TerminationOrdinary,
		ReceivedOn: calendar.NewDate(2026, 11, 10)}
	withdraw = carecontract.NoticeInput{Kind: carecontract.NoticeWithdrawal, ReceivedOn: calendar.NewDate(2026, 11, 10)}
)

// noticeState is the contract after the declarations.
type noticeState struct {
	status  string
	endsOn  *calendar.Date
	notices int
	events  int
}

func (f noticeFixture) state(t *testing.T) noticeState {
	t.Helper()
	ctx := context.Background()
	var s noticeState
	var row struct {
		Status string         `bun:"status"`
		EndsOn *calendar.Date `bun:"ends_on,type:date"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT status, ends_on FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(ctx, &row))
	s.status, s.endsOn = row.Status, row.EndsOn
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_notices WHERE contract_id = ?`, f.contract).Scan(ctx, &s.notices))
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM care_contract.contract_events WHERE contract_id = ?
		AND action IN ('terminate', 'withdraw')`, f.contract).Scan(ctx, &s.events))
	return s
}

func (f noticeFixture) versionsInForce(t *testing.T, from, to calendar.Date) []carecontract.VersionPeriod {
	t.Helper()
	var periods []carecontract.VersionPeriod
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		var err error
		periods, err = NewVersions(postgres.NewStore()).VersionsInForce(txCtx, f.org, from, to, f.contract)
		return err
	}))
	return periods
}

// Invariant: two terminations at once end the contract once. The second
// waits for the row lock, sees gekuendigt and gets ErrContractNotEndable;
// one declaration, one trail entry, one confirmation mail.
func TestTerminationAgainstTerminationLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Rennen Kündigung", noticeToday)
	won, lost := f.race(t, f.declare(terminate), f.declare(terminate))

	require.NoError(t, won)
	require.ErrorIs(t, lost, carecontract.ErrContractNotEndable)
	end := calendar.NewDate(2026, 12, 31)
	assert.Equal(t, noticeState{status: carecontract.StatusTerminated, endsOn: &end, notices: 1, events: 1}, f.state(t))
	assert.Len(t, f.post.mails, 1, "one confirmation")
}

// Invariant: a withdrawal while a termination holds the lock waits and is
// refused; the contract keeps the termination.
func TestWithdrawalAgainstTerminationLetsOneWin(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Rennen Widerruf", noticeToday)
	won, lost := f.race(t, f.declare(terminate), f.declare(withdraw))

	require.NoError(t, won)
	require.ErrorIs(t, lost, carecontract.ErrContractNotEndable)
	s := f.state(t)
	assert.Equal(t, carecontract.StatusTerminated, s.status)
	assert.Equal(t, 1, s.notices)
}

// Invariant: a termination ends the contract on its notice date. Before
// it, VersionsInForce answers the version into January; after it, only up
// to 31 December, so the monthly contribution (#4684) stops there.
func TestTerminationEndsTheVersionsInForceOnTheNoticeDate(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Kündigung Ende", noticeToday)
	from, to := calendar.NewDate(2026, 12, 1), calendar.NewDate(2027, 1, 31)
	before := f.versionsInForce(t, from, to)
	require.Len(t, before, 1)
	require.Equal(t, to, before[0].Until, "an active contract holds through January")

	var options carecontract.NoticeOptions
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		var err error
		options, err = f.notices.declare(txCtx, declaration{organizationID: f.org, accountID: f.reviewer.AccountID,
			channel: carecontract.NoticeChannelOffice, contractID: f.contract, input: terminate, traeger: "Ganztag im Quartier e.V."})
		return err
	}))
	end := calendar.NewDate(2026, 12, 31)
	require.NotNil(t, options.Notice)
	assert.Equal(t, &end, options.Notice.EndsOn)
	assert.Equal(t, "Kündigung für Lina Muster zum 31.12.2026, eingegangen am 10.11.2026. Ganztag im Quartier e.V.",
		options.Notice.ConfirmationText, "the confirmation as sent")
	require.Len(t, f.post.mails, 1)
	assert.Equal(t, "erika@example.org", f.post.mails[0].Recipient, "the office confirms to the contract party")

	after := f.versionsInForce(t, from, to)
	require.Len(t, after, 1)
	assert.Equal(t, from, after[0].From)
	assert.Equal(t, end, after[0].Until, "no day after the notice date")
	assert.Empty(t, f.versionsInForce(t, calendar.NewDate(2027, 1, 1), to), "nothing in January")

	var listed []carecontract.Version
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		var err error
		listed, err = postgres.NewStore().Versions(txCtx, f.org, f.contract)
		return err
	}))
	require.Len(t, listed, 1)
	assert.Equal(t, &end, listed[0].ValidUntil, "the version list shows the contract's end")
}

// Invariant: a withdrawal undoes the contract from its start:
// VersionsInForce answers no day at all, also before the withdrawal.
func TestWithdrawalUndoesTheContractFromItsStart(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger Widerruf Aufhebung", noticeToday)
	from, to := calendar.NewDate(2026, 9, 1), calendar.NewDate(2026, 12, 31)
	require.Len(t, f.versionsInForce(t, from, to), 1, "the concluded contract holds since September")

	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(withdraw)))

	assert.Empty(t, f.versionsInForce(t, from, to), "a withdrawn contract holds on no day")
	assert.Equal(t, noticeState{status: carecontract.StatusWithdrawn, notices: 1, events: 1}, f.state(t))
}

// Invariant: a withdrawal is open only within its period. After 20
// November it is refused; a contract concluded before #4682, whose PDF had
// no withdrawal notice, may still be withdrawn for 12 months and 14 days.
func TestWithdrawalOnlyWithinThePeriod(t *testing.T) {
	t.Parallel()
	late := time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)
	f := newNoticeFixture(t, "Träger Widerruf Frist", late)
	lateWithdrawal := withdraw
	lateWithdrawal.ReceivedOn = calendar.NewDate(2026, 11, 21)
	err := testpkg.WithinTraeger(f.ctx, f.org, f.declare(lateWithdrawal))
	require.ErrorIs(t, err, carecontract.ErrWithdrawalClosed)
	assert.Equal(t, carecontract.StatusActive, f.state(t).status, "the contract stays")

	var options carecontract.NoticeOptions
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(txCtx context.Context) error {
		record, err := f.notices.store.Contract(txCtx, f.org, f.contract, false)
		if err != nil {
			return err
		}
		options, err = f.notices.options(txCtx, f.org, record, f.notices.today(), false)
		return err
	}))
	assert.Nil(t, options.WithdrawalUntil, "the page offers no withdrawal after the period")
	assert.Equal(t, new(calendar.NewDate(2026, 12, 31)), options.OrdinaryEndsOn)

	_, err = f.db.ExecContext(context.Background(), `UPDATE care_contract.contracts SET withdrawal_informed_at = NULL WHERE id = ?`, f.contract)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(lateWithdrawal)), "without the notice the period runs a year longer")
	assert.Equal(t, carecontract.StatusWithdrawn, f.state(t).status)
}

// A declaration on a draft or a sent contract is refused, and a termination
// dated before the conclusion or in the future is invalid.
func TestDeclarationNeedsAnActiveContractAndAValidDay(t *testing.T) {
	t.Parallel()
	draft := newRaceFixture(t, "Träger Kündigung Entwurf")
	store := postgres.NewStore()
	notices := NewNotices(store, store, NoticePorts{Holidays: schoolHolidays{}, Templates: noticeTemplates{}, Post: &noticePost{}}, Clock{Now: func() time.Time { return noticeToday }},
		draft.contracts)
	err := testpkg.WithinTraeger(draft.ctx, draft.org, func(txCtx context.Context) error {
		_, err := notices.declare(txCtx, declaration{organizationID: draft.org, accountID: draft.reviewer.AccountID,
			channel: carecontract.NoticeChannelOffice, contractID: draft.contract, input: terminate})
		return err
	})
	require.ErrorIs(t, err, carecontract.ErrContractNotEndable)

	f := newNoticeFixture(t, "Träger Kündigung Tag", noticeToday)
	for _, day := range []calendar.Date{calendar.NewDate(2026, 11, 5), calendar.NewDate(2026, 11, 11), ""} {
		input := terminate
		input.ReceivedOn = day
		err := testpkg.WithinTraeger(f.ctx, f.org, f.declare(input))
		var invalid *carecontract.ValidationError
		require.True(t, errors.As(err, &invalid), fmt.Sprintf("received %q: %v", day, err))
		assert.Equal(t, "out_of_range", invalid.Fields["received_on"])
	}
	special := terminate
	special.TerminationKind = carecontract.TerminationSpecial
	require.ErrorIs(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(special)), carecontract.ErrSpecialTerminationClosed)
	assert.Equal(t, carecontract.StatusActive, f.state(t).status)
}

func TestDelayedConclusionStartsWithdrawalAtNoticeProvision(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger verspäteter Abschluss")
	enqueued := time.Date(2026, 12, 1, 10, 0, 0, 0, time.UTC)
	provided := enqueued.AddDate(0, 0, 9)
	f.contracts.clock = Clock{Now: func() time.Time { return enqueued }}
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.conclude))
	var informed *time.Time
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	assert.Nil(t, informed, "queueing is not provision")
	var outboxID int64
	require.NoError(t, f.db.NewRaw(`SELECT outbox_id FROM care_contract.contract_mailings WHERE contract_id = ?`, f.contract).Scan(f.ctx, &outboxID))
	post := &noticePost{deliveries: []carecontract.NoticeDelivery{{OutboxID: outboxID, ProvidedAt: provided}}}
	store := postgres.NewStore()
	notices := NewNotices(store, store, NoticePorts{Holidays: schoolHolidays{}, Templates: noticeTemplates{}, Post: post}, Clock{Now: func() time.Time { return provided }}, f.contracts)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		state, err := notices.noticeState(ctx, f.org, f.contract)
		require.NoError(t, err)
		assert.Equal(t, new(calendar.NewDate(2026, 12, 24)), withdrawalUntil(&f.conclusion, state.InformedAt, nil))
		return nil
	}))
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	require.NotNil(t, informed)
	assert.WithinDuration(t, provided, *informed, time.Second)
}

// deliveredRacePost reports the first send as complete so the conclusion
// can reuse the legacy PDF rather than rejecting a still-pending mail.
type deliveredRacePost struct{ *racePost }

func (p deliveredRacePost) ContractMailStates(ctx context.Context, organizationID, contractID int64) ([]carecontract.MailState, error) {
	states, err := p.racePost.ContractMailStates(ctx, organizationID, contractID)
	for i := range states {
		states[i].State = carecontract.MailSent
	}
	return states, err
}

func TestLegacyFrozenDocumentDoesNotClaimWithdrawalNotice(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger altes PDF")
	f.contracts.post = deliveredRacePost{f.contracts.post.(*racePost)}
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		return f.contracts.send(ctx, f.reviewer, f.contract, "")
	}))
	// A pre-migration sent PDF has no notice-provision evidence.
	_, err := f.db.ExecContext(f.ctx, `UPDATE care_contract.contracts SET withdrawal_informed_at = NULL WHERE id = ?`, f.contract)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.conclude))
	var informed *time.Time
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	assert.Nil(t, informed, "conclusion and resend of an old PDF prove no notice provision")
	assert.Equal(t, new(calendar.NewDate(2027, 11, 22)), withdrawalUntil(&f.conclusion, informed, nil), "20 November 2027 is a Saturday (§ 193 BGB)")
}

func TestOfficeOptionsUseTheReceiptDay(t *testing.T) {
	t.Parallel()
	today := time.Date(2027, 1, 5, 10, 0, 0, 0, time.UTC)
	f := newNoticeFixture(t, "Träger älterer Eingang", today)
	// One-month notice: in November the half-year ends earlier than July;
	// in January that special end is no longer earlier than the ordinary end.
	_, err := f.db.ExecContext(f.ctx, `UPDATE care_contract.contract_versions SET notice_to = 'school_year_end', special_termination = true WHERE contract_id = ?`, f.contract)
	require.NoError(t, err)
	now, err := f.notices.NoticeOptions(f.ctx, f.reviewer, f.contract, "")
	require.NoError(t, err)
	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), now.WithdrawalUntil, "the office must still record a timely postal dispatch")
	assert.Nil(t, now.SpecialEndsOn)
	earlier, err := f.notices.NoticeOptions(f.ctx, f.reviewer, f.contract, calendar.NewDate(2026, 11, 10))
	require.NoError(t, err)
	assert.Equal(t, new(calendar.NewDate(2026, 11, 20)), earlier.WithdrawalUntil)
	assert.Equal(t, new(calendar.NewDate(2027, 1, 31)), earlier.SpecialEndsOn)
	for _, day := range []calendar.Date{calendar.NewDate(2026, 11, 5), calendar.NewDate(2027, 1, 6)} {
		_, err := f.notices.NoticeOptions(f.ctx, f.reviewer, f.contract, day)
		var invalid *carecontract.ValidationError
		assert.ErrorAs(t, err, &invalid)
	}
}

func TestOfficeConfirmationUsesCorrectedMailingRecipient(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger korrigierte Bestätigung", noticeToday)
	_, err := f.db.ExecContext(f.ctx, `UPDATE care_contract.contract_mailings SET recipient_email = 'corrected@example.org' WHERE contract_id = ?`, f.contract)
	require.NoError(t, err)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(terminate)))
	require.Len(t, f.post.mails, 1)
	assert.Equal(t, "corrected@example.org", f.post.mails[0].Recipient)
}

func TestPostalWithdrawalUsesDispatchAndKeepsReceipt(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger rechtzeitig abgesendet", time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC))
	var input carecontract.NoticeInput
	require.NoError(t, json.Unmarshal([]byte(`{"kind":"widerruf","received_on":"2026-11-25","dispatched_on":"2026-11-19"}`), &input))
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(input)))
	assert.Equal(t, carecontract.StatusWithdrawn, f.state(t).status)
	var received, dispatched calendar.Date
	require.NoError(t, f.db.NewRaw(`SELECT received_on, dispatched_on FROM care_contract.contract_notices WHERE contract_id = ?`, f.contract).Scan(f.ctx, &received, &dispatched))
	assert.Equal(t, calendar.NewDate(2026, 11, 25), received)
	assert.Equal(t, calendar.NewDate(2026, 11, 19), dispatched)
}

func TestEnqueuedContractDoesNotProveWithdrawalNoticeProvision(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger wartende Belehrung")
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.conclude))
	var informed *time.Time
	require.NoError(t, f.db.NewRaw(`SELECT withdrawal_informed_at FROM care_contract.contracts WHERE id = ?`, f.contract).Scan(f.ctx, &informed))
	assert.Nil(t, informed, "an enqueued mail may fail without ever providing the notice")
}

func (p *noticePost) NoticeDeliveries(context.Context, int64, int64) ([]carecontract.NoticeDelivery, error) {
	return p.deliveries, nil
}

func TestParentConfirmationKeepsTheSuppliedRecipient(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger persönliche Bestätigung", noticeToday)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		_, err := f.notices.declare(ctx, declaration{organizationID: f.org, accountID: f.reviewer.AccountID,
			channel: carecontract.NoticeChannelParents, contractID: f.contract, input: terminate, recipient: "declaring-parent@example.org"})
		return err
	}))
	require.Len(t, f.post.mails, 1)
	assert.Equal(t, "declaring-parent@example.org", f.post.mails[0].Recipient)
}

func TestWithdrawalRejectsDispatchAfterReceiptAndChangedTerminationDate(t *testing.T) {
	t.Parallel()
	f := newNoticeFixture(t, "Träger unzulässige Erklärung", noticeToday)
	for _, raw := range []string{
		`{"kind":"widerruf","received_on":"2026-11-10","dispatched_on":"2026-11-11"}`,
		`{"kind":"widerruf","received_on":"2026-11-10","dispatched_on":"2026-11-05"}`,
		`{"kind":"kuendigung","termination_kind":"ordentlich","received_on":"2026-11-10","expected_ends_on":"2026-11-30"}`,
	} {
		var input carecontract.NoticeInput
		require.NoError(t, json.Unmarshal([]byte(raw), &input))
		var invalid *carecontract.ValidationError
		assert.ErrorAs(t, testpkg.WithinTraeger(f.ctx, f.org, f.declare(input)), &invalid)
		assert.Equal(t, carecontract.StatusActive, f.state(t).status)
	}
	assert.Empty(t, f.post.mails)
}

type failedRacePost struct{ *racePost }

func (p failedRacePost) ContractMailStates(ctx context.Context, organizationID, contractID int64) ([]carecontract.MailState, error) {
	states, err := p.racePost.ContractMailStates(ctx, organizationID, contractID)
	for i := range states {
		states[i].State = carecontract.MailFailed
	}
	return states, err
}

func TestFailedMailAndCorrectedResendWaitForActualProvision(t *testing.T) {
	t.Parallel()
	f := newRaceFixture(t, "Träger Belehrung nach Korrektur")
	now := f.conclusion.At
	f.contracts.clock = Clock{Now: func() time.Time { return now }}
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, f.conclude))
	f.contracts.post = failedRacePost{f.contracts.post.(*racePost)}
	now = now.AddDate(0, 0, 40)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		result, err := f.contracts.ResendContract(ctx, f.reviewer, f.contract, "corrected@example.org")
		require.NoError(t, err)
		require.Equal(t, []int64{f.contract}, result.Sent)
		return nil
	}))
	store, post := postgres.NewStore(), &noticePost{}
	notices := NewNotices(store, store, NoticePorts{Holidays: schoolHolidays{}, Templates: noticeTemplates{}, Post: post}, Clock{Now: func() time.Time { return now }}, f.contracts)
	require.NoError(t, testpkg.WithinTraeger(f.ctx, f.org, func(ctx context.Context) error {
		state, err := notices.noticeState(ctx, f.org, f.contract)
		require.NoError(t, err)
		assert.Nil(t, state.InformedAt, "a correction queued after 40 days has still provided no notice")
		mailings, err := store.Mailings(ctx, f.org, f.contract)
		require.NoError(t, err)
		require.Len(t, mailings, 2)
		assert.Equal(t, "corrected@example.org", mailings[0].Recipient)
		post.deliveries = []carecontract.NoticeDelivery{{OutboxID: mailings[0].OutboxID, ProvidedAt: now}}
		state, err = notices.noticeState(ctx, f.org, f.contract)
		require.NoError(t, err)
		assert.Equal(t, new(calendar.DateFromTime(now).AddDays(14)), withdrawalUntil(&f.conclusion, state.InformedAt, nil))
		return nil
	}))
}
