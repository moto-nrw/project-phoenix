package application

import (
	"context"
	"fmt"
	"slices"
	"strconv"
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
	noticeRateStore
	// ExportFileStore records each downloaded file in the export protocol
	// with the next file number of the Träger (#4693).
	ExportFileStore
	// SEPADebitsOfRun reads the debits of a released run, with the sealed
	// account when sealed is set, locked when lock is set.
	SEPADebitsOfRun(ctx context.Context, organizationID int64, runKind string, runID int64, sealed, lock bool) ([]parentbilling.SealedSEPADebit, error)
	// SubmitSEPADebits marks debits submitted and collects their mandates.
	SubmitSEPADebits(ctx context.Context, organizationID, accountID int64, messageID string, debits []parentbilling.SEPADebit,
		collected map[int64]calendar.Date) error
	// InsertSEPAMandateEvents records one action for each mandate.
	InsertSEPAMandateEvents(ctx context.Context, organizationID, accountID int64, action, source string, fields []string,
		mandates []parentbilling.SEPAMandate) error
	// SEPACreditor reads the creditor of the Träger, nil without one.
	SEPACreditor(ctx context.Context, organizationID int64) (*parentbilling.SEPACreditor, error)
	// SaveSEPADebitFile keeps the downloaded file of a message (ADR 0187),
	// replacing an earlier download until the message is submitted.
	SaveSEPADebitFile(ctx context.Context, organizationID, accountID int64, file parentbilling.StoredSEPADebitFile) error
	// SubmitSEPADebitFile marks the stored file of a message submitted; a
	// message without a stored download keeps the creditor account of file.
	// It drops the unsubmitted downloads of the run that share a member
	// with file: the confirmation superseded them.
	SubmitSEPADebitFile(ctx context.Context, organizationID, accountID int64, file parentbilling.StoredSEPADebitFile) error
}

// SEPADebits serves parentbilling.SEPADebits.
type SEPADebits struct {
	store       DebitStore
	letterheads parentbilling.BillingLetterheads
	cipher      parentbilling.BankDataCipher
	today       func() calendar.Date
	now         func() time.Time
	// fingerprint is the platform's SHA-256 of a stored file (ADR 0187).
	fingerprint func([]byte) string
}

var _ parentbilling.SEPADebits = (*SEPADebits)(nil)

// NewSEPADebits builds the debit file.
func NewSEPADebits(store DebitStore, letterheads parentbilling.BillingLetterheads, cipher parentbilling.BankDataCipher,
	today func() calendar.Date, now func() time.Time, fingerprint func([]byte) string,
) *SEPADebits {
	if store == nil || letterheads == nil || cipher == nil || today == nil || now == nil || fingerprint == nil {
		panic("parent billing: the store, the letterhead, the cipher, the clocks and the fingerprint are required")
	}
	return &SEPADebits{store: store, letterheads: letterheads, cipher: cipher, today: today, now: now, fingerprint: fingerprint}
}

// debitPlan is the file of a run as it is now.
type debitPlan struct {
	account  *parentbilling.LetterheadBankAccount
	run      parentbilling.SEPADebitRun
	included []parentbilling.SealedSEPADebit
	// collectOn is the collection day of each included debit.
	collectOn map[int64]calendar.Date
}

func validRunKind(kind string) error {
	if kind != parentbilling.DebitRunCare && kind != parentbilling.DebitRunHoliday && kind != parentbilling.DebitRunNotice {
		return parentbilling.ErrSEPADebitRunNotFound
	}
	return nil
}

// plan sorts the debits of a run into the file, the submitted ones and
// those left out, and names the file. A pending debit is collected on its
// due date, at the earliest tomorrow: a bank refuses a day that passed.
func (d *SEPADebits) plan(runKind string, runID int64, debits []parentbilling.SealedSEPADebit, day calendar.Date) debitPlan {
	plan := debitPlan{collectOn: map[int64]calendar.Date{}, run: parentbilling.SEPADebitRun{RunKind: runKind, RunID: runID,
		Groups: []parentbilling.SEPADebitGroup{}, Debits: []parentbilling.SEPADebit{}, Submitted: []parentbilling.SEPADebit{},
		Left: []parentbilling.SEPADebit{}}}
	earliest := day.AddDays(1)
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
		debit.Left = pendingDebitLeft(entry, debit.AmountCents, collectOn)
		if debit.Left != "" {
			plan.run.Left = append(plan.run.Left, debit)
			continue
		}
		if debit.DueOn.Before(earliest) {
			plan.run.PastDue++
		}
		debit.CollectOn = collectOn
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
	case entry.Debit.MandateID == 0:
		return parentbilling.DebitLeftNoMandate
	case (entry.MandateKind == parentbilling.MandateOneOff || entry.Debit.SequenceType == parentbilling.SequenceTypeOneOff) && entry.MandateLastCollectedOn != nil:
		return parentbilling.DebitLeftOneOffUsed
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

// load reads the debits of a run and the creditor and plans the file.
func (d *SEPADebits) load(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64, sealed, lock bool, day calendar.Date,
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
	rates, err := noticeRates(ctx, d.store, reviewer.OrganizationID, runKind, runID, debits, sealed)
	if err != nil {
		return debitPlan{}, err
	}
	plan := d.plan(runKind, runID, append(debits, rates...), day)
	creditor, err := d.store.SEPACreditor(ctx, reviewer.OrganizationID)
	if err != nil {
		return debitPlan{}, err
	}
	if creditor != nil {
		plan.run.CreditorID = creditor.CreditorID
	}
	plan.account, err = d.creditorAccount(ctx, reviewer)
	if err != nil {
		return debitPlan{}, err
	}
	plan.run.MessageID = messageIDOf(runKind, runID, plan)
	return plan, nil
}

// SEPADebitRun answers the file of a run without bank data of a family.
func (d *SEPADebits) SEPADebitRun(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
) (parentbilling.SEPADebitRun, error) {
	plan, err := d.load(ctx, reviewer, runKind, runID, false, false, d.today())
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	account := plan.account
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
	plan, err := d.load(ctx, reviewer, runKind, runID, true, false, d.today())
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	if len(plan.included) == 0 {
		return parentbilling.SEPADebitFile{}, parentbilling.ErrSEPADebitsEmpty
	}
	account := plan.account
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
	if err := d.storeFile(ctx, reviewer, runKind, runID, plan, data); err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	entry, err := d.recordDebitFile(ctx, reviewer, runKind, runID, plan, data)
	if err != nil {
		return parentbilling.SEPADebitFile{}, err
	}
	return parentbilling.SEPADebitFile{Filename: entry.FileName, ContentType: "application/xml", Data: data, ExportFileID: entry.ID}, nil
}

// storeFile keeps the exact bytes the office downloads, sealed with the
// bank key, with their SHA-256 and the creditor account in the file: the
// Rückgabe ships this original of a submitted file (#6314, ADR 0187).
func (d *SEPADebits) storeFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
	plan debitPlan, data []byte,
) error {
	sealed, err := d.cipher.Seal(data, debitFileContext(reviewer.OrganizationID, plan.run.MessageID))
	if err != nil {
		return fmt.Errorf("parent billing: seal the SEPA direct debit file: %w", err)
	}
	file := fileCreditor(runKind, runID, plan)
	file.Sealed, file.SHA256 = sealed, d.fingerprint(data)
	if err := d.store.SaveSEPADebitFile(ctx, reviewer.OrganizationID, reviewer.AccountID, file); err != nil {
		return fmt.Errorf("parent billing: keep the SEPA direct debit file: %w", err)
	}
	return nil
}

// fileCreditor names the message, the creditor account of its file and
// its members (ADR 0187).
func fileCreditor(runKind string, runID int64, plan debitPlan) parentbilling.StoredSEPADebitFile {
	file := parentbilling.StoredSEPADebitFile{MessageID: plan.run.MessageID, RunKind: runKind, RunID: runID, CreditorID: plan.run.CreditorID,
		DebitIDs: []int64{}, NoticeIDs: []int64{}, MandateIDs: []int64{}}
	if account := plan.account; account != nil {
		file.CreditorName, file.CreditorIBAN, file.CreditorBIC = account.AccountHolder, sepa.Compact(account.IBAN), sepa.Compact(account.BIC)
	}
	for _, entry := range plan.included {
		if debit := entry.Debit; isNoticeRate(debit) {
			file.NoticeIDs = append(file.NoticeIDs, debit.NoticeID)
		} else {
			file.DebitIDs = append(file.DebitIDs, debit.ID)
		}
		if !slices.Contains(file.MandateIDs, entry.Debit.MandateID) {
			file.MandateIDs = append(file.MandateIDs, entry.Debit.MandateID)
		}
	}
	return file
}

// debitFileContext binds a sealed debit file to the Träger and its message.
func debitFileContext(organizationID int64, messageID string) []byte {
	return []byte("parent_billing.sepa_debit_files.file|" + strconv.FormatInt(organizationID, 10) + "|" + messageID)
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
	return openDebitBatches(d.cipher, organizationID, plan.run.MessageID, plan.included, plan.collectOn)
}

// openDebitBatches opens the account of each debit of the file messageID
// and groups the debits into batches by sequence type and collection day.
// The download and the Rückgabe of a submitted file (#6314) share it, so
// the same debits give the same batches.
func openDebitBatches(cipher parentbilling.BankDataCipher, organizationID int64, messageID string,
	included []parentbilling.SealedSEPADebit, collectOns map[int64]calendar.Date,
) ([]debitFileBatch, []parentbilling.SEPAMandate, error) {
	var batches []debitFileBatch
	index := map[[2]string]int{}
	endToEnd := map[string]bool{}
	var revealed []parentbilling.SEPAMandate
	for _, entry := range included {
		debit := entry.Debit
		endToEndID := debitEndToEndID(debit)
		if endToEnd[endToEndID] {
			// A bank matches returns and refunds by this ID; a file with two
			// equal IDs would mix up families. Refuse it instead.
			return nil, nil, parentbilling.ErrSEPAEndToEndDuplicate
		}
		endToEnd[endToEndID] = true
		iban, holder, err := openDebitAccount(cipher, organizationID, entry)
		if err != nil {
			return nil, nil, err
		}
		collectOn := collectOns[debit.ID]
		key := [2]string{debit.SequenceType, collectOn.String()}
		at, ok := index[key]
		if !ok {
			at = len(batches)
			index[key] = at
			batches = append(batches, debitFileBatch{SequenceType: debit.SequenceType, CollectOn: collectOn,
				ID: sepaIdentifier(fmt.Sprintf("%s-%d", messageID[:min(len(messageID), 30)], at+1))})
		}
		batches[at].Entries = append(batches[at].Entries, debitFileEntry{
			EndToEndID: endToEndID, AmountCents: debit.AmountCents,
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

// openDebitAccount opens the IBAN and the account holder a debit copied.
func openDebitAccount(cipher parentbilling.BankDataCipher, organizationID int64, entry parentbilling.SealedSEPADebit) (iban, holder []byte, err error) {
	reference := entry.Debit.MandateReference
	if iban, err = cipher.Open(entry.IBAN, ibanContext(organizationID, reference)); err != nil {
		return nil, nil, fmt.Errorf("parent billing: open the IBAN of debit %d: %w", entry.Debit.ID, err)
	}
	if holder, err = cipher.Open(entry.Holder, holderContext(organizationID, reference)); err != nil {
		return nil, nil, fmt.Errorf("parent billing: open the account holder of debit %d: %w", entry.Debit.ID, err)
	}
	return iban, holder, nil
}

// debitEndToEndID is the ID the bank keeps for one debit: the invoice
// number when it fits the SEPA characters and 35 characters unchanged, else
// MOTO-D and the debit's own ID, which is unique. A cut-down number could
// give two invoices the same ID; the remittance text keeps the full number.
func debitEndToEndID(debit parentbilling.SEPADebit) string {
	// A stored installment (its confirmation wrote the row) keeps the ID of
	// the file it was collected with.
	if isNoticeRate(debit) || debit.NoticeID > 0 {
		return noticeEndToEndID(debit)
	}
	if sepaIdentifier(debit.InvoiceNumber) == debit.InvoiceNumber {
		return debit.InvoiceNumber
	}
	return "MOTO-D" + strconv.FormatInt(debit.ID, 10)
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
	day, err := debitFileDay(messageID)
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	plan, err := d.load(ctx, reviewer, runKind, runID, false, true, day)
	if err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if len(plan.included) == 0 {
		return parentbilling.SEPADebitRun{}, parentbilling.ErrSEPADebitsEmpty
	}
	if strings.TrimSpace(messageID) != plan.run.MessageID {
		return parentbilling.SEPADebitRun{}, parentbilling.ErrSEPADebitsChanged
	}
	submitted, collected, mandates := submittedDebits(plan)
	invoices, rates := splitNoticeRates(submitted)
	if err := d.store.InsertSubmittedNoticeRates(ctx, reviewer.OrganizationID, reviewer.AccountID, runID, plan.run.MessageID, rates); err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if err := d.store.SubmitSEPADebits(ctx, reviewer.OrganizationID, reviewer.AccountID, plan.run.MessageID, invoices, collected); err != nil {
		return parentbilling.SEPADebitRun{}, err
	}
	if err := d.store.SubmitSEPADebitFile(ctx, reviewer.OrganizationID, reviewer.AccountID, fileCreditor(runKind, runID, plan)); err != nil {
		return parentbilling.SEPADebitRun{}, fmt.Errorf("parent billing: keep the submitted SEPA direct debit file: %w", err)
	}
	if err := d.store.InsertSEPAMandateEvents(ctx, reviewer.OrganizationID, reviewer.AccountID, mandateCollect,
		parentbilling.MandateFromOffice, nil, mandates); err != nil {
		return parentbilling.SEPADebitRun{}, fmt.Errorf("parent billing: record the collection of the SEPA mandates: %w", err)
	}
	if err := d.store.MarkExportFilesSubmitted(ctx, reviewer.OrganizationID, reviewer.AccountID, runKind, runID, plan.run.MessageID); err != nil {
		return parentbilling.SEPADebitRun{}, fmt.Errorf("parent billing: record the submission in the export protocol: %w", err)
	}
	return d.SEPADebitRun(ctx, reviewer, runKind, runID)
}

// submittedDebits are the debits of a confirmed file with their collection
// day, the last collection day per mandate and the mandates they collect.
func submittedDebits(plan debitPlan) ([]parentbilling.SEPADebit, map[int64]calendar.Date, []parentbilling.SEPAMandate) {
	submitted := make([]parentbilling.SEPADebit, len(plan.included))
	collected := map[int64]calendar.Date{}
	var mandates []parentbilling.SEPAMandate
	for i, entry := range plan.included {
		submitted[i] = entry.Debit
		submitted[i].CollectOn = plan.collectOn[entry.Debit.ID]
		on := plan.collectOn[entry.Debit.ID]
		if last, ok := collected[entry.Debit.MandateID]; !ok || on.After(last) {
			if !ok {
				mandates = append(mandates, parentbilling.SEPAMandate{ID: entry.Debit.MandateID, ContractID: entry.Debit.ContractID})
			}
			collected[entry.Debit.MandateID] = on
		}
	}
	return submitted, collected, mandates
}

// splitNoticeRates parts the debits of a confirmed file into invoices,
// whose rows exist, and installments, which the confirmation writes.
func splitNoticeRates(debits []parentbilling.SEPADebit) ([]parentbilling.SEPADebit, []parentbilling.SEPADebit) {
	var invoices, rates []parentbilling.SEPADebit
	for _, debit := range debits {
		if isNoticeRate(debit) {
			rates = append(rates, debit)
		} else {
			invoices = append(invoices, debit)
		}
	}
	return invoices, rates
}
