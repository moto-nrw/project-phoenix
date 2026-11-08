package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The event is the first mailing to the address, not whichever version a
// later resend put in the inbox. Replacing that version cannot revive it.
const validContractPushesSQL = `SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint,contract_id bigint,email text,school_id bigint,student_ids jsonb)
 JOIN care_contract.contracts AS c ON c.id=candidate.contract_id AND c.organization_id=? AND c.school_id=candidate.school_id
 JOIN LATERAL (SELECT m.contract_document_id FROM care_contract.contract_mailings AS m
  WHERE m.organization_id=c.organization_id AND m.contract_id=c.id AND lower(m.recipient_email)=lower(candidate.email)
  ORDER BY m.sent_at,m.id LIMIT 1) AS first_send ON TRUE
 JOIN care_contract.contract_documents AS d ON d.organization_id=c.organization_id AND d.id=first_send.contract_document_id AND d.contract_id=c.id
 JOIN care_contract.contract_versions AS v ON v.organization_id=c.organization_id AND v.id=d.contract_version_id AND v.contract_id=c.id
 WHERE c.status IN ('` + carecontract.StatusSent + `','` + carecontract.StatusActive + `')
 AND c.student_id IN (SELECT value::bigint FROM jsonb_array_elements_text(candidate.student_ids))
 AND v.status NOT IN ('` + carecontract.VersionReplaced + `','` + carecontract.VersionSuperseded + `','` + carecontract.VersionRejected + `')
 AND NOT EXISTS (SELECT 1 FROM care_contract.contract_versions AS newer
  WHERE newer.organization_id=c.organization_id AND newer.contract_id=c.id AND newer.version_number>v.version_number
  AND newer.status IN ('` + carecontract.VersionEffective + `','` + carecontract.VersionSuperseded + `'))`

func (Store) ValidContractPushes(ctx context.Context, organizationID int64, candidates []carecontract.DocumentPushCandidate) (map[int64]bool, error) {
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
	if err := db.NewRaw(validContractPushesSQL, string(encoded), organizationID).Scan(ctx, &keys); err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(keys))
	for _, key := range keys {
		result[key] = true
	}
	return result, nil
}
