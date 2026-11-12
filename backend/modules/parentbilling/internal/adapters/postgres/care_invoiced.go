package postgres

import "github.com/moto-ogs/project-phoenix/modules/parentbilling"

// The contracts a month already bills (#4684, #6219): the issued monthly
// invoices and the months of the Jahresbescheide nobody amended, read by
// InvoicedContracts in one statement.

// invoicedContractRow is one position of an issued invoice or of a month
// of a Jahresbescheid, as invoicedContractsSQL reads it.
type invoicedContractRow struct {
	InvoiceID      int64
	ContractID     int64
	SchoolID       int64
	RecipientEmail string
	FamilyEmail    string
	TotalCents     int64
	CareOfferingID int64
	AmountCents    int64
	PayableCents   int64
	Reductions     []parentbilling.CareReduction `bun:",type:jsonb"`
	NoticeNumber   string
	// Source names the invoice or notice month a row comes from;
	// CostShares are the parts its cost bearers already took (ADR 0169).
	Source     string
	CostShares []parentbilling.CareInvoiceCostShare `bun:",type:jsonb"`
}

// invoicedContractsSQL reads the positions of the issued monthly invoices
// of a month and of the months of the Jahresbescheide nobody amended.
const invoicedContractsSQL = `SELECT invoice_id, contract_id, school_id, recipient_email, family_email, total_cents, care_offering_id,
		amount_cents, payable_cents, reductions, notice_number, source, cost_shares FROM (
		SELECT i.id AS invoice_id, i.care_contract_id AS contract_id, i.school_id, i.recipient_email,
		COALESCE(i.family_email, '') AS family_email, i.total_cents,
		COALESCE(p.care_offering_id, 0) AS care_offering_id, COALESCE(p.amount_cents, 0) AS amount_cents,
		COALESCE(p.payable_cents, p.amount_cents, 0) AS payable_cents,
		COALESCE((SELECT jsonb_agg(to_jsonb(r) ORDER BY r.position)
			FROM parent_billing.care_invoice_reductions r
			WHERE r.invoice_id = i.id), '[]'::jsonb) AS reductions,
		'' AS notice_number, COALESCE(p.position, 0)::bigint AS ord,
		'invoice:' || i.id AS source,
		COALESCE((SELECT jsonb_agg(jsonb_build_object(
			'kind', share.kind, 'cost_bearer_id', share.cost_bearer_id, 'cost_bearer_name', share.cost_bearer_name,
			'share_basis_points', share.share_basis_points, 'share_cents', share.share_cents, 'scope', share.scope,
			'amount_cents', share.amount_cents) ORDER BY share.position)
			FROM parent_billing.care_invoice_cost_shares AS share
			WHERE share.organization_id = i.organization_id AND share.invoice_id = i.id), '[]'::jsonb) AS cost_shares
		FROM parent_billing.care_invoices i
		LEFT JOIN parent_billing.care_invoice_positions p ON p.invoice_id = i.id
		WHERE i.organization_id = ? AND i.billing_month = ?
			-- The care's invoice; a Mensa invoice of its own (#4686) does not
			-- bill the contract's care.
			AND i.service = 'care'
			AND (p.id IS NOT NULL OR EXISTS (SELECT 1 FROM parent_billing.care_meal_positions AS m
				WHERE m.organization_id = i.organization_id AND m.invoice_id = i.id))
			-- A cancelled invoice bills nothing any more (#4684, ADR 0165):
			-- a later run of the month bills its contract again.
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_invoice_cancellations AS cancellation
				WHERE cancellation.organization_id = i.organization_id AND cancellation.invoice_id = i.id)
		UNION ALL
		SELECT 0::bigint, m.care_contract_id, n.school_id, n.recipient_email, n.family_email, m.amount_cents,
		COALESCE((p.position->>'care_offering_id')::bigint, 0), COALESCE((p.position->>'amount_cents')::bigint, 0),
		COALESCE((m.snapshot->'payable'->>(p.ord - 1)::int)::bigint, (p.position->>'amount_cents')::bigint, 0),
		COALESCE(NULLIF(m.snapshot->'invoice'->'reductions', 'null'::jsonb), '[]'::jsonb),
		n.number, COALESCE(p.ord, 0),
		'notice:' || m.id,
		COALESCE(NULLIF(m.snapshot->'invoice'->'cost_shares', 'null'::jsonb), '[]'::jsonb)
		FROM parent_billing.care_notice_months AS m
		JOIN parent_billing.care_notices AS n ON n.organization_id = m.organization_id AND n.id = m.notice_id
		LEFT JOIN LATERAL jsonb_array_elements(COALESCE(NULLIF(m.snapshot->'invoice'->'positions', 'null'::jsonb), '[]'::jsonb))
			WITH ORDINALITY AS p(position, ord) ON true
		WHERE m.organization_id = ? AND m.billing_month = ?
			AND NOT EXISTS (SELECT 1 FROM parent_billing.care_notices AS amendment
				WHERE amendment.organization_id = n.organization_id AND amendment.amends_notice_id = n.id)
		) AS billed
		ORDER BY contract_id, notice_number, ord`

// invoicedFromRows folds the positions into one billed entry per contract.
// The cost bearer parts of every source of the contract add up once each:
// a fixed share takes at most its amount per month, whether a monthly
// invoice or a Jahresbescheid took it (ADR 0169).
func invoicedFromRows(rows []invoicedContractRow) map[int64]parentbilling.CareInvoice {
	invoiced := make(map[int64]parentbilling.CareInvoice)
	seen := map[string]bool{}
	for _, row := range rows {
		invoice := invoiced[row.ContractID]
		if !seen[row.Source] {
			seen[row.Source] = true
			invoice.CostShares = append(invoice.CostShares, row.CostShares...)
		}
		// A meal-only care invoice can coexist with a notice. Keep its ID
		// while the notice supplies the care assessment for the rule chain.
		if row.InvoiceID != 0 {
			invoice.ID = row.InvoiceID
		}
		invoice.ContractID, invoice.SchoolID = row.ContractID, row.SchoolID
		invoice.Recipient.Email, invoice.TotalCents = row.RecipientEmail, row.TotalCents
		invoice.FamilyEmail = row.FamilyEmail
		invoice.Reductions = row.Reductions
		if row.NoticeNumber != "" {
			invoice.Number, invoice.DocumentKind = row.NoticeNumber, parentbilling.DocumentKindFeeNotice
		}
		if row.CareOfferingID != 0 {
			invoice.Positions = append(invoice.Positions, parentbilling.CarePosition{
				CareOfferingID: row.CareOfferingID, AmountCents: row.AmountCents, PayableCents: row.PayableCents,
			})
		}
		invoiced[row.ContractID] = invoice
	}
	return invoiced
}
