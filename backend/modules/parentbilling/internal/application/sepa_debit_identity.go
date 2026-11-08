package application

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/sharedkernel/sepa"
)

// messageIDOf binds payment terms and creditor details to the earliest
// collection day. Keeping it in the ID lets a later confirmation rebuild
// the exported dates without storing a file or recording a download.
func messageIDOf(runKind string, runID int64, plan debitPlan) string {
	if len(plan.included) == 0 || plan.account == nil {
		return ""
	}
	hash := fnv.New128a()
	_, _ = fmt.Fprintf(hash, "%q|%d|%q|%q|%q|%q;", runKind, runID, plan.run.CreditorID,
		sepaText(plan.account.AccountHolder, maxSEPAName), sepa.Compact(plan.account.IBAN), sepa.Compact(plan.account.BIC))
	for _, entry := range plan.included {
		_, _ = fmt.Fprintf(hash, "%d|%d|%q|%s;", entry.Debit.ID, entry.Debit.AmountCents,
			entry.Debit.SequenceType, plan.collectOn[entry.Debit.ID])
	}
	earliest := plan.collectOn[plan.included[0].Debit.ID]
	for _, on := range plan.collectOn {
		earliest = min(earliest, on)
	}
	return fmt.Sprintf("MOTO-%s-%x", earliest.Format("20060102"), hash.Sum(nil)[:10])
}

func debitFileDay(messageID string) (calendar.Date, error) {
	parts := strings.Split(messageID, "-")
	if len(parts) != 3 || parts[0] != "MOTO" || len(parts[1]) != 8 || len(parts[2]) != 20 {
		return "", parentbilling.ErrSEPADebitsChanged
	}
	day, err := calendar.ParseDate(parts[1][:4] + "-" + parts[1][4:6] + "-" + parts[1][6:])
	if err != nil {
		return "", parentbilling.ErrSEPADebitsChanged
	}
	return day.AddDays(-1), nil
}

// Ambiguous one-off authorizations stay out entirely; never choose an
// invoice on the family's behalf. Cancelled and otherwise excluded entries
// do not consume an authorization.
func duplicateOneOffMandates(debits []parentbilling.SealedSEPADebit, earliest calendar.Date) map[int64]bool {
	type authorization struct {
		kind             string
		runID, mandateID int64
	}
	seen, duplicate := map[authorization]int64{}, map[int64]bool{}
	for _, entry := range debits {
		if entry.Debit.Status != parentbilling.DebitPending || (entry.MandateKind != parentbilling.MandateOneOff && entry.Debit.SequenceType != parentbilling.SequenceTypeOneOff) ||
			leftReason(entry, entry.Debit.AmountCents+entry.CreditedCents, max(entry.Debit.DueOn, earliest)) != "" {
			continue
		}
		key := authorization{entry.Debit.RunKind, entry.Debit.RunID, entry.Debit.MandateID}
		if first, ok := seen[key]; ok {
			duplicate[first], duplicate[entry.Debit.ID] = true, true
		}
		seen[key] = entry.Debit.ID
	}
	return duplicate
}

func pendingDebitLeft(entry parentbilling.SealedSEPADebit, amount int64, collectOn calendar.Date, duplicates map[int64]bool) string {
	if reason := leftReason(entry, amount, collectOn); reason != "" {
		return reason
	}
	if duplicates[entry.Debit.ID] {
		return parentbilling.DebitLeftOneOffMultiple
	}
	return ""
}
