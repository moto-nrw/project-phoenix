package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// Shared documents of a Träger (#4692). The tables carry organization_id
// and the Träger policy of ADR 0105, so the Träger statements run in the
// Träger transaction of the Träger they name and name it again. A school
// reads no row of them: its statements call the SECURITY DEFINER functions
// of migration 1.15.760, which answer for the school of the tenant
// transaction with the documents an active release covers.

// traegerDocumentReadRetention is how long a read stays recorded, the three
// years of the master data log.
const traegerDocumentReadRetention = "3 years"

// TraegerDocumentStore persists the shared documents.
type TraegerDocumentStore struct{}

// NewTraegerDocumentStore builds the store.
func NewTraegerDocumentStore() *TraegerDocumentStore { return &TraegerDocumentStore{} }

// schoolDB returns the tenant transaction of schoolID. Any other
// transaction is refused.
func schoolDB(ctx context.Context, schoolID int64) (bun.IDB, error) {
	current, err := tenant.TenantFromContext(ctx)
	if err != nil || schoolID <= 0 || current.Int64() != schoolID || tenant.IsAdminTx(ctx) {
		return nil, errors.New("organization tenancy postgres: the tenant transaction of this school is required")
	}
	return NewDatabase()(ctx)
}

type traegerDocumentRow struct {
	ID          int64     `bun:"id"`
	Title       string    `bun:"title"`
	FileName    string    `bun:"file_name"`
	ContentType string    `bun:"content_type"`
	SizeBytes   int       `bun:"size_bytes"`
	CreatedAt   time.Time `bun:"created_at"`
	Reads       int       `bun:"reads"`
}

type traegerReleaseRow struct {
	ID         int64     `bun:"id"`
	DocumentID int64     `bun:"document_id"`
	Audience   string    `bun:"audience"`
	SchoolID   *int64    `bun:"school_id"`
	GroupID    *int64    `bun:"group_id"`
	GroupName  *string   `bun:"group_name"`
	ReleasedAt time.Time `bun:"released_at"`
}

func (r traegerReleaseRow) toDomain() organizationtenancy.TraegerDocumentRelease {
	release := organizationtenancy.TraegerDocumentRelease{
		ID: r.ID, Audience: organizationtenancy.TraegerDocumentAudience(r.Audience),
		SchoolID: r.SchoolID, GroupID: r.GroupID, ReleasedAt: r.ReleasedAt,
	}
	if r.GroupName != nil {
		release.GroupName = *r.GroupName
	}
	return release
}

type traegerFileRow struct {
	FileName    string `bun:"file_name"`
	ContentType string `bun:"content_type"`
	Content     []byte `bun:"content"`
}

func (r traegerFileRow) toDomain() organizationtenancy.TraegerDocumentFile {
	return organizationtenancy.TraegerDocumentFile{FileName: r.FileName, ContentType: r.ContentType, Data: r.Content}
}

const listTraegerDocumentsQuery = `
SELECT document.id, document.title, document.file_name, document.content_type, document.size_bytes, document.created_at,
	(SELECT COUNT(*) FROM platform.traeger_document_reads AS reading
		WHERE reading.organization_id = document.organization_id AND reading.document_id = document.id) AS reads
FROM platform.traeger_documents AS document
WHERE document.organization_id = ?
ORDER BY document.created_at DESC, document.id DESC`

const listTraegerReleasesQuery = `
SELECT id, document_id, audience, school_id, group_id, group_name, released_at
FROM platform.traeger_document_releases
WHERE organization_id = ? AND revoked_at IS NULL
ORDER BY released_at, id`

// ListDocuments lists the documents with their active releases and read
// counts in two statements.
func (s *TraegerDocumentStore) ListDocuments(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	var documents []traegerDocumentRow
	if err := db.NewRaw(listTraegerDocumentsQuery, organizationID).Scan(ctx, &documents); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list Träger documents: %w", err)
	}
	var releases []traegerReleaseRow
	if err := db.NewRaw(listTraegerReleasesQuery, organizationID).Scan(ctx, &releases); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list Träger document releases: %w", err)
	}
	byDocument := map[int64][]organizationtenancy.TraegerDocumentRelease{}
	for _, release := range releases {
		byDocument[release.DocumentID] = append(byDocument[release.DocumentID], release.toDomain())
	}
	result := make([]organizationtenancy.TraegerDocument, 0, len(documents))
	for _, row := range documents {
		result = append(result, row.toDomain(byDocument[row.ID]))
	}
	return result, nil
}

func (r traegerDocumentRow) toDomain(releases []organizationtenancy.TraegerDocumentRelease) organizationtenancy.TraegerDocument {
	if releases == nil {
		releases = []organizationtenancy.TraegerDocumentRelease{}
	}
	return organizationtenancy.TraegerDocument{
		ID: r.ID, Title: r.Title, FileName: r.FileName, ContentType: r.ContentType,
		SizeBytes: r.SizeBytes, CreatedAt: r.CreatedAt, Releases: releases, Reads: r.Reads,
	}
}

const insertTraegerDocumentQuery = `
INSERT INTO platform.traeger_documents (organization_id, title, file_name, content_type, size_bytes, content, created_by)
VALUES (?, ?, ?, ?, ?, ?, ?)
RETURNING id, title, file_name, content_type, size_bytes, created_at, 0 AS reads`

// InsertDocument adds a checked document.
func (s *TraegerDocumentStore) InsertDocument(ctx context.Context, organizationID, accountID int64, document domain.TraegerDocumentWrite) (organizationtenancy.TraegerDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocument{}, err
	}
	var row traegerDocumentRow
	err = db.NewRaw(insertTraegerDocumentQuery, organizationID, document.Title, document.FileName, document.ContentType,
		len(document.Data), document.Data, traegerActor(accountID)).Scan(ctx, &row)
	if err != nil {
		return organizationtenancy.TraegerDocument{}, fmt.Errorf("organization tenancy postgres: insert Träger document: %w", err)
	}
	return row.toDomain(nil), nil
}

// DocumentFile reads the file of a document of the Träger.
func (s *TraegerDocumentStore) DocumentFile(ctx context.Context, organizationID, documentID int64) (organizationtenancy.TraegerDocumentFile, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, false, err
	}
	var rows []traegerFileRow
	if err := db.NewRaw(`SELECT file_name, content_type, content FROM platform.traeger_documents WHERE organization_id = ? AND id = ?`,
		organizationID, documentID).Scan(ctx, &rows); err != nil {
		return organizationtenancy.TraegerDocumentFile{}, false, fmt.Errorf("organization tenancy postgres: read Träger document: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerDocumentFile{}, false, nil
	}
	return rows[0].toDomain(), true, nil
}

const insertTraegerReleaseQuery = `
INSERT INTO platform.traeger_document_releases (organization_id, document_id, audience, school_id, group_id, group_name, released_by)
SELECT document.organization_id, document.id, ?, ?, ?, ?, ?
FROM platform.traeger_documents AS document
WHERE document.organization_id = ? AND document.id = ?
RETURNING id, document_id, audience, school_id, group_id, group_name, released_at`

// InsertRelease adds an active release of a document of the Träger.
func (s *TraegerDocumentStore) InsertRelease(ctx context.Context, organizationID, accountID, documentID int64, input organizationtenancy.TraegerDocumentReleaseInput, groupName string) (organizationtenancy.TraegerDocumentRelease, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocumentRelease{}, err
	}
	var name *string
	if input.GroupID != nil {
		name = &groupName
	}
	var rows []traegerReleaseRow
	err = db.NewRaw(insertTraegerReleaseQuery, string(input.Audience), input.SchoolID, input.GroupID, name,
		traegerActor(accountID), organizationID, documentID).Scan(ctx, &rows)
	var postgresError pgdriver.Error
	switch {
	case errors.As(err, &postgresError) && postgresError.Field('C') == "23505":
		return organizationtenancy.TraegerDocumentRelease{}, organizationtenancy.ErrTraegerDocumentAlreadyReleased
	case errors.As(err, &postgresError) && postgresError.Field('C') == "23503":
		return organizationtenancy.TraegerDocumentRelease{}, organizationtenancy.ErrTraegerDocumentAudienceInvalid
	case err != nil:
		return organizationtenancy.TraegerDocumentRelease{}, fmt.Errorf("organization tenancy postgres: release Träger document: %w", err)
	case len(rows) == 0:
		return organizationtenancy.TraegerDocumentRelease{}, organizationtenancy.ErrTraegerDocumentNotFound
	}
	return rows[0].toDomain(), nil
}

// RevokeRelease revokes an active release of the document.
func (s *TraegerDocumentStore) RevokeRelease(ctx context.Context, organizationID, accountID, documentID, releaseID int64) (bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return false, err
	}
	result, err := db.ExecContext(ctx, `
		UPDATE platform.traeger_document_releases SET revoked_at = CURRENT_TIMESTAMP, revoked_by = ?
		WHERE organization_id = ? AND document_id = ? AND id = ? AND revoked_at IS NULL`,
		traegerActor(accountID), organizationID, documentID, releaseID)
	if err != nil {
		return false, fmt.Errorf("organization tenancy postgres: revoke Träger document release: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("organization tenancy postgres: revoke Träger document release: %w", err)
	}
	return affected == 1, nil
}

// parentReleasesQuery selects the active parent releases that cover the
// parent: for every school of the Träger, for one of the parent's schools,
// or for a group of a child at one of them. parentDocumentReleaseQuery
// narrows it to one document.
const parentReleasesQuery = `
SELECT release.id, release.document_id, release.audience, release.school_id, release.group_id, release.group_name, release.released_at,
	document.title, document.file_name, document.content_type, document.size_bytes
FROM platform.traeger_document_releases AS release
JOIN platform.traeger_documents AS document
	ON document.organization_id = release.organization_id AND document.id = release.document_id
WHERE release.organization_id = ? AND release.audience = 'parents' AND release.revoked_at IS NULL
	AND (release.school_id IS NULL
		OR (release.group_id IS NULL AND release.school_id = ANY(?::bigint[]))
		OR EXISTS (
			SELECT 1 FROM unnest(?::bigint[], ?::bigint[]) AS membership(school_id, group_id)
			WHERE membership.school_id = release.school_id AND membership.group_id = release.group_id
		))`

const parentDocumentReleaseQuery = `
SELECT release.id, release.document_id, release.audience, release.school_id, release.group_id, release.group_name, release.released_at,
	document.title, document.file_name, document.content_type, document.size_bytes
FROM platform.traeger_document_releases AS release
JOIN platform.traeger_documents AS document
	ON document.organization_id = release.organization_id AND document.id = release.document_id
WHERE release.organization_id = ? AND release.audience = 'parents' AND release.revoked_at IS NULL
	AND (release.school_id IS NULL
		OR (release.group_id IS NULL AND release.school_id = ANY(?::bigint[]))
		OR EXISTS (
			SELECT 1 FROM unnest(?::bigint[], ?::bigint[]) AS membership(school_id, group_id)
			WHERE membership.school_id = release.school_id AND membership.group_id = release.group_id
		))
	AND release.document_id = ?
LIMIT 1`

type sharedReleaseRow struct {
	traegerReleaseRow
	Title       string `bun:"title"`
	FileName    string `bun:"file_name"`
	ContentType string `bun:"content_type"`
	SizeBytes   int    `bun:"size_bytes"`
}

// parentReleases runs one of the two parent queries with the parent's
// schools and groups; documentID narrows parentDocumentReleaseQuery.
func parentReleases(ctx context.Context, db bun.IDB, organizationID int64, parent organizationtenancy.TraegerDocumentParent, documentID int64) ([]sharedReleaseRow, error) {
	schools := make([]int64, 0, len(parent.Schools))
	var groupSchools, groups []int64
	for school, ids := range parent.Schools {
		schools = append(schools, school)
		for _, group := range ids {
			groupSchools, groups = append(groupSchools, school), append(groups, group)
		}
	}
	if len(schools) == 0 {
		return nil, nil
	}
	var rows []sharedReleaseRow
	var err error
	if documentID > 0 {
		err = db.NewRaw(parentDocumentReleaseQuery, organizationID, pgdialect.Array(schools), pgdialect.Array(groupSchools), pgdialect.Array(groups), documentID).Scan(ctx, &rows)
	} else {
		err = db.NewRaw(parentReleasesQuery, organizationID, pgdialect.Array(schools), pgdialect.Array(groupSchools), pgdialect.Array(groups)).Scan(ctx, &rows)
	}
	if err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list parent releases: %w", err)
	}
	return rows, nil
}

// ParentDocuments lists the documents released for the parent.
func (s *TraegerDocumentStore) ParentDocuments(ctx context.Context, organizationID int64, parent organizationtenancy.TraegerDocumentParent) ([]organizationtenancy.SharedDocument, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return nil, err
	}
	rows, err := parentReleases(ctx, db, organizationID, parent, 0)
	if err != nil {
		return nil, err
	}
	return sharedDocuments(rows), nil
}

// OpenParentDocument reads a document released for the parent and records
// the read.
func (s *TraegerDocumentStore) OpenParentDocument(ctx context.Context, organizationID, documentID int64, parent organizationtenancy.TraegerDocumentParent) (organizationtenancy.TraegerDocumentFile, bool, error) {
	db, err := traegerDB(ctx, organizationID)
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, false, err
	}
	if documentID <= 0 {
		return organizationtenancy.TraegerDocumentFile{}, false, nil
	}
	rows, err := parentReleases(ctx, db, organizationID, parent, documentID)
	if err != nil || len(rows) == 0 {
		return organizationtenancy.TraegerDocumentFile{}, false, err
	}
	if err := appendRead(ctx, db, organizationID, documentID, parent.AccountID); err != nil {
		return organizationtenancy.TraegerDocumentFile{}, false, err
	}
	file, found, err := s.DocumentFile(ctx, organizationID, documentID)
	return file, found, err
}

// appendRead records a parent's read and deletes the Träger's reads older
// than the retention in the same transaction.
func appendRead(ctx context.Context, db bun.IDB, organizationID, documentID, accountID int64) error {
	if _, err := db.ExecContext(ctx, `
		INSERT INTO platform.traeger_document_reads (organization_id, document_id, account_id, reader) VALUES (?, ?, ?, 'parent')`,
		organizationID, documentID, traegerActor(accountID)); err != nil {
		return fmt.Errorf("organization tenancy postgres: record Träger document read: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		DELETE FROM platform.traeger_document_reads
		WHERE organization_id = ? AND read_at < CURRENT_TIMESTAMP - INTERVAL '`+traegerDocumentReadRetention+`'`,
		organizationID); err != nil {
		return fmt.Errorf("organization tenancy postgres: prune Träger document reads: %w", err)
	}
	return nil
}

// SchoolDocuments lists the documents released for the school or for the
// parents at it.
func (s *TraegerDocumentStore) SchoolDocuments(ctx context.Context, schoolID int64) ([]organizationtenancy.SharedDocument, error) {
	db, err := schoolDB(ctx, schoolID)
	if err != nil {
		return nil, err
	}
	var rows []sharedReleaseRow
	if err := db.NewRaw(`
		SELECT release_id AS id, document_id, audience, school_id, group_id, group_name, released_at,
			title, file_name, content_type, size_bytes
		FROM platform.traeger_school_document_releases()`).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("organization tenancy postgres: list school releases: %w", err)
	}
	return sharedDocuments(rows), nil
}

// OpenSchoolDocument reads a document released for the school and records
// the read in the same statement.
func (s *TraegerDocumentStore) OpenSchoolDocument(ctx context.Context, schoolID, accountID, documentID int64) (organizationtenancy.TraegerDocumentFile, bool, error) {
	db, err := schoolDB(ctx, schoolID)
	if err != nil {
		return organizationtenancy.TraegerDocumentFile{}, false, err
	}
	var rows []traegerFileRow
	err = db.NewRaw(`SELECT file_name, content_type, content FROM platform.open_traeger_school_document(?, ?)`,
		documentID, traegerActor(accountID)).Scan(ctx, &rows)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return organizationtenancy.TraegerDocumentFile{}, false, fmt.Errorf("organization tenancy postgres: open school document: %w", err)
	}
	if len(rows) == 0 {
		return organizationtenancy.TraegerDocumentFile{}, false, nil
	}
	return rows[0].toDomain(), true, nil
}

// sharedDocuments folds the covering releases into one entry per document,
// the newest release first.
func sharedDocuments(rows []sharedReleaseRow) []organizationtenancy.SharedDocument {
	index := map[int64]int{}
	documents := []organizationtenancy.SharedDocument{}
	for _, row := range rows {
		position, seen := index[row.DocumentID]
		if !seen {
			position = len(documents)
			index[row.DocumentID] = position
			documents = append(documents, organizationtenancy.SharedDocument{
				ID: row.DocumentID, Title: row.Title, FileName: row.FileName, ContentType: row.ContentType, SizeBytes: row.SizeBytes,
			})
		}
		document := &documents[position]
		document.Releases = append(document.Releases, row.toDomain())
		if row.ReleasedAt.After(document.ReleasedAt) {
			document.ReleasedAt = row.ReleasedAt
		}
	}
	for i := range documents {
		slices.SortFunc(documents[i].Releases, func(a, b organizationtenancy.TraegerDocumentRelease) int { return int(a.ID - b.ID) })
	}
	slices.SortStableFunc(documents, func(a, b organizationtenancy.SharedDocument) int {
		if order := b.ReleasedAt.Compare(a.ReleasedAt); order != 0 {
			return order
		}
		return int(b.ID - a.ID)
	})
	return documents
}
