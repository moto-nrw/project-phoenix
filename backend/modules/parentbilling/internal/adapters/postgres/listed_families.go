package postgres

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// ReadListedFamilies reads documents and remembered families in one snapshot,
// and records every read in the same statement. Combining the two existing
// reads leaves room for the contract owner's batched membership lookup within
// the list's fixed query budget.
func (s *Store) ReadListedFamilies(ctx context.Context, organizationID, accountID int64) ([]parentbilling.BillingFamilyDocument, []parentbilling.FamilyBilling, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, nil, err
	}
	var rows struct {
		Documents []familyDocumentRow `bun:"documents,type:json"`
		Families  []familyBillingRow  `bun:"families,type:json"`
	}
	if err := db.NewRaw(`WITH documents AS MATERIALIZED (`+familyDocumentsSQL+`),
  holiday_reads AS (
   INSERT INTO parent_billing.run_events (organization_id, run_id, action, actor_account_id, details)
   SELECT ?0, run_id, 'read', ?2, jsonb_build_object('view', 'families', 'document_ids',
    jsonb_agg(id ORDER BY issued_at, service DESC, id))
   FROM documents WHERE service = 'holiday' GROUP BY run_id),
  care_reads AS (
   INSERT INTO parent_billing.care_run_events (organization_id, run_id, action, actor_account_id, details)
   SELECT ?0, run_id, 'read', ?2, jsonb_build_object('view', 'families', 'document_ids',
    jsonb_agg(id ORDER BY issued_at, service DESC, id))
   FROM documents WHERE service = 'care' GROUP BY run_id),
  shown AS MATERIALIZED (
   SELECT family.* FROM parent_billing.family_billing AS family
   WHERE family.organization_id = ?0 AND (family.care_contract_id IS NULL OR family.debtor_number IS NOT NULL
    OR family.family_email IN (SELECT family_key FROM documents))),
  family_reads AS (
   INSERT INTO parent_billing.family_billing_events (organization_id, family_billing_id, care_contract_id, action, actor_account_id)
   SELECT ?0, id, care_contract_id, 'read', ?2 FROM shown)
 SELECT COALESCE((SELECT jsonb_agg(documents ORDER BY issued_at, service DESC, id) FROM documents), '[]'::jsonb) AS documents,
  COALESCE((SELECT jsonb_agg(shown ORDER BY family_email) FROM shown), '[]'::jsonb) AS families`,
		organizationID, int64(0), accountRef(accountID)).Scan(ctx, &rows); err != nil {
		return nil, nil, fmt.Errorf("read the families and record their reads: %w", err)
	}
	documents := make([]parentbilling.BillingFamilyDocument, len(rows.Documents))
	for i, row := range rows.Documents {
		documents[i] = row.document()
	}
	return documents, familiesOfRows(rows.Families), nil
}
