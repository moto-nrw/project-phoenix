package application

import (
	"context"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/sepa"
)

// The SEPA direct debit file of an issued run (#4697, ADR 0181). The file
// of a run is a pure function of its pending debits, the state of their
// mandates and the day: the same debits give the same file with the same
// message ID. Only the confirmation of its submission changes anything.

// mandateCollect is the trail action of a collection (#4697).
const mandateCollect = "collect"

// DebitStore persists the debits. Every method runs in the Träger
// transaction of organizationID.
type DebitStore interface {
	// SEPADebitsOfRun reads the debits of a released run, with the sealed
	// account when sealed is set, locked when lock is set.
	SEPADebitsOfRun(ctx context.Context, organizationID int64, runKind string, runID int64, sealed, lock bool) ([]parentbilling.SealedSEPADebit, error)
	// SubmitSEPADebits marks debits submitted and collects their mandates.
	SubmitSEPADebits(ctx context.Context, organizationID, accountID int64, messageID string, debitIDs []int64,
		collected map[int64]calendar.Date) error
	// InsertSEPAMandateEvents records one action for each mandate.
	InsertSEPAMandateEvents(ctx context.Context, organizationID, accountID int64, action, source string, fields []string,
		mandates []parentbilling.SEPAMandate) error
	// SEPACreditor reads the creditor of the Träger, nil without one.
	SEPACreditor(ctx context.Context, organizationID int64) (*parentbilling.SEPACreditor, error)
}

// SEPADebits serves parentbilling.SEPADebits.
type SEPADebits struct {
	store       DebitStore
	letterheads parentbilling.BillingLetterheads
	cipher      parentbilling.BankDataCipher
	today       func() calendar.Date
	now         func() time.Time
}

var _ parentbilling.SEPADebits = (*SEPADebits)(nil)

// NewSEPADebits builds the debit file.
func NewSEPADebits(store DebitStore, letterheads parentbilling.BillingLetterheads, cipher parentbilling.BankDataCipher,
	today func() calendar.Date, now func() time.Time,
) *SEPADebits {
	if store == nil || letterheads == nil || cipher == nil || today == nil || now == nil {
		panic("parent billing: the store, the letterhead, the cipher and the clocks are required")
	}
	return &SEPADebits{store: store, letterheads: letterheads, cipher: cipher, today: today, now: now}
}

// debitPlan is the file of a run as it is now.
type debitPlan struct {
	run      parentbilling.SEPADebitRun
	included []parentbilling.SealedSEPADebit
	// collectOn is the collection day of each included debit.
	collectOn map[int64]calendar.Date
}

func validRunKind(kind string) error {
	if kind != parentbilling.DebitRunCare && kind != parentbilling.DebitRunHoliday {
		return parentbilling.ErrSEPADebitRunNotFound
	}
	return nil
}

// plan sorts the debits of a run into the file, the submitted ones and
// those left out, and names the file. A pending debit is collected on its
// due date, at the earliest tomorrow: a bank refuses a day that passed.
func (d *SEPADebits) plan(runKind string, runID int64, debits []parentbilling.SealedSEPADebit) debitPlan {
	plan := debitPlan{collectOn: map[int64]calendar.Date{}, run: parentbilling.SEPADebitRun{RunKind: runKind, RunID: runID,
		Groups: []parentbilling.SEPADebitGroup{}, Debits: []parentbilling.SEPADebit{}, Submitted: []parentbilling.SEPADebit{},
		Left: []parentbilling.SEPADebit{}}}
	earliest := d.today().AddDays(1)
	groups := map[[2]string]*parentbilling.SEPADebitGroup{}
	for _, entry := range debits {
		debit := entry.Debit
		if debit.Status == parentbilling.DebitSubmitted {
			plan.run.Submitted = append(plan.run.Submitted, debit)
			if plan.run.SubmittedAt == nil || debit.SubmittedAt.After(*plan.run.SubmittedAt) {
				plan.run.SubmittedAt = debit.SubmittedAt
			}
			continue
		}
		debit.AmountCents += entry.CreditedCents
		collectOn := max(debit.DueOn, earliest)
		if debit.Left = leftReason(entry, debit.AmountCents, collectOn); debit.Left != "" {
			plan.run.Left = append(plan.run.Left, debit)
			continue
		}
		if debit.DueOn.Before(earliest) {
			plan.run.PastDue++
		}
		entry.Debit = debit
		plan.included = append(plan.included, entry)
		plan.collectOn[debit.ID] = collectOn
		plan.run.Debits = append(plan.run.Debits, debit)
		plan.run.Count++
		plan.run.TotalCents += debit.AmountCents
		key := [2]string{debit.SequenceType, collectOn.String()}
		if groups[key] == nil {
			groups[key] = &parentbilling.SEPADebitGroup{SequenceType: debit.SequenceType, CollectOn: collectOn}
		}
		groups[key].Count++
		groups[key].TotalCents += debit.AmountCents
	}
	for _, group := range groups {
		plan.run.Groups = append(plan.run.Groups, *group)
	}
	slices.SortFunc(plan.run.Groups, func(a, b parentbilling.SEPADebitGroup) int {
		return batchOrder(a.CollectOn, b.CollectOn, a.SequenceType, b.SequenceType)
	})
	plan.run.MessageID = messageIDOf(runKind, runID, plan)
	return plan
}

// batchOrder sorts batches by collection day, then by sequence type.
func batchOrder(dayA, dayB calendar.Date, sequenceA, sequenceB string) int {
	if order := dayA.Compare(dayB); order != 0 {
		return order
	}
	return strings.Compare(sequenceA, sequenceB)
}

// leftReason tells why a pending debit stays out of the file: its invoice
// was cancelled or credited in full, its mandate was revoked, it belongs to
// an earlier creditor identifier or the mandate expired by the collection.
func leftReason(entry parentbilling.SealedSEPADebit, amountCents int64, collectOn calendar.Date) string {
	switch {
	case entry.Cancelled || amountCents <= 0:
		return parentbilling.DebitLeftCancelled
	case entry.MandateRevoked:
		return parentbilling.DebitLeftRevoked
	case entry.CreditorID != entry.CreditorNow:
		return parentbilling.DebitLeftCreditorChanged
	case !parentbilling.MandateExpiresOn(entry.MandateSigned, parentbilling.MandateUsedOn(entry.MandateSequence,
		entry.MandateLastCollectedOn, entry.MandateCreatedOn)).After(collectOn):
		return parentbilling.DebitLeftExpired
	}
	return ""
}

// messageIDOf names a file by what it collects: the same debits with the
// same amounts, sequences and days give the same ID, so a bank refuses the
// second submission of one file, and a changed file gets a new ID.
func messageIDOf(runKind string, runID int64, plan debitPlan) string {
	if len(plan.included) == 0 {
		return ""
	}
	hash := fnv.New64a()
	for _, entry := range plan.included {
		_, _ = fmt.Fprintf(hash, "%d|%d|%s|%s;", entry.Debit.ID, entry.Debit.AmountCents, entry.Debit.SequenceType,
			plan.collectOn[entry.Debit.ID])
	}
	kind := "M"
	if runKind == parentbilling.DebitRunHoliday {
		kind = "F"
	}
	return fmt.Sprintf("MOTO-%s%d-%016x", kind, runID, hash.Sum64())
}

// load reads the debits of a run and the creditor and plans the file.
func (d *SEPADebits) load(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64, sealed, lock bool,
) (debitPlan, error) {
	if err := validReviewer(reviewer); err != nil {
		return debitPlan{}, err
	}
	if err := validRunKind(runKind); err != nil || runID <= 0 {
		return debitPlan{}, parentbilling.ErrSEPADebitRunNotFound
	}
	debits, err := d.store.SEPADebitsOfRun(ctx, reviewer.OrganizationID, runKind, runID, sealed, lock)
	if err != nil {
		return debitPlan{}, err
	}
	plan := d.plan(runKind, runID, debits)
	creditor, err := d.store.SEPACreditor(ctx, reviewer.OrganizationID)
	if err != nil {
		return debitPlan{}, err
	}
	if creditor != nil {
		plan.run.CreditorID = creditor.CreditorID
	}
	return plan, nil
}

// SEPADebitRun answers the file of a run without bank data of a family.
func (d *SEPADebits) SEPADebitRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) (parentbilling.SEPADebitRun, error) {
	plan, err := d.load(ctx, reviewer, runKind, runID, false, false)
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	account, err := d.creditorAccount(ctx, reviewer)
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if account == nil {
		plan.run.AccountMissing = true
	} else {
		plan.run.CreditorName, plan.run.CreditorIBAN = account.AccountHolder, sepa.MaskIBAN(sepa.Compact(account.IBAN))
	}
	return plan.run, nil
}

// SEPADebitFile opens the account of each debit of the file and writes it.
// Every mandate whose account it opens gets a reveal in its trail.
func (d *SEPADebits) SEPADebitFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) (parentbilling.SEPADebitFile, error) {
	plan, err := d.load(ctx, reviewer, runKind, runID, true, false)
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	if len(plan.included) == 0 {
		return parentbilling.SEPADebitFile{}, parentbilling.ErrSEPADebitsEmpty
	}
	account, err := d.creditorAccount(ctx, reviewer)
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	if account == nil {
		return parentbilling.SEPADebitFile{}, parentbilling.ErrSEPACreditorAccountMissing
	}
	batches, revealed, err := d.openBatches(reviewer.OrganizationID, plan)
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	if err := d.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateReveal,
		parentbilling.MandateFromOffice, nil, revealed); err != nil {
		return parentbilling.SEPADebitFile{}, fmt.Errorf("parent billing: record the reveal of the SEPA mandates: %w", err)
	}
	data, err := renderPain008(debitFileHead{MessageID: plan.run.MessageID, CreatedAt: d.now().In(berlin()),
		CreditorName: account.AccountHolder, CreditorIBAN: sepa.Compact(account.IBAN), CreditorBIC: sepa.Compact(account.BIC),
		CreditorID: plan.run.CreditorID}, batches)
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	return parentbilling.SEPADebitFile{Filename: plan.run.MessageID + ".xml", ContentType: "application/xml", Data: data}, nil
}

// creditorAccount is the default bank account of the Träger the debits
// are credited to, nil without one. The letterhead reads it from the master
// data of Organisation & Tenancy, which logs the read.
func (d *SEPADebits) creditorAccount(ctx context.Context, reviewer parentbilling.HolidayReviewer) (*parentbilling.LetterheadBankAccount, error) {
	head, err := d.letterheads.Letterhead(ctx, reviewer)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the bank account of the Träger: %w", err)
	}
	return head.BankAccount, nil
}

// berlin is the zone of the creation time of the file.
func berlin() *time.Location {
	if location, err := time.LoadLocation("Europe/Berlin"); err == nil {
		return location
	}
	return time.UTC
}

// openBatches opens the account of each included debit and groups the
// debits into batches; it answers the mandates it opened, once each.
func (d *SEPADebits) openBatches(organizationID int64, plan debitPlan) ([]debitFileBatch, []parentbilling.SEPAMandate, error) {
	var batches []debitFileBatch
	index := map[[2]string]int{}
	var revealed []parentbilling.SEPAMandate
	for _, entry := range plan.included {
		debit := entry.Debit
		iban, err := d.cipher.Open(entry.IBAN, ibanContext(organizationID, debit.MandateReference))
		if err != nil {
			return nil, nil, fmt.Errorf("parent billing: open the IBAN of debit %d: %w", debit.ID, err)
		}
		holder, err := d.cipher.Open(entry.Holder, holderContext(organizationID, debit.MandateReference))
		if err != nil {
			return nil, nil, fmt.Errorf("parent billing: open the account holder of debit %d: %w", debit.ID, err)
		}
		collectOn := plan.collectOn[debit.ID]
		key := [2]string{debit.SequenceType, collectOn.String()}
		at, ok := index[key]
		if !ok {
			at = len(batches)
			index[key] = at
			batches = append(batches, debitFileBatch{SequenceType: debit.SequenceType, CollectOn: collectOn,
				ID: sepaIdentifier(fmt.Sprintf("%s-%d", plan.run.MessageID[:min(len(plan.run.MessageID), 30)], at+1))})
		}
		batches[at].Entries = append(batches[at].Entries, debitFileEntry{
			EndToEndID: sepaIdentifier(debit.InvoiceNumber), AmountCents: debit.AmountCents,
			MandateReference: debit.MandateReference, MandateSignedOn: entry.MandateSignedOn,
			DebtorName: string(holder), DebtorIBAN: string(iban), DebtorBIC: entry.BIC,
			Remittance: "Rechnung " + debit.InvoiceNumber,
		})
		if !slices.ContainsFunc(revealed, func(m parentbilling.SEPAMandate) bool { return m.ID == debit.MandateID }) {
			revealed = append(revealed, parentbilling.SEPAMandate{ID: debit.MandateID, ContractID: debit.ContractID})
		}
	}
	slices.SortStableFunc(batches, func(a, b debitFileBatch) int {
		return batchOrder(a.CollectOn, b.CollectOn, a.SequenceType, b.SequenceType)
	})
	return batches, revealed, nil
}

// SubmitSEPADebits confirms that the office submitted the file messageID:
// in the one transaction its debits become submitted, each mandate gets its
// last collection, a recurring one turns from FRST to RCUR, its other
// pending FRST debits too, and the trail records the collection. A file
// that is no longer the run's file is ErrSEPADebitsChanged; one without a
// pending debit ErrSEPADebitsEmpty, so a second confirmation changes
// nothing.
func (d *SEPADebits) SubmitSEPADebits(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
	messageID string,
) (parentbilling.SEPADebitRun, error) {
	plan, err := d.load(ctx, reviewer, runKind, runID, false, true)
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if len(plan.included) == 0 {
		return parentbilling.SEPADebitRun{}, parentbilling.ErrSEPADebitsEmpty
	}
	if strings.TrimSpace(messageID) != plan.run.MessageID {
		return parentbilling.SEPADebitRun{}, parentbilling.ErrSEPADebitsChanged
	}
	ids := make([]int64, len(plan.included))
	collected := map[int64]calendar.Date{}
	var mandates []parentbilling.SEPAMandate
	for i, entry := range plan.included {
		ids[i] = entry.Debit.ID
		on := plan.collectOn[entry.Debit.ID]
		if last, ok := collected[entry.Debit.MandateID]; !ok || on.After(last) {
			if !ok {
				mandates = append(mandates, parentbilling.SEPAMandate{ID: entry.Debit.MandateID, ContractID: entry.Debit.ContractID})
			}
			collected[entry.Debit.MandateID] = on
		}
	}
	if err := d.store.SubmitSEPADebits(ctx, reviewer.OrganizationID, reviewer.AccountID, plan.run.MessageID, ids, collected); err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if err := d.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateCollect,
		parentbilling.MandateFromOffice, nil, mandates); err != nil {
		return parentbilling.SEPADebitRun{}, fmt.Errorf("parent billing: record the collection of the SEPA mandates: %w", err)
	}
	return d.SEPADebitRun(ctx, reviewer, runKind, runID)
}
