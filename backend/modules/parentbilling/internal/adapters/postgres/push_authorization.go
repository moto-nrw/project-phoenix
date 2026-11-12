package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// A monthly letter (ADR 0165) pushes like a holiday letter, and only for the
// child of its invoice at the school of the push, which the account's
// relationship grants the inbox; a cancelled invoice no longer pushes, its
// Storno letter does. A replaced invoice is cancelled, so its new invoice
// pushes in its place.
//
// A historical invoice stays downloadable after cancellation, but its old
// announcement is no longer current. Cancellation/credit notices themselves
// remain valid. A Jahresbescheid (#6347) pushes like a monthly letter, for
// the child of its notice at the school of the push, once it was sent; a
// change notice is a document of its own. Read all candidates of one Träger in one statement.
const validDocumentPushesSQL = `SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint, document_id bigint, email text)
 JOIN parent_billing.documents AS document ON document.id = candidate.document_id AND document.organization_id = ?
 JOIN parent_billing.runs AS run ON run.id = document.run_id AND run.organization_id = document.organization_id
 WHERE LOWER(document.recipient_email) = LOWER(candidate.email) AND run.status = 'released'
 AND EXISTS (SELECT 1 FROM parent_billing.document_letters AS letter
  WHERE letter.document_id = document.id AND letter.organization_id = document.organization_id)
 AND EXISTS (SELECT 1 FROM parent_billing.document_mailings AS mailing
  WHERE mailing.document_id = document.id AND mailing.organization_id = document.organization_id)
 AND ` + notCancelledSQL + `
 UNION
 SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint, document_id bigint, email text, school_id bigint, student_ids jsonb)
 JOIN parent_billing.care_letters AS letter ON letter.id = candidate.document_id AND letter.organization_id = ?
 JOIN parent_billing.care_invoices AS invoice ON invoice.id = letter.invoice_id AND invoice.organization_id = letter.organization_id
 JOIN parent_billing.care_runs AS run ON run.id = invoice.run_id AND run.organization_id = invoice.organization_id
 WHERE LOWER(invoice.recipient_email) = LOWER(candidate.email) AND run.status = 'released'
 AND invoice.school_id = candidate.school_id
 AND invoice.student_id IN (SELECT value::bigint FROM jsonb_array_elements_text(candidate.student_ids))
 AND EXISTS (SELECT 1 FROM parent_billing.care_letter_mailings AS mailing
  WHERE mailing.letter_id = letter.id AND mailing.organization_id = letter.organization_id)
 AND (letter.kind = 'cancellation' OR NOT EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
  WHERE cancellation.organization_id = invoice.organization_id AND cancellation.invoice_id = invoice.id))
 UNION
 SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint, document_id bigint, email text, school_id bigint, student_ids jsonb)
 JOIN parent_billing.care_notice_letters AS letter ON letter.id = candidate.document_id AND letter.organization_id = ?
 JOIN parent_billing.care_notices AS notice ON notice.id = letter.notice_id AND notice.organization_id = letter.organization_id
 WHERE LOWER(notice.recipient_email) = LOWER(candidate.email)
 AND notice.school_id = candidate.school_id
 AND notice.student_id IN (SELECT value::bigint FROM jsonb_array_elements_text(candidate.student_ids))
 AND EXISTS (SELECT 1 FROM parent_billing.care_notice_letter_mailings AS mailing
  WHERE mailing.letter_id = letter.id AND mailing.organization_id = letter.organization_id)`

func (s *Store) ValidDocumentPushes(ctx context.Context, organizationID int64, candidates []parentbilling.DocumentPushCandidate) (map[int64]bool, error) {
	if len(candidates) == 0 {
		return map[int64]bool{}, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return nil, err
	}
	var keys []int64
	if err := db.NewRaw(validDocumentPushesSQL, string(encoded), organizationID, string(encoded), organizationID,
		string(encoded), organizationID).Scan(ctx, &keys); err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(keys))
	for _, key := range keys {
		result[key] = true
	}
	return result, nil
}
