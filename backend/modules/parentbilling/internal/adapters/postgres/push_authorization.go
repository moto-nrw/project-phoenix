package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// A historical invoice stays downloadable after cancellation, but its old
// announcement is no longer current. Cancellation/credit notices themselves
// remain valid. Read all candidates of one Träger in one statement.
const validDocumentPushesSQL = `SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint, document_id bigint, email text)
 JOIN parent_billing.documents AS document ON document.id = candidate.document_id AND document.organization_id = ?
 JOIN parent_billing.runs AS run ON run.id = document.run_id AND run.organization_id = document.organization_id
 WHERE LOWER(document.recipient_email) = LOWER(candidate.email) AND run.status = 'released'
 AND EXISTS (SELECT 1 FROM parent_billing.document_letters AS letter
  WHERE letter.document_id = document.id AND letter.organization_id = document.organization_id)
 AND EXISTS (SELECT 1 FROM parent_billing.document_mailings AS mailing
  WHERE mailing.document_id = document.id AND mailing.organization_id = document.organization_id)
 AND ` + notCancelledSQL

func (s *Store) ValidDocumentPushes(ctx context.Context, organizationID int64, candidates []parentbilling.DocumentPushCandidate) (map[int64]bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(candidates)
	if err != nil {
		return nil, err
	}
	var keys []int64
	if err := db.NewRaw(validDocumentPushesSQL, string(encoded), organizationID).Scan(ctx, &keys); err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(keys))
	for _, key := range keys {
		result[key] = true
	}
	return result, nil
}
