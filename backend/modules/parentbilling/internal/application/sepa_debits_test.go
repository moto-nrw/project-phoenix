package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The PAIN.008 file of a run (#4697, ADR 0181): only valid mandates, the
// account the release copied, FRST and RCUR apart, a download that changes
// nothing and a confirmation that collects once.

const sjrDebitCreditor = "DE98ZZZ09999999999"

// prefixCipher seals by prefixing the context, so a value opens only with
// the context it was sealed with.
type prefixCipher struct{}

func (prefixCipher) Seal(plaintext, context []byte) ([]byte, error) {
	return append(append(append([]byte{}, context...), '#'), plaintext...), nil
}

func (prefixCipher) Open(sealed, context []byte) ([]byte, error) {
	prefix := append(append([]byte{}, context...), '#')
	if !bytes.HasPrefix(sealed, prefix) {
		return nil, errors.New("wrong context")
	}
	return bytes.TrimPrefix(sealed, prefix), nil
}

type fakeLetterheads struct {
	account *parentbilling.LetterheadBankAccount
}

func (f fakeLetterheads) Letterhead(context.Context, parentbilling.HolidayReviewer) (parentbilling.Letterhead, error) {
	return parentbilling.Letterhead{BankAccount: f.account}, nil
}

func (fakeLetterheads) DocumentTemplate(context.Context, int64, string) (parentbilling.LetterTemplate, error) {
	return parentbilling.LetterTemplate{}, nil
}

type debitStoreFake struct {
	debits    []parentbilling.SealedSEPADebit
	creditor  string
	events    map[string][]int64
	submitted [][]int64
	collected []map[int64]calendar.Date
}

func (s *debitStoreFake) SEPADebitsOfRun(context.Context, int64, string, int64, bool, bool) ([]parentbilling.SealedSEPADebit, error) {
	out := make([]parentbilling.SealedSEPADebit, len(s.debits))
	copy(out, s.debits)
	for i := range out {
		out[i].CreditorNow = s.creditor
		// As the store's statement: another uncancelled debit of the same
		// one-off mandate claims its single authorization too.
		for j, other := range s.debits {
			if i != j && out[i].MandateKind == parentbilling.MandateOneOff && other.Debit.MandateID == out[i].Debit.MandateID &&
				(other.Debit.Status == parentbilling.DebitSubmitted || !other.Cancelled) {
				out[i].OneOffShared = true
			}
		}
	}
	return out, nil
}

func (s *debitStoreFake) SubmitSEPADebits(_ context.Context, _, _ int64, messageID string, debits []parentbilling.SEPADebit,
	collected map[int64]calendar.Date,
) error {
	ids := make([]int64, len(debits))
	for i, debit := range debits {
		ids[i] = debit.ID
	}
	s.submitted = append(s.submitted, ids)
	s.collected = append(s.collected, collected)
	for i := range s.debits {
		if slices.Contains(ids, s.debits[i].Debit.ID) {
			s.debits[i].Debit.Status, s.debits[i].Debit.MessageID = parentbilling.DebitSubmitted, messageID
			at := time.Date(2026, 11, 20, 9, 0, 0, 0, time.UTC)
			s.debits[i].Debit.SubmittedAt = &at
			for _, debit := range debits {
				if debit.ID == s.debits[i].Debit.ID {
					s.debits[i].Debit.AmountCents = debit.AmountCents
					s.debits[i].Debit.CollectOn = debit.CollectOn
				}
			}
		}
	}
	return nil
}

func (s *debitStoreFake) InsertSEPAMandateEvents(_ context.Context, _, _ int64, action, _ string, _ []string,
	mandates []parentbilling.SEPAMandate,
) error {
	if s.events == nil {
		s.events = map[string][]int64{}
	}
	for _, mandate := range mandates {
		s.events[action] = append(s.events[action], mandate.ID)
	}
	return nil
}

func (s *debitStoreFake) SEPACreditor(context.Context, int64) (*parentbilling.SEPACreditor, error) {
	return &parentbilling.SEPACreditor{CreditorID: s.creditor}, nil
}

// sealedDebit is a pending debit of the SJR run with its account sealed
// for the Träger 7.
func sealedDebit(id, mandateID int64, number string, cents int64, sequence, holder, iban string) parentbilling.SealedSEPADebit {
	reference := "SJR-" + number
	cipher := prefixCipher{}
	sealedIBAN, _ := cipher.Seal([]byte(iban), ibanContext(7, reference))
	sealedHolder, _ := cipher.Seal([]byte(holder), holderContext(7, reference))
	return parentbilling.SealedSEPADebit{
		Debit: parentbilling.SEPADebit{ID: id, MandateID: mandateID, ContractID: mandateID * 10, RunKind: parentbilling.DebitRunCare,
			RunID: 3, InvoiceNumber: number, AmountCents: cents, DueOn: calendar.NewDate(2027, 1, 1), SequenceType: sequence,
			MandateReference: reference, IBANMasked: "DE•• 30 00", Status: parentbilling.DebitPending},
		CreditorID: sjrDebitCreditor, MandateSignedOn: calendar.NewDate(2026, 9, 1), MandateKind: parentbilling.MandateRecurring,
		MandateSigned: calendar.NewDate(2026, 9, 1), Holder: sealedHolder, IBAN: sealedIBAN,
	}
}

func sjrDebits() *debitStoreFake {
	return &debitStoreFake{creditor: sjrDebitCreditor, debits: []parentbilling.SealedSEPADebit{
		sealedDebit(1, 11, "RE-2027-0001", 7800, parentbilling.SequenceTypeFirst, "Jörg Müller-Lüdenscheidt & Söhne", "DE89370400440532013000"),
		sealedDebit(2, 12, "RE-2027-0002", 4550, parentbilling.SequenceTypeRecurring, "Zoë Çelik", "DE02120300000000202051"),
		sealedDebit(3, 13, "RE-2027-0003", 2200, parentbilling.SequenceTypeOneOff, "Anna Becker", "AT611904300234573201"),
	}}
}

var sjrReviewer = parentbilling.HolidayReviewer{OrganizationID: 7, AccountID: 70}

func newSJRDebits(store *debitStoreFake) *SEPADebits {
	return NewSEPADebits(store, fakeLetterheads{account: &parentbilling.LetterheadBankAccount{
		AccountHolder: "Stadtjugendring Pforzheim e. V.", IBAN: "DE75 5121 0800 1245 1261 99", BIC: "SOLADES1PFO"}},
		prefixCipher{}, func() calendar.Date { return calendar.NewDate(2026, 11, 20) },
		func() time.Time { return time.Date(2026, 11, 20, 9, 30, 0, 0, time.UTC) })
}

func TestSEPADebitFileValidatesAgainstTheISOSchema(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	file, err := newSJRDebits(store).SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.Equal(t, "application/xml", file.ContentType)
	assert.True(t, strings.HasSuffix(file.Filename, ".xml"))
	testpkg.AssertXMLMatchesSchema(t, file.Data, filepath.Join("testdata", "sepa", "pain.008.001.08.xsd"))
	if dk := os.Getenv("SEPA_DK_XSD"); dk != "" {
		// The technical validation subset of the Deutsche Kreditwirtschaft
		// (GBIC_5) may not be redistributed; a local copy checks it too.
		testpkg.AssertXMLMatchesSchema(t, file.Data, dk)
	}
	xml := string(file.Data)
	for _, want := range []string{
		`<Document xmlns="urn:iso:std:iso:20022:tech:xsd:pain.008.001.08"`,
		"<NbOfTxs>3</NbOfTxs>", "<CtrlSum>145.50</CtrlSum>", "<CreDtTm>2026-11-20T10:30:00</CreDtTm>",
		"<SeqTp>FRST</SeqTp>", "<SeqTp>RCUR</SeqTp>", "<SeqTp>OOFF</SeqTp>", "<Cd>CORE</Cd>",
		"<ReqdColltnDt>2027-01-01</ReqdColltnDt>", "<Id>" + sjrDebitCreditor + "</Id>", "<Prtry>SEPA</Prtry>",
		"<IBAN>DE75512108001245126199</IBAN>", "<BICFI>SOLADES1PFO</BICFI>", "<Id>NOTPROVIDED</Id>",
		"<EndToEndId>RE-2027-0001</EndToEndId>", `<InstdAmt Ccy="EUR">78.00</InstdAmt>`, "<MndtId>SJR-RE-2027-0001</MndtId>",
		"<DtOfSgntr>2026-09-01</DtOfSgntr>", "<Nm>Jörg Müller-Lüdenscheidt &amp; Söhne</Nm>", "<Nm>Zoe Celik</Nm>",
		"<IBAN>DE89370400440532013000</IBAN>", "<Ustrd>Rechnung RE-2027-0001</Ustrd>",
	} {
		assert.Contains(t, xml, want)
	}
	assert.ElementsMatch(t, []int64{11, 12, 13}, store.events[mandateReveal], "each opened account is recorded in its trail")
	assert.Empty(t, store.submitted, "a download submits nothing")
}

// Only valid mandates: a revoked one, one of an earlier creditor identifier
// and an expired one leave their debit out, so does a cancelled or fully
// credited invoice; a partial credit lowers the amount.
func TestSEPADebitFileTakesOnlyValidMandates(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	revoked := sealedDebit(4, 14, "RE-2027-0004", 1000, parentbilling.SequenceTypeRecurring, "A", "DE89370400440532013000")
	revoked.MandateRevoked = true
	changed := sealedDebit(5, 15, "RE-2027-0005", 1000, parentbilling.SequenceTypeRecurring, "B", "DE89370400440532013000")
	changed.CreditorID = "DE98ZZZ01111111111"
	expired := sealedDebit(6, 16, "RE-2027-0006", 1000, parentbilling.SequenceTypeRecurring, "C", "DE89370400440532013000")
	expired.MandateSigned = calendar.NewDate(2023, 9, 1)
	last := calendar.NewDate(2023, 12, 31)
	expired.MandateLastCollectedOn = &last
	cancelled := sealedDebit(7, 17, "RE-2027-0007", 1000, parentbilling.SequenceTypeRecurring, "D", "DE89370400440532013000")
	cancelled.Cancelled = true
	credited := sealedDebit(8, 18, "RE-2027-0008", 1000, parentbilling.SequenceTypeRecurring, "E", "DE89370400440532013000")
	credited.CreditedCents = -1000
	partial := sealedDebit(9, 19, "RE-2027-0009", 1000, parentbilling.SequenceTypeRecurring, "F", "DE89370400440532013000")
	partial.CreditedCents = -400
	store.debits = append(store.debits, revoked, changed, expired, cancelled, credited, partial)

	run, err := newSJRDebits(store).SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	left := map[string]string{}
	for _, debit := range run.Left {
		left[debit.InvoiceNumber] = debit.Left
	}
	assert.Equal(t, map[string]string{
		"RE-2027-0004": parentbilling.DebitLeftRevoked, "RE-2027-0005": parentbilling.DebitLeftCreditorChanged,
		"RE-2027-0006": parentbilling.DebitLeftExpired, "RE-2027-0007": parentbilling.DebitLeftCancelled,
		"RE-2027-0008": parentbilling.DebitLeftCancelled,
	}, left)
	assert.Equal(t, 4, run.Count)
	assert.Equal(t, int64(7800+4550+2200+600), run.TotalCents, "the partial credit lowers the debit")
	assert.Equal(t, "DE•• •••• •••• •••• ••61 99", run.CreditorIBAN, "the account of the Träger only masked")

	file, err := newSJRDebits(store).SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	for _, number := range []string{"0004", "0005", "0006", "0007", "0008"} {
		assert.NotContains(t, string(file.Data), "RE-2027-"+number)
	}
	assert.Contains(t, string(file.Data), `<InstdAmt Ccy="EUR">6.00</InstdAmt>`)
	assert.NotContains(t, store.events[mandateReveal], int64(14), "a left-out mandate is not opened")
}

// A download changes nothing: twice the same file with the same message ID,
// no debit submitted, no FRST turned to RCUR.
func TestSEPADebitDownloadTwiceGivesTheSameFile(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	first, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	second, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.Equal(t, first.Data, second.Data)
	assert.Equal(t, first.Filename, second.Filename)
	assert.Empty(t, store.submitted)
	assert.Equal(t, parentbilling.SequenceTypeFirst, store.debits[0].Debit.SequenceType)
}

// The confirmation collects exactly the file the office saw: once, with
// each mandate's collection day, and a second confirmation changes nothing.
func TestSEPADebitSubmissionCollectsOnce(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := newSJRDebits(store)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)

	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, "MOTO-M3-0000000000000000")
	require.ErrorIs(t, err, parentbilling.ErrSEPADebitsChanged, "another file is not this run's file")
	assert.Empty(t, store.submitted)

	after, err := debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err)
	require.Len(t, store.submitted, 1)
	assert.ElementsMatch(t, []int64{1, 2, 3}, store.submitted[0])
	assert.Equal(t, map[int64]calendar.Date{11: calendar.NewDate(2027, 1, 1), 12: calendar.NewDate(2027, 1, 1),
		13: calendar.NewDate(2027, 1, 1)}, store.collected[0])
	assert.ElementsMatch(t, []int64{11, 12, 13}, store.events[mandateCollect])
	assert.Len(t, after.Submitted, 3)
	assert.Zero(t, after.Count)
	require.NotNil(t, after.SubmittedAt)

	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.ErrorIs(t, err, parentbilling.ErrSEPADebitsEmpty, "a second confirmation collects nothing")
	assert.Len(t, store.submitted, 1)
}

// A debit whose due date passed is collected tomorrow: a bank refuses a
// collection day in the past. The changed day gives a new message ID.
func TestSEPADebitPastDueIsCollectedTomorrow(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	before, err := newSJRDebits(store).SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	store.debits[0].Debit.DueOn = calendar.NewDate(2026, 11, 1)
	run, err := newSJRDebits(store).SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.Equal(t, 1, run.PastDue)
	assert.Equal(t, calendar.NewDate(2026, 11, 21), run.Groups[0].CollectOn)
	assert.NotEqual(t, before.MessageID, run.MessageID)
}

// Without a default bank account of the Träger there is no file; the
// overview says so.
func TestSEPADebitFileNeedsTheAccountOfTheTraeger(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	debits := NewSEPADebits(store, fakeLetterheads{}, prefixCipher{}, func() calendar.Date { return calendar.NewDate(2026, 11, 20) }, time.Now)
	_, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.ErrorIs(t, err, parentbilling.ErrSEPACreditorAccountMissing)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.True(t, run.AccountMissing)
	_, err = debits.SEPADebitRun(context.Background(), sjrReviewer, "school", 3)
	require.ErrorIs(t, err, parentbilling.ErrSEPADebitRunNotFound)
}

func TestSEPATextKeepsTheCharacterSetOfTheDeutscheKreditwirtschaft(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "Jörg Müller & Söhne", sepaText("  Jörg\tMüller & Söhne ", 70))
	assert.Equal(t, "Francois Celik", sepaText("François Çelik", 70))
	assert.Equal(t, "Ab", sepaText("Ab€", 70))
	assert.Equal(t, "RE-2027-0001", sepaIdentifier("RE-2027-0001"))
	assert.Equal(t, "RE-2027-0001", sepaIdentifier("RE_2027 0001"))
	assert.Equal(t, "RE-2027-0001-", sepaIdentifier("RE-2027-0001_"))
	assert.Equal(t, "A/B", sepaIdentifier("/A//B"))
	assert.Len(t, sepaIdentifier(strings.Repeat("X", 40)), 35)
}

func TestSEPADebitOneOffCannotBeUsedTwice(t *testing.T) {
	t.Parallel()
	for _, consumed := range []bool{false, true} {
		t.Run(fmt.Sprint(consumed), func(t *testing.T) {
			store := sjrDebits()
			store.debits = store.debits[:2]
			for i := range store.debits {
				store.debits[i].MandateKind = parentbilling.MandateOneOff
				store.debits[i].Debit.MandateID = 11
				store.debits[i].Debit.SequenceType = parentbilling.SequenceTypeOneOff
				if consumed {
					on := calendar.NewDate(2026, 11, 1)
					store.debits[i].MandateLastCollectedOn = &on
				}
			}
			run, err := newSJRDebits(store).SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
			require.NoError(t, err)
			assert.Zero(t, run.Count, "an already used or ambiguous one-off mandate authorizes no debit")
			assert.Len(t, run.Left, 2)
		})
	}
}

func TestSEPADebitFingerprintIncludesCreditorAccount(t *testing.T) {
	t.Parallel()
	debits := newSJRDebits(sjrDebits())
	before, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	debits.letterheads = fakeLetterheads{account: &parentbilling.LetterheadBankAccount{
		AccountHolder: "Anderer Träger", IBAN: "AT611904300234573201", BIC: "BKAUATWW"}}
	after, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.NotEqual(t, before.MessageID, after.MessageID)
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, before.MessageID)
	require.ErrorIs(t, err, parentbilling.ErrSEPADebitsChanged)
}

func TestSEPADebitConfirmationKeepsDownloadedDates(t *testing.T) {
	t.Parallel()
	store := sjrDebits()
	store.debits[0].Debit.DueOn = calendar.NewDate(2026, 11, 1)
	debits := newSJRDebits(store)
	run, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	debits.today = func() calendar.Date { return calendar.NewDate(2026, 11, 22) }
	_, err = debits.SubmitSEPADebits(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3, run.MessageID)
	require.NoError(t, err, "confirmation after midnight must still identify the file accepted by the bank")
	require.Len(t, store.collected, 1)
	assert.Equal(t, calendar.NewDate(2026, 11, 21), store.collected[0][11])
}

func TestSEPADebitFileRejectsEmptyConvertedNames(t *testing.T) {
	t.Parallel()
	for _, creditor := range []bool{false, true} {
		t.Run(fmt.Sprint(creditor), func(t *testing.T) {
			store := sjrDebits()
			debits := newSJRDebits(store)
			if creditor {
				debits.letterheads = fakeLetterheads{account: &parentbilling.LetterheadBankAccount{
					AccountHolder: "李明", IBAN: "DE89370400440532013000"}}
			} else {
				store.debits[0] = sealedDebit(1, 11, "RE-2027-0001", 7800, parentbilling.SequenceTypeFirst, "李明", "DE89370400440532013000")
			}
			_, err := debits.SEPADebitFile(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
			require.Error(t, err, "a name empty after conversion makes the XML invalid")
		})
	}
}

func TestSEPADebitFutureDatesKeepTheirIdentityAcrossMidnight(t *testing.T) {
	t.Parallel()
	debits := newSJRDebits(sjrDebits())
	before, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	debits.today = func() calendar.Date { return calendar.NewDate(2026, 11, 21) }
	after, err := debits.SEPADebitRun(context.Background(), sjrReviewer, parentbilling.DebitRunCare, 3)
	require.NoError(t, err)
	assert.Equal(t, before.MessageID, after.MessageID, "an unchanged collection date must retain the bank's duplicate detection ID")
	assert.LessOrEqual(t, len(after.MessageID), maxSEPAIdentifier)
}
