package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The inbox of the parents portal (#4671, ADR 0126): the invoices of a
// Träger addressed to one address whose letter was sent at least once. The
// address compares without case, like the login of an account.

// recipientDocumentsSQL lists the sent documents of one recipient, the
// newest send first. The first send is when the document reached the
// inbox; the children are the first names on its positions.
const recipientDocumentsSQL = `SELECT document.id, document.run_id, FALSE AS care, FALSE AS notice, document.kind, document.number, run.period_name,
		document.service_from, document.service_until, document.issued_on, document.total_cents, sends.first_sent_at,
		COALESCE((SELECT json_agg(DISTINCT position.child_first_name ORDER BY position.child_first_name)
			FROM parent_billing.document_positions AS position
			WHERE position.organization_id = document.organization_id AND position.document_id = document.id), '[]') AS child_names
	FROM parent_billing.documents AS document
	JOIN parent_billing.runs AS run ON run.organization_id = document.organization_id AND run.id = document.run_id
	JOIN (SELECT mailing.document_id, MIN(mailing.created_at) AS first_sent_at
		FROM parent_billing.document_mailings AS mailing
		WHERE mailing.organization_id = ?
		GROUP BY mailing.document_id) AS sends ON sends.document_id = document.id
	WHERE document.organization_id = ? AND LOWER(document.recipient_email) = LOWER(?)
	UNION ALL
	SELECT letter.id, letter.run_id, TRUE AS care, FALSE AS notice, letter.kind, COALESCE(cancellation.number, invoice.number), '',
		invoice.billing_month, (invoice.billing_month + INTERVAL '1 month' - INTERVAL '1 day')::date,
		COALESCE(cancellation.issued_on, invoice.issued_on), COALESCE(cancellation.total_cents, invoice.total_cents),
		sends.first_sent_at, json_build_array(invoice.child_first_name)
	FROM parent_billing.care_letters AS letter
	JOIN parent_billing.care_invoices AS invoice ON invoice.organization_id = letter.organization_id AND invoice.id = letter.invoice_id
	LEFT JOIN parent_billing.care_invoice_cancellations AS cancellation
		ON letter.kind = 'cancellation' AND cancellation.organization_id = letter.organization_id AND cancellation.invoice_id = letter.invoice_id
	JOIN (SELECT mailing.letter_id, MIN(mailing.created_at) AS first_sent_at
		FROM parent_billing.care_letter_mailings AS mailing
		WHERE mailing.organization_id = ?
		GROUP BY mailing.letter_id) AS sends ON sends.letter_id = letter.id
	WHERE letter.organization_id = ? AND LOWER(invoice.recipient_email) = LOWER(?)
		AND EXISTS (SELECT 1 FROM jsonb_to_recordset(?::jsonb) AS permitted(school_id bigint, student_id bigint)
			WHERE permitted.school_id = invoice.school_id AND permitted.student_id = invoice.student_id)
	UNION ALL
	` + recipientNoticesSQL + `
	ORDER BY first_sent_at DESC, id DESC`

// recipientLetterSQL reads the stored letter of one sent document of one
// recipient.
const recipientLetterSQL = `SELECT document.run_id, FALSE AS care, letter.filename, letter.pdf
	FROM parent_billing.documents AS document
	JOIN parent_billing.document_letters AS letter
		ON letter.organization_id = document.organization_id AND letter.document_id = document.id
	WHERE document.organization_id = ? AND document.id = ? AND LOWER(document.recipient_email) = LOWER(?)
		AND EXISTS (SELECT 1 FROM parent_billing.document_mailings AS mailing
			WHERE mailing.organization_id = document.organization_id AND mailing.document_id = document.id)
	UNION ALL
	SELECT letter.run_id, TRUE AS care, letter.filename, letter.pdf
	FROM parent_billing.care_letters AS letter
	JOIN parent_billing.care_invoices AS invoice ON invoice.organization_id = letter.organization_id AND invoice.id = letter.invoice_id
	WHERE letter.organization_id = ? AND letter.id = ? AND LOWER(invoice.recipient_email) = LOWER(?)
		AND EXISTS (SELECT 1 FROM parent_billing.care_letter_mailings AS mailing
			WHERE mailing.organization_id = letter.organization_id AND mailing.letter_id = letter.id)
		AND EXISTS (SELECT 1 FROM jsonb_to_recordset(?::jsonb) AS permitted(school_id bigint, student_id bigint)
			WHERE permitted.school_id = invoice.school_id AND permitted.student_id = invoice.student_id)`

type recipientDocumentRow struct {
	ID           int64         `bun:"id"`
	RunID        int64         `bun:"run_id"`
	Care         bool          `bun:"care"`
	Notice       bool          `bun:"notice"`
	Kind         string        `bun:"kind"`
	Number       string        `bun:"number"`
	PeriodName   string        `bun:"period_name"`
	ServiceFrom  calendar.Date `bun:"service_from,type:date"`
	ServiceUntil calendar.Date `bun:"service_until,type:date"`
	IssuedOn     calendar.Date `bun:"issued_on,type:date"`
	TotalCents   int64         `bun:"total_cents"`
	FirstSentAt  time.Time     `bun:"first_sent_at"`
	ChildNames   []string      `bun:"child_names,type:json"`
}

// RecipientDocuments lists the sent documents of the Träger addressed to
// email, the newest first.
func (s *Store) RecipientDocuments(ctx context.Context, organizationID int64, email string, children []parentbilling.ParentDocumentChild) ([]parentbilling.ParentDocument, error) {
	permitted, err := permittedChildren(children)
	if err != nil {
		return nil, err
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []recipientDocumentRow
	if err := db.NewRaw(recipientDocumentsSQL, organizationID, organizationID, email, organizationID, organizationID, email, permitted,
		organizationID, organizationID, email, permitted).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	documents := make([]parentbilling.ParentDocument, len(rows))
	for i, row := range rows {
		names := row.ChildNames
		if names == nil {
			names = []string{}
		}
		if row.Care {
			row.PeriodName = parentbilling.CarePeriodName(row.ServiceFrom)
		}
		kind := parentDocumentKind(row.Care, row.Kind)
		if row.Notice {
			kind = noticeDocumentKind(row.Kind)
		}
		documents[i] = parentbilling.ParentDocument{
			DocumentID: row.ID, RunID: row.RunID, CareRun: row.Care, NoticeRun: row.Notice, Kind: kind, Number: row.Number,
			PeriodName: row.PeriodName, ServiceFrom: row.ServiceFrom, ServiceUntil: row.ServiceUntil, IssuedOn: row.IssuedOn,
			TotalCents: row.TotalCents, ChildNames: names, SentAt: row.FirstSentAt,
		}
	}
	return documents, nil
}

// noticeDocumentKind names a Jahresbescheid in the inbox (#6347).
func noticeDocumentKind(kind string) string {
	if kind == parentbilling.NoticeKindChange {
		return parentbilling.ParentDocumentKindCareChangeNotice
	}
	return parentbilling.ParentDocumentKindCareFeeNotice
}

// parentDocumentKind names the letter of a document in the inbox.
func parentDocumentKind(care bool, kind string) string {
	switch {
	case care && kind == parentbilling.CareLetterCancellation:
		return parentbilling.ParentDocumentKindCareCancellationLetter
	case care:
		return parentbilling.ParentDocumentKindCareInvoiceLetter
	}
	switch kind {
	case parentbilling.DocumentCancellation:
		return parentbilling.ParentDocumentKindCancellationLetter
	case parentbilling.DocumentCredit:
		return parentbilling.ParentDocumentKindCreditLetter
	}
	return parentbilling.ParentDocumentKindConfirmationLetter
}

// RecipientDocumentLetter reads the stored letter of one sent document of
// the Träger addressed to email, with its run and whether that is a monthly
// run. A holiday document and a monthly letter never share an ID: both take
// it from parent_billing.documents_id_seq (1.15.809).
func (s *Store) RecipientDocumentLetter(ctx context.Context, organizationID, documentID int64, email string, children []parentbilling.ParentDocumentChild) (int64, bool, parentbilling.BillingLetterFile, error) {
	permitted, err := permittedChildren(children)
	if err != nil {
		return 0, false, parentbilling.BillingLetterFile{}, err
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, false, parentbilling.BillingLetterFile{}, err
	}
	var row struct {
		RunID    int64  `bun:"run_id"`
		Care     bool   `bun:"care"`
		Filename string `bun:"filename"`
		PDF      []byte `bun:"pdf"`
	}
	if err := db.NewRaw(recipientLetterSQL, organizationID, documentID, email, organizationID, documentID, email, permitted).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, parentbilling.BillingLetterFile{}, parentbilling.ErrParentDocumentNotFound
		}
		return 0, false, parentbilling.BillingLetterFile{}, err
	}
	return row.RunID, row.Care, parentbilling.BillingLetterFile{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}, nil
}

// permittedChildren encodes the children the account may read the inbox of,
// never null, so no child matches an empty list.
func permittedChildren(children []parentbilling.ParentDocumentChild) (string, error) {
	if children == nil {
		children = []parentbilling.ParentDocumentChild{}
	}
	encoded, err := json.Marshal(children)
	return string(encoded), err
}

// The Jahresbescheide in the inbox (#6347): a stored original that was sent
// at least once, addressed to the reader's address and to a child the
// reader's relationship grants the inbox at the school of the notice. A
// change notice is a document of its own next to the notice it amends.

// recipientNoticesSQL is the branch of recipientDocumentsSQL that lists the
// sent notices of one recipient for the permitted children (#6347).
const recipientNoticesSQL = `SELECT letter.id, letter.run_id, FALSE AS care, TRUE AS notice, notice.kind, notice.number,
		'Betreuung ' || notice.billing_year::text, notice.period_from, notice.period_until, notice.issued_on, notice.total_cents,
		sends.first_sent_at, json_build_array(notice.child_first_name)
	FROM parent_billing.care_notice_letters AS letter
	JOIN parent_billing.care_notices AS notice ON notice.organization_id = letter.organization_id AND notice.id = letter.notice_id
	JOIN (SELECT mailing.letter_id, MIN(mailing.created_at) AS first_sent_at
		FROM parent_billing.care_notice_letter_mailings AS mailing
		WHERE mailing.organization_id = ?
		GROUP BY mailing.letter_id) AS sends ON sends.letter_id = letter.id
	WHERE letter.organization_id = ? AND LOWER(notice.recipient_email) = LOWER(?)
		AND EXISTS (SELECT 1 FROM jsonb_to_recordset(?::jsonb) AS permitted(school_id bigint, student_id bigint)
			WHERE permitted.school_id = notice.school_id AND permitted.student_id = notice.student_id)`

// recipientNoticeLetterSQL reads the stored original of one sent notice of
// one recipient.
const recipientNoticeLetterSQL = `SELECT letter.run_id, letter.filename, letter.pdf
	FROM parent_billing.care_notice_letters AS letter
	JOIN parent_billing.care_notices AS notice ON notice.organization_id = letter.organization_id AND notice.id = letter.notice_id
	WHERE letter.organization_id = ? AND letter.id = ? AND LOWER(notice.recipient_email) = LOWER(?)
		AND EXISTS (SELECT 1 FROM parent_billing.care_notice_letter_mailings AS mailing
			WHERE mailing.organization_id = letter.organization_id AND mailing.letter_id = letter.id)
		AND EXISTS (SELECT 1 FROM jsonb_to_recordset(?::jsonb) AS permitted(school_id bigint, student_id bigint)
			WHERE permitted.school_id = notice.school_id AND permitted.student_id = notice.student_id)`

// RecipientCareNoticeLetter reads the stored original of one sent notice of
// the Träger addressed to email for a permitted child, with its run. Its ID
// comes from parent_billing.documents_id_seq like every letter of the inbox,
// so it never names a holiday or monthly letter.
func (s *Store) RecipientCareNoticeLetter(ctx context.Context, organizationID, documentID int64, email string, children []parentbilling.ParentDocumentChild) (int64, parentbilling.BillingLetterFile, error) {
	permitted, err := permittedChildren(children)
	if err != nil {
		return 0, parentbilling.BillingLetterFile{}, err
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return 0, parentbilling.BillingLetterFile{}, err
	}
	var row struct {
		RunID    int64  `bun:"run_id"`
		Filename string `bun:"filename"`
		PDF      []byte `bun:"pdf"`
	}
	if err := db.NewRaw(recipientNoticeLetterSQL, organizationID, documentID, email, permitted).Scan(ctx, &row); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, parentbilling.BillingLetterFile{}, parentbilling.ErrParentDocumentNotFound
		}
		return 0, parentbilling.BillingLetterFile{}, err
	}
	return row.RunID, parentbilling.BillingLetterFile{Filename: row.Filename, ContentType: "application/pdf", Data: row.PDF}, nil
}
