package migrations

import (
	"context"
	"fmt"
	"strings"

	"github.com/uptrace/bun"
)

// careInvoiceExclusionErasureAnchor is the statement of the erasure of
// 1.15.843 the new statement goes before: the exclusions and returns of the
// school's contracts go next, and the contracts they name exist until then.
const careInvoiceExclusionErasureAnchor = "\t\t\t\t\tDELETE FROM parent_billing.care_run_events AS event\n"

// careInvoiceExclusionErasureStatement takes the retired demo school's
// contracts out of the release entries of the monthly runs (#6346). A
// release names the invoices it left out, by contract and service
// (excluded_invoices) and by contract (excluded_contract_ids). The entry
// itself belongs to the run, which may bill other schools of the Träger, so
// it stays with the contracts of the other schools; only the retired
// school's contracts leave it.
const careInvoiceExclusionErasureStatement = `					-- A release names the invoices it left out by their contract
					-- (#6346); the retired school's contracts leave the entry,
					-- which stays with the run and the other schools.
					UPDATE parent_billing.care_run_events AS event
					SET details = event.details
						|| jsonb_build_object('excluded_contract_ids', COALESCE((
							SELECT jsonb_agg(kept.value ORDER BY kept.position)
							FROM jsonb_array_elements(COALESCE(event.details->'excluded_contract_ids', '[]'::jsonb))
								WITH ORDINALITY AS kept(value, position)
							WHERE NOT EXISTS (SELECT 1 FROM care_contract.contracts AS contract
								WHERE contract.school_id = ANY (hidden_schools)
									AND contract.organization_id = event.organization_id
									AND to_jsonb(contract.id) = kept.value)), '[]'::jsonb))
						|| jsonb_build_object('excluded_invoices', COALESCE((
							SELECT jsonb_agg(kept.value ORDER BY kept.position)
							FROM jsonb_array_elements(COALESCE(event.details->'excluded_invoices', '[]'::jsonb))
								WITH ORDINALITY AS kept(value, position)
							WHERE NOT EXISTS (SELECT 1 FROM care_contract.contracts AS contract
								WHERE contract.school_id = ANY (hidden_schools)
									AND contract.organization_id = event.organization_id
									AND to_jsonb(contract.id) = kept.value->'contract_id')), '[]'::jsonb))
					WHERE event.action = 'release'
						AND EXISTS (SELECT 1 FROM care_contract.contracts AS contract
							WHERE contract.school_id = ANY (hidden_schools)
								AND contract.organization_id = event.organization_id
								AND (event.details->'excluded_contract_ids' @> to_jsonb(contract.id)
									OR event.details->'excluded_invoices' @> jsonb_build_array(jsonb_build_object('contract_id', contract.id))));
`

// demoVisitorErasureCareInvoiceExclusionsSQL is platform.erase_closed_demo_visitors
// of 1.15.843 (#6372, with its outbox kinds and the export files of #6361)
// with careInvoiceExclusionErasureStatement before the exclusions of the
// school's contracts go. Nothing else changes.
var demoVisitorErasureCareInvoiceExclusionsSQL = strings.Replace(demoVisitorErasureOutboxKindsSQL,
	careInvoiceExclusionErasureAnchor, careInvoiceExclusionErasureStatement+careInvoiceExclusionErasureAnchor, 1)

// installCareInvoiceExclusionErasure installs the erasure of
// parentBillingCareInvoiceExclusionsVersion.
func installCareInvoiceExclusionErasure(ctx context.Context, db bun.IDB) error {
	if _, err := db.ExecContext(ctx, demoVisitorErasureCareInvoiceExclusionsSQL); err != nil {
		return fmt.Errorf("erase the retired demo school's contracts from the release entries: %w", err)
	}
	return nil
}
