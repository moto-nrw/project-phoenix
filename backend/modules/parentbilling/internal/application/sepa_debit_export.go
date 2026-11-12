package application

import (
	"context"
	"strconv"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// recordDebitFile records a download of the PAIN.008 file of a run in the
// export protocol (#4693, decision on #4697 of 08.11.): the next file number
// of the Träger, which the file name carries, the message ID and the
// invoices and installments the file collects. The file itself stays the
// pure function of the debits (ADR 0181): a repeated download has the same
// message ID and a new file number.
func (d *SEPADebits) recordDebitFile(ctx context.Context, reviewer parentbilling.HolidayReviewer, runKind string, runID int64,
	plan debitPlan, data []byte,
) (parentbilling.ExportProtocolEntry, error) {
	file := exportFile{format: parentbilling.ExportFormatPain008, runKind: runKind, runID: runID, data: data,
		totalCents: plan.run.TotalCents, reference: plan.run.MessageID,
		fileName: func(number int64) string { return debitFileName(number, plan.run.MessageID) }}
	for _, entry := range plan.included {
		if isNoticeRate(entry.Debit) {
			file.notices = append(file.notices, entry.Debit.NoticeID)
			continue
		}
		file.debits = append(file.debits, entry.Debit.ID)
	}
	entry, _, err := recordExportFile(ctx, d.store, reviewer, file)
	return entry, err
}

// debitFileName names the file with its file number and its message ID.
func debitFileName(number int64, messageID string) string {
	return "lastschrift-" + strconv.FormatInt(number, 10) + "-" + messageID + ".xml"
}
