package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The documents of the families of a Träger (#4691). A family is the mail
// address of the registration the release copied (#4689), so a payer who
// gets the invoice does not split the family; a cancellation or credit
// belongs to the family of its invoice, a document issued before #4689 to
// its recipient address, compared without case. The COALESCE in
// familyDocumentsSQL is the one place that says so. The kinds of the
// positions are parentbilling.LineCostTakeover and LineBenefitsExemption.

// familyDocumentsSQL reads the issued documents of the Träger, the oldest
// first: the holiday invoices with their cancellations and credits and,
// since #4689, the monthly invoices with their Storno (service "care"). It
// reads the period of a holiday run, the first names of the children, the
// cost bearers the positions name and whether the exemption freed them. The
// last filter keeps the documents of one family: the family of the invoice
// the anchor names (a holiday document by its ID, a monthly invoice by its
// negated ID), or every family when the anchor is zero.
const familyDocumentsSQL = `WITH family_rows AS (
	SELECT 'holiday' AS service, document.id, document.run_id, document.kind, document.number, document.issued_on,
		document.issued_at, run.period_name, document.service_from, document.service_until, document.reference_document_id,
		reference.number AS reference_number, document.reason, document.recipient_first_name, document.recipient_last_name,
		document.recipient_email, document.recipient_kind, document.recipient_second_name, document.debtor_number,
		document.total_cents,
		COALESCE(document.family_email, reference.family_email, LOWER(document.recipient_email)) AS family_key,
		document.payment_method, document.due_on,
		COALESCE((SELECT json_agg(DISTINCT position.child_first_name ORDER BY position.child_first_name)
			FROM parent_billing.document_positions AS position
			WHERE position.organization_id = document.organization_id AND position.document_id = document.id), '[]') AS child_names,
		COALESCE((SELECT json_agg(DISTINCT position.cost_bearer_name ORDER BY position.cost_bearer_name)
			FROM parent_billing.document_positions AS position
			WHERE position.organization_id = document.organization_id AND position.document_id = document.id
				AND position.kind = 'cost_takeover'), '[]') AS cost_bearer_names,
		EXISTS (SELECT 1 FROM parent_billing.document_positions AS position
			WHERE position.organization_id = document.organization_id AND position.document_id = document.id
				AND position.kind = 'benefits_exemption') AS exempt
	FROM parent_billing.documents AS document
	JOIN parent_billing.runs AS run ON run.organization_id = document.organization_id AND run.id = document.run_id
	LEFT JOIN parent_billing.documents AS reference
		ON reference.organization_id = document.organization_id AND reference.id = document.reference_document_id
	WHERE document.organization_id = ?0
	UNION ALL
	SELECT 'care', invoice.id, invoice.run_id, 'invoice', invoice.number, invoice.issued_on, invoice.issued_at, '',
		invoice.billing_month, (invoice.billing_month + INTERVAL '1 month' - INTERVAL '1 day')::date, NULL, NULL, '',
		invoice.recipient_first_name, invoice.recipient_last_name, invoice.recipient_email, invoice.recipient_kind,
		invoice.recipient_second_name, invoice.debtor_number, invoice.total_cents, COALESCE(invoice.family_email, LOWER(invoice.recipient_email)),
		invoice.payment_method, invoice.due_on, json_build_array(invoice.child_first_name), '[]'::json, false
	FROM parent_billing.care_invoices AS invoice
	WHERE invoice.organization_id = ?0
	UNION ALL
	SELECT 'care', cancellation.id, cancellation.run_id, 'cancellation', cancellation.number, cancellation.issued_on,
		cancellation.issued_at, '', invoice.billing_month, (invoice.billing_month + INTERVAL '1 month' - INTERVAL '1 day')::date,
		invoice.id, invoice.number, cancellation.reason, invoice.recipient_first_name, invoice.recipient_last_name,
		invoice.recipient_email, invoice.recipient_kind, invoice.recipient_second_name, invoice.debtor_number, cancellation.total_cents,
		COALESCE(invoice.family_email, LOWER(invoice.recipient_email)), NULL, NULL, json_build_array(invoice.child_first_name),
		'[]'::json, false
	FROM parent_billing.care_invoice_cancellations AS cancellation
	JOIN parent_billing.care_invoices AS invoice
		ON invoice.organization_id = cancellation.organization_id AND invoice.id = cancellation.invoice_id
	WHERE cancellation.organization_id = ?0
)
SELECT * FROM family_rows
WHERE ?1::bigint = 0 OR family_key = (
	SELECT anchor.family_key FROM family_rows AS anchor
	WHERE anchor.kind = 'invoice' AND ((anchor.service = 'holiday' AND anchor.id = ?1) OR (anchor.service = 'care' AND anchor.id = -?1)))
ORDER BY issued_at, service DESC, id`

type familyDocumentRow struct {
	Service             string         `bun:"service" json:"service"`
	IssuedAt            time.Time      `bun:"issued_at" json:"issued_at"`
	RecipientKind       *string        `bun:"recipient_kind" json:"recipient_kind"`
	RecipientSecondName *string        `bun:"recipient_second_name" json:"recipient_second_name"`
	ID                  int64          `bun:"id" json:"id"`
	RunID               int64          `bun:"run_id" json:"run_id"`
	Kind                string         `bun:"kind" json:"kind"`
	Number              string         `bun:"number" json:"number"`
	IssuedOn            calendar.Date  `bun:"issued_on,type:date" json:"issued_on"`
	PeriodName          string         `bun:"period_name" json:"period_name"`
	ServiceFrom         calendar.Date  `bun:"service_from,type:date" json:"service_from"`
	ServiceUntil        calendar.Date  `bun:"service_until,type:date" json:"service_until"`
	ReferenceDocumentID *int64         `bun:"reference_document_id" json:"reference_document_id"`
	ReferenceNumber     *string        `bun:"reference_number" json:"reference_number"`
	Reason              string         `bun:"reason" json:"reason"`
	RecipientFirstName  string         `bun:"recipient_first_name" json:"recipient_first_name"`
	RecipientLastName   string         `bun:"recipient_last_name" json:"recipient_last_name"`
	RecipientEmail      string         `bun:"recipient_email" json:"recipient_email"`
	DebtorNumber        *string        `bun:"debtor_number" json:"debtor_number"`
	TotalCents          int64          `bun:"total_cents" json:"total_cents"`
	FamilyKey           string         `bun:"family_key" json:"family_key"`
	PaymentMethod       *string        `bun:"payment_method" json:"payment_method"`
	DueOn               *calendar.Date `bun:"due_on,type:date" json:"due_on"`
	ChildNames          []string       `bun:"child_names,type:json" json:"child_names"`
	CostBearerNames     []string       `bun:"cost_bearer_names,type:json" json:"cost_bearer_names"`
	Exempt              bool           `bun:"exempt" json:"exempt"`
}

// FamilyDocuments reads the issued documents of the family of the document
// familyID, or of every family when familyID is zero, the oldest first.
func (s *Store) FamilyDocuments(ctx context.Context, organizationID, familyID int64) ([]parentbilling.BillingFamilyDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []familyDocumentRow
	if err := db.NewRaw(familyDocumentsSQL, organizationID, familyID).Scan(ctx, &rows); err != nil {
		return nil, err
	}
	documents := make([]parentbilling.BillingFamilyDocument, len(rows))
	for i, row := range rows {
		documents[i] = row.document()
	}
	return documents, nil
}

func (row familyDocumentRow) document() parentbilling.BillingFamilyDocument {
	document := parentbilling.BillingFamilyDocument{
		ID: row.ID, RunID: row.RunID, Kind: row.Kind, Number: row.Number, IssuedOn: row.IssuedOn, PeriodName: row.PeriodName,
		ServiceFrom: row.ServiceFrom, ServiceUntil: row.ServiceUntil, ReferenceDocumentID: row.ReferenceDocumentID,
		Reason: row.Reason, TotalCents: row.TotalCents, ChildNames: nonNil(row.ChildNames),
		RecipientFirstName: row.RecipientFirstName, RecipientLastName: row.RecipientLastName, RecipientEmail: row.RecipientEmail,
		FamilyKey: row.FamilyKey, CostBearerNames: nonNil(row.CostBearerNames), Exempt: row.Exempt,
	}
	if row.ReferenceNumber != nil {
		document.ReferenceNumber = *row.ReferenceNumber
	}
	if row.DebtorNumber != nil {
		document.DebtorNumber = *row.DebtorNumber
	}
	document.PaymentMethod, document.DueOn = textOf(row.PaymentMethod), row.DueOn
	document.Service, document.RecipientKind, document.RecipientSecondName = row.Service, textOf(row.RecipientKind), textOf(row.RecipientSecondName)
	return document
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// InsertRunEvents appends one entry to the trail of each holiday run
// (holiday) and of each monthly run (care), in one statement.
func (s *Store) InsertRunEvents(ctx context.Context, organizationID, accountID int64, action string, holiday, care map[int64]map[string]any) error {
	if len(holiday) == 0 && len(care) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	holidayEntries, err := runEventEntries(holiday)
	if err != nil {
		return err
	}
	careEntries, err := runEventEntries(care)
	if err != nil {
		return err
	}
	_, err = db.ExecContext(ctx, `WITH holiday AS (
			INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, details)
			SELECT ?0, entry.run_id, ?1, ?2, entry.details
			FROM jsonb_to_recordset(?3::jsonb) AS entry(run_id bigint, details jsonb))
		INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, actor_account_id, details)
		SELECT ?0, entry.run_id, ?1, ?2, entry.details
		FROM jsonb_to_recordset(?4::jsonb) AS entry(run_id bigint, details jsonb)`,
		organizationID, action, accountID, string(holidayEntries), string(careEntries))
	return err
}

// runEventEntries is the JSON of trail entries by run that the statements
// above insert with jsonb_to_recordset.
func runEventEntries(details map[int64]map[string]any) ([]byte, error) {
	type entry struct {
		RunID   int64          `json:"run_id"`
		Details map[string]any `json:"details"`
	}
	list := make([]entry, 0, len(details))
	for runID, detail := range details {
		if detail == nil {
			detail = map[string]any{}
		}
		list = append(list, entry{RunID: runID, Details: detail})
	}
	return json.Marshal(list)
}
