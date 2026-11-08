package postgres

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The documents of the families of a Träger (#4691). A family is the mail
// address of the registration the release copied (#4689), so a payer who
// gets the invoice does not split the family; a cancellation or credit
// belongs to the family of its invoice, a document issued before #4689 to
// its recipient address, compared without case. The COALESCE in
// familyDocumentsSQL is the one place that says so. The kinds of the
// positions are parentbilling.LineCostTakeover and LineBenefitsExemption.

// familyDocumentsSQL reads the issued documents of the Träger, the oldest
// first, with the period of their run, the first names of their children,
// the cost bearers their positions name and whether the exemption freed
// them. The second filter keeps the documents of one family: the family of
// the document with the given ID, or every family when that ID is zero.
const familyDocumentsSQL = `SELECT document.id, document.run_id, document.kind, document.number, document.issued_on,
		run.period_name, document.service_from, document.service_until, document.reference_document_id,
		reference.number AS reference_number, document.reason, document.recipient_first_name, document.recipient_last_name,
		document.recipient_email, document.debtor_number, document.total_cents, COALESCE(document.family_email, reference.family_email, LOWER(document.recipient_email)) AS family_key,
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
	WHERE document.organization_id = ? AND (?::bigint = 0 OR COALESCE(document.family_email, reference.family_email, LOWER(document.recipient_email)) = (
		SELECT COALESCE(anchor.family_email, anchor_reference.family_email, LOWER(anchor.recipient_email)) FROM parent_billing.documents AS anchor
		LEFT JOIN parent_billing.documents AS anchor_reference
			ON anchor_reference.organization_id = anchor.organization_id AND anchor_reference.id = anchor.reference_document_id
		WHERE anchor.organization_id = document.organization_id AND anchor.id = ?))
	ORDER BY document.id`

type familyDocumentRow struct {
	ID                  int64          `bun:"id"`
	RunID               int64          `bun:"run_id"`
	Kind                string         `bun:"kind"`
	Number              string         `bun:"number"`
	IssuedOn            calendar.Date  `bun:"issued_on,type:date"`
	PeriodName          string         `bun:"period_name"`
	ServiceFrom         calendar.Date  `bun:"service_from,type:date"`
	ServiceUntil        calendar.Date  `bun:"service_until,type:date"`
	ReferenceDocumentID *int64         `bun:"reference_document_id"`
	ReferenceNumber     *string        `bun:"reference_number"`
	Reason              string         `bun:"reason"`
	RecipientFirstName  string         `bun:"recipient_first_name"`
	RecipientLastName   string         `bun:"recipient_last_name"`
	RecipientEmail      string         `bun:"recipient_email"`
	DebtorNumber        *string        `bun:"debtor_number"`
	TotalCents          int64          `bun:"total_cents"`
	FamilyKey           string         `bun:"family_key"`
	PaymentMethod       *string        `bun:"payment_method"`
	DueOn               *calendar.Date `bun:"due_on,type:date"`
	ChildNames          []string       `bun:"child_names,type:json"`
	CostBearerNames     []string       `bun:"cost_bearer_names,type:json"`
	Exempt              bool           `bun:"exempt"`
}

// FamilyDocuments reads the issued documents of the family of the document
// familyID, or of every family when familyID is zero, the oldest first.
func (s *Store) FamilyDocuments(ctx context.Context, organizationID, familyID int64) ([]parentbilling.BillingFamilyDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var rows []familyDocumentRow
	if err := db.NewRaw(familyDocumentsSQL, organizationID, familyID, familyID).Scan(ctx, &rows); err != nil {
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
	return document
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// InsertRunEvents appends one entry to the trail of each run, in one
// statement.
func (s *Store) InsertRunEvents(ctx context.Context, organizationID, accountID int64, action string, details map[int64]map[string]any) error {
	if len(details) == 0 {
		return nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return err
	}
	rows := make([]runEventRow, 0, len(details))
	for runID, detail := range details {
		if detail == nil {
			detail = map[string]any{}
		}
		rows = append(rows, runEventRow{
			OrganizationID: organizationID, RunID: runID, Action: action, ActorAccountID: &accountID, Details: detail,
		})
	}
	_, err = db.NewInsert().Model(&rows).Exec(ctx)
	return err
}
