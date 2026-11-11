package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/ports"
)

// UpsertTemplates joins the caller's Träger transaction. Preparation already
// validated the school copies, and every inserted row carries its Träger.
func (Store) UpsertTemplates(ctx context.Context, organizationID, accountID, schoolID int64, inputs []carecontract.ModuleTemplateInput) ([]ports.TemplateRow, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	type inputRow struct {
		OfferingID int64 `json:"care_offering_id"`
		carecontract.TemplateInput
	}
	payload := make([]inputRow, 0, len(inputs))
	for _, input := range inputs {
		payload = append(payload, inputRow{input.CareOfferingID, input.Input})
	}
	if len(payload) == 0 {
		return []ports.TemplateRow{}, nil
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	var rows []templateRow
	err = db.NewRaw(`INSERT INTO care_contract.contract_templates (organization_id,school_id,care_offering_id,
  term_kind,term_months,notice_months,notice_to,special_termination,special_termination_note,
  statute_required,statute_title,statute_url,contract_partner,updated_by_account_id)
  SELECT ?,?,batch.care_offering_id,batch.term_kind,batch.term_months,batch.notice_months,batch.notice_to,
   batch.special_termination,batch.special_termination_note,batch.statute_required,batch.statute_title,
   batch.statute_url,batch.contract_partner,?
  FROM jsonb_to_recordset(?::jsonb) AS batch(care_offering_id bigint,term_kind text,term_months int,
   notice_months int,notice_to text,special_termination bool,special_termination_note text,
   statute_required bool,statute_title text,statute_url text,contract_partner text)
  ORDER BY batch.care_offering_id
  ON CONFLICT (organization_id,school_id,care_offering_id) DO UPDATE SET
   term_kind=EXCLUDED.term_kind,term_months=EXCLUDED.term_months,notice_months=EXCLUDED.notice_months,
   notice_to=EXCLUDED.notice_to,special_termination=EXCLUDED.special_termination,
   special_termination_note=EXCLUDED.special_termination_note,statute_required=EXCLUDED.statute_required,
   statute_title=EXCLUDED.statute_title,statute_url=EXCLUDED.statute_url,contract_partner=EXCLUDED.contract_partner,
   updated_by_account_id=EXCLUDED.updated_by_account_id
  RETURNING `+templateColumns, organizationID, schoolID, accountID, string(encoded)).Scan(ctx, &rows)
	if err != nil {
		return nil, fmt.Errorf("care contract: save template batch: %w", err)
	}
	result := make([]ports.TemplateRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, ports.TemplateRow{SchoolID: schoolID, OfferingID: row.CareOfferingID, Template: row.template()})
	}
	return result, nil
}
