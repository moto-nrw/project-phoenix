package application

import (
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/sepa"
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
		if isNoticeRate(entry.Debit) {
			// An installment takes the account of the mandate now: a change
			// of it is a different file.
			_, _ = fmt.Fprintf(hash, "%d|%d;", entry.Debit.MandateID, entry.MandateUpdatedAt.UnixNano())
		}
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

// pendingDebitLeft tells why a pending debit stays out of the file. A
// one-off mandate another uncancelled debit of any run also claims leaves
// all of them out: moto never chooses an invoice on the family's behalf.
func pendingDebitLeft(entry parentbilling.SealedSEPADebit, amount int64, collectOn calendar.Date) string {
	if reason := leftReason(entry, amount, collectOn); reason != "" {
		return reason
	}
	if entry.OneOffShared {
		return parentbilling.DebitLeftOneOffMultiple
	}
	return ""
}
