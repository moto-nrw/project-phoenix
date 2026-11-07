package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uptrace/bun/dialect/pgdialect"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
)

// The worker's checks for the push of a release for parents (#5941). Both
// run in the Träger transaction of the release and name the Träger again
// beside the Träger policy, so a missing policy still answers for no other
// Träger. Neither records a read: a push opens no document.

// activeParentReleaseQuery reads an active release for parents of an
// undeleted document.
const activeParentReleaseQuery = `
SELECT release.id, release.document_id, release.school_id, release.group_id
FROM platform.traeger_document_releases AS release
JOIN platform.traeger_documents AS document
	ON document.organization_id = release.organization_id AND document.id = release.document_id
	AND document.deleted_at IS NULL
WHERE release.organization_id = ? AND release.id = ? AND release.audience = 'parents' AND release.revoked_at IS NULL`

// releaseIsNewForParentQuery is true while the active release covers the
// parent, exactly as parentReleasesQuery does, and no older active release
// for parents of the same document covers it too: such a parent already has
// the document in its inbox.
//
// Bind order: Träger, release, schools, group schools, groups, schools,
// group schools, groups.
const releaseIsNewForParentQuery = `
SELECT EXISTS (
	SELECT 1 FROM platform.traeger_document_releases AS release
	JOIN platform.traeger_documents AS document
		ON document.organization_id = release.organization_id AND document.id = release.document_id
		AND document.deleted_at IS NULL
	WHERE release.organization_id = ? AND release.id = ? AND release.audience = 'parents' AND release.revoked_at IS NULL
		AND (release.school_id IS NULL
			OR (release.group_id IS NULL AND release.school_id = ANY(?::bigint[]))
			OR EXISTS (
				SELECT 1 FROM unnest(?::bigint[], ?::bigint[]) AS membership(school_id, group_id)
				WHERE membership.school_id = release.school_id AND membership.group_id = release.group_id
			))
		AND NOT EXISTS (
			SELECT 1 FROM platform.traeger_document_releases AS older
			WHERE older.organization_id = release.organization_id AND older.document_id = release.document_id
				AND older.audience = 'parents' AND older.revoked_at IS NULL
				AND (older.released_at, older.id) < (release.released_at, release.id)
				AND (older.school_id IS NULL
					OR (older.group_id IS NULL AND older.school_id = ANY(?::bigint[]))
					OR EXISTS (
						SELECT 1 FROM unnest(?::bigint[], ?::bigint[]) AS membership(school_id, group_id)
						WHERE membership.school_id = older.school_id AND membership.group_id = older.group_id
					))
		)
) AS is_new`

var _ organizationtenancy.ParentTraegerDocumentNotices = (*TraegerDocumentStore)(nil)

// ActiveParentRelease reads an active release for parents of an undeleted
// document of the Träger.
func (s *TraegerDocumentStore) ActiveParentRelease(ctx context.Context, organizationID, releaseID int64) (organizationtenancy.ParentReleaseAudience, bool, error) {
	if releaseID <= 0 {
		return organizationtenancy.ParentReleaseAudience{}, false, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.ParentReleaseAudience{}, false, err
	}
	var rows []struct {
		ID         int64  `bun:"id"`
		DocumentID int64  `bun:"document_id"`
		SchoolID   *int64 `bun:"school_id"`
		GroupID    *int64 `bun:"group_id"`
	}
	if err := db.NewRaw(activeParentReleaseQuery, organizationID, releaseID).Scan(ctx, &rows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return organizationtenancy.ParentReleaseAudience{}, false, fmt.Errorf("organization tenancy postgres: read release for its push: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.ParentReleaseAudience{}, false, nil
	}
	row := rows[0]
	return organizationtenancy.ParentReleaseAudience{ReleaseID: row.ID, DocumentID: row.DocumentID, SchoolID: row.SchoolID, GroupID: row.GroupID}, true, nil
}

// ReleaseIsNewForParent tells whether the release shows the parent a
// document no older active release showed it.
func (s *TraegerDocumentStore) ReleaseIsNewForParent(ctx context.Context, organizationID, releaseID int64, parent organizationtenancy.TraegerDocumentParent) (bool, error) {
	if releaseID <= 0 || len(parent.Schools) == 0 {
		return false, nil
	}
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	schools := make([]int64, 0, len(parent.Schools))
	groupSchools, groups := []int64{}, []int64{}
	for school, ids := range parent.Schools {
		schools = append(schools, school)
		for _, group := range ids {
			groupSchools, groups = append(groupSchools, school), append(groups, group)
		}
	}
	var isNew bool
	if err := db.NewRaw(releaseIsNewForParentQuery, organizationID, releaseID,
		pgdialect.Array(schools), pgdialect.Array(groupSchools), pgdialect.Array(groups),
		pgdialect.Array(schools), pgdialect.Array(groupSchools), pgdialect.Array(groups)).Scan(ctx, &isNew); err != nil {
		return false, fmt.Errorf("organization tenancy postgres: check release for its push: %w", err)
	}
	return isNew, nil
}
