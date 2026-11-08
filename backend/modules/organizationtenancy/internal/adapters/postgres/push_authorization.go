package postgres

import (
	"context"
	"encoding/json"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The immutable release ID is the event identity: a later release never
// revives a revoked one. Current group membership controls group releases.
const validReleasePushesSQL = `SELECT candidate.key
 FROM jsonb_to_recordset(?::jsonb) AS candidate(key bigint,release_id bigint,school_id bigint,group_ids jsonb)
 JOIN platform.traeger_document_releases AS release ON release.id=candidate.release_id AND release.organization_id=?
 JOIN platform.traeger_documents AS document ON document.organization_id=release.organization_id AND document.id=release.document_id AND document.deleted_at IS NULL
 WHERE release.audience='parents' AND release.revoked_at IS NULL
 AND (release.school_id IS NULL OR (release.school_id=candidate.school_id
  AND (release.group_id IS NULL OR release.group_id IN (SELECT value::bigint FROM jsonb_array_elements_text(candidate.group_ids)))))
 AND NOT EXISTS (SELECT 1 FROM platform.traeger_document_releases AS older
  WHERE older.organization_id=release.organization_id AND older.document_id=release.document_id
  AND older.audience='parents' AND older.revoked_at IS NULL AND (older.released_at,older.id)<(release.released_at,release.id)
  AND (older.school_id IS NULL OR (older.school_id=candidate.school_id
   AND (older.group_id IS NULL OR older.group_id IN (SELECT value::bigint FROM jsonb_array_elements_text(candidate.group_ids))))))`

func (s *TraegerDocumentStore) ValidReleasePushes(ctx context.Context, organizationID int64, candidates []organizationtenancy.ReleasePushCandidate) (map[int64]bool, error) {
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
	if err := db.NewRaw(validReleasePushesSQL, string(encoded), organizationID).Scan(ctx, &keys); err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(keys))
	for _, key := range keys {
		result[key] = true
	}
	return result, nil
}
