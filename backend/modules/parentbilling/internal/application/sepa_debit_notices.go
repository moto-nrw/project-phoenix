package application

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The installments of the Jahresbescheide in the SEPA file (#4697, ADR
// 0181, ADR 0182). A month assessed by a notice has no monthly invoice, and
// a monthly run without invoices is never released; the installments due in
// a month therefore have a file of their own (kind notice, ID YYYYMM), so
// every installment has exactly one file and no family is in the file of a
// monthly run and in that of its notice for the same month. The installments come from the read and the resolution behind
// CareFeeNotices.EffectiveNoticeDues (effectiveDues): the file never resolves the change
// notices itself. An installment is planned from the family's mandate in
// force; the confirmation writes it as a submitted debit with that
// mandate's account, one per contract and month.

// noticeRateStore reads what the file needs of the installments.
type noticeRateStore interface {
	noticeDueReader
	// SEPANoticeRates reads the recipient and the family's mandate in force
	// of each notice, with the sealed account when sealed is set.
	SEPANoticeRates(ctx context.Context, organizationID int64, noticeIDs []int64, sealed bool) (map[int64]parentbilling.SEPANoticeRate, error)
	// InsertSubmittedNoticeRates writes the installments of a confirmed file.
	InsertSubmittedNoticeRates(ctx context.Context, organizationID, accountID, runID int64, messageID string, rates []parentbilling.SEPADebit) error
}

// isNoticeRate tells an installment planned from a notice: it has no row
// yet and carries the negative notice ID as its key in the plan.
func isNoticeRate(debit parentbilling.SEPADebit) bool { return debit.ID < 0 }

// noticeMonthOf is the month a file of installments names as YYYYMM.
func noticeMonthOf(runID int64) (calendar.Date, bool) {
	year, month := int(runID/100), int(runID%100)
	if year < 2000 || year > 9999 || month < 1 || month > 12 {
		return "", false
	}
	return calendar.NewDate(year, time.Month(month), 1), true
}

// noticeRates plans the installments due in the month of a file of
// installments, leaving out those its confirmation collected already.
func noticeRates(ctx context.Context, store noticeRateStore, organizationID int64, runKind string, runID int64,
	debits []parentbilling.SealedSEPADebit, sealed bool,
) ([]parentbilling.SealedSEPADebit, error) {
	if runKind != parentbilling.DebitRunNotice {
		return nil, nil
	}
	month, ok := noticeMonthOf(runID)
	if !ok {
		return nil, parentbilling.ErrSEPADebitRunNotFound
	}
	// The read and the resolution of CareFeeNotices.EffectiveNoticeDues; the
	// month is a month start and the Träger is the transaction's.
	read, err := store.EffectiveNoticeDues(ctx, organizationID, month)
	if err != nil {
		return nil, fmt.Errorf("parent billing: read the installments of the month: %w", err)
	}
	open, notices := openNoticeDues(effectiveDues(read), debits)
	contexts, err := store.SEPANoticeRates(ctx, organizationID, notices, sealed)
	if err != nil {
		return nil, err
	}
	rates := make([]parentbilling.SealedSEPADebit, 0, len(open))
	for _, due := range open {
		rates = append(rates, noticeRate(due, contexts[due.NoticeID]))
	}
	markSharedOneOffs(rates)
	return rates, nil
}

// openNoticeDues keeps the installments by direct debit over 0 € that no
// confirmation of the month collected yet, with their notices.
func openNoticeDues(dues []parentbilling.CareNoticeDue, debits []parentbilling.SealedSEPADebit,
) ([]parentbilling.CareNoticeDue, []int64) {
	collected := map[int64]bool{}
	for _, entry := range debits {
		if entry.Debit.NoticeID != 0 {
			collected[entry.Debit.ContractID] = true
		}
	}
	var open []parentbilling.CareNoticeDue
	var notices []int64
	for _, due := range dues {
		if due.PaymentMethod == parentbilling.PaymentDirectDebit && due.AmountCents > 0 && !collected[due.ContractID] {
			open = append(open, due)
			notices = append(notices, due.NoticeID)
		}
	}
	return open, notices
}

// markSharedOneOffs marks siblings of one family that share a one-off
// mandate: it covers one of their installments only, so none of them goes
// into the file.
func markSharedOneOffs(rates []parentbilling.SealedSEPADebit) {
	oneOff := map[int64]int{}
	for _, rate := range rates {
		if rate.MandateKind == parentbilling.MandateOneOff {
			oneOff[rate.Debit.MandateID]++
		}
	}
	for i := range rates {
		if rates[i].MandateKind == parentbilling.MandateOneOff && oneOff[rates[i].Debit.MandateID] > 1 {
			rates[i].OneOffShared = true
		}
	}
}

// noticeRate is one installment as a pending debit of the plan.
func noticeRate(due parentbilling.CareNoticeDue, rate parentbilling.SEPANoticeRate) parentbilling.SealedSEPADebit {
	entry := parentbilling.SealedSEPADebit{
		Debit: parentbilling.SEPADebit{ID: -due.NoticeID, ContractID: due.ContractID, RunKind: parentbilling.DebitRunNotice,
			InvoiceNumber: due.NoticeNumber, RecipientName: rate.RecipientName, AmountCents: due.AmountCents, DueOn: due.DueOn,
			Status: parentbilling.DebitPending, NoticeID: due.NoticeID, NoticeMonth: due.Month},
		CreditorNow: rate.CreditorNow, OneOffShared: rate.OneOffClaimed, Holder: rate.Holder, IBAN: rate.IBAN,
	}
	mandate := rate.Mandate
	if mandate == nil {
		return entry
	}
	entry.Debit.MandateID, entry.Debit.MandateReference, entry.Debit.IBANMasked = mandate.ID, mandate.Reference, mandate.IBANMasked
	entry.Debit.SequenceType = mandate.SequenceType()
	entry.CreditorID, entry.MandateSignedOn, entry.BIC = mandate.CreditorID, mandate.SignedOn, mandate.BIC
	entry.MandateKind, entry.MandateSigned, entry.MandateSequence = mandate.Kind, mandate.SignedOn, mandate.Sequence
	entry.MandateLastCollectedOn, entry.MandateCreatedOn = mandate.LastCollectedOn, calendar.DateFromTime(mandate.CreatedAt)
	entry.MandateUpdatedAt = mandate.UpdatedAt
	return entry
}

// noticeEndToEndID is the end-to-end ID of an installment: its notice and
// month, unique per installment and at most 35 characters.
func noticeEndToEndID(debit parentbilling.SEPADebit) string {
	return "MOTO-N" + strconv.FormatInt(debit.NoticeID, 10) + "-" + debit.NoticeMonth.Format("200601")
}
