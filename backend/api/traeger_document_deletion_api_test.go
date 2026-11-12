// Router-level tests for deleting a shared document of a Träger (#5993): the
// deletion revokes every release at once, ends every list and every file
// link, removes the file and keeps the audit; it needs the right to manage,
// stays within the Träger, changes nothing when it fails, and never lets a
// release committing at the same time survive it.
package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/auth/authorize/permissions"
	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	organizationtenancyCompose "github.com/moto-ogs/project-phoenix/modules/organizationtenancy/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

func (f traegerDocumentsFixture) deleteDocument(t *testing.T, token string, document int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.router, token, http.MethodDelete, fmt.Sprintf("/traeger/documents/%d", document), nil)
}

// traegerDocumentTrail is what the database keeps of one document: the
// file, the deletion and every piece of audit that names the document.
type traegerDocumentTrail struct {
	HasFile        bool       `bun:"has_file"`
	DeletedAt      *time.Time `bun:"deleted_at"`
	DeletedByActor bool       `bun:"deleted_by_actor"`
	Releases       int        `bun:"releases"`
	ActiveReleases int        `bun:"active_releases"`
	RevokedByActor int        `bun:"revoked_by_actor"`
	Reads          int        `bun:"reads"`
	Downloads      int        `bun:"downloads"`
}

func (f traegerDocumentsFixture) trail(t *testing.T, document int64) traegerDocumentTrail {
	t.Helper()
	var trail traegerDocumentTrail
	require.NoError(t, f.db.NewRaw(`
		SELECT document.content IS NOT NULL AS has_file, document.deleted_at,
			document.deleted_by IS NOT NULL AND document.deleted_by = document.created_by AS deleted_by_actor,
			(SELECT COUNT(*) FROM platform.traeger_document_releases AS r WHERE r.document_id = document.id) AS releases,
			(SELECT COUNT(*) FROM platform.traeger_document_releases AS r WHERE r.document_id = document.id AND r.revoked_at IS NULL) AS active_releases,
			(SELECT COUNT(*) FROM platform.traeger_document_releases AS r WHERE r.document_id = document.id AND r.revoked_by = document.created_by) AS revoked_by_actor,
			(SELECT COUNT(*) FROM platform.traeger_document_reads AS r WHERE r.document_id = document.id) AS reads,
			(SELECT COUNT(*) FROM audit.traeger_document_downloads AS d WHERE d.document_id = document.id) AS downloads
		FROM platform.traeger_documents AS document WHERE document.id = ?`, document).Scan(t.Context(), &trail))
	return trail
}

// TestDeletingATraegerDocumentEndsEveryReleaseAndKeepsTheAudit pins #5993:
// deleting a document with several releases revokes all of them with the
// acting account; the Träger, the schools and the parents no longer list it
// and no existing file link opens it, the Träger-Büro's own download
// included; the file is gone from the database; releases, reads, downloads
// and the access log of the schools stay; a second deletion and a new
// release answer 404.
func TestDeletingATraegerDocumentEndsEveryReleaseAndKeepsTheAudit(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	nord, sued := f.office.schools[0], f.office.schools[1]
	sonnen := f.group(t, nord, "Sonnenblumen")
	document := f.uploaded(t, f.office.token, "Hausordnung")
	kept := f.uploaded(t, f.office.token, "Elternbrief")
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools", "school_id": nord})
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents", "school_id": nord, "group_id": sonnen})
	f.released(t, f.office.token, kept.ID, map[string]any{"audience": "schools"})

	nordLead, suedLead := f.staff(t, nord, permissions.FilesManage), f.staff(t, sued, permissions.FilesManage)
	link := fmt.Sprintf("/%d/file", document.ID)
	require.Equal(t, http.StatusOK, f.schoolGet(t, nordLead, link).Code)
	f.download(t, f.office.token, document.ID)
	shared := organizationtenancyCompose.NewSharedTraegerDocuments()
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	parentAccount := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("eltern-loeschen-%d@example.org", testpkg.UniqueTestTenantID(t)))
	parent := organizationtenancy.TraegerDocumentParent{AccountID: parentAccount.ID, Schools: map[int64][]int64{nord: {sonnen}}}
	parentOpens := func() ([]organizationtenancy.SharedDocument, error) {
		var listed []organizationtenancy.SharedDocument
		var openErr error
		require.NoError(t, tenant.WithinTraeger(unit, f.office.organization, func(ctx context.Context) error {
			var err error
			if listed, err = shared.ParentDocuments(ctx, f.office.organization, parent); err != nil {
				return err
			}
			_, openErr = shared.OpenParentDocument(ctx, f.office.organization, document.ID, parent)
			return nil
		}))
		return listed, openErr
	}
	listed, err := parentOpens()
	require.NoError(t, err, "the parent opens the document before the deletion")
	require.Len(t, listed, 1)
	_, accessLogBefore := f.downloadEntries(t, f.office.organization, document.ID)
	before := f.trail(t, document.ID)
	require.Equal(t, 3, before.ActiveReleases)
	require.Equal(t, 2, before.Reads, "the school and the parent opened it")
	require.Equal(t, 1, before.Downloads)

	rec := f.deleteDocument(t, f.office.token, document.ID)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	after := f.trail(t, document.ID)
	assert.False(t, after.HasFile, "the file is removed from the database")
	assert.NotNil(t, after.DeletedAt, "the deletion is recorded with its time")
	assert.True(t, after.DeletedByActor, "the deletion is recorded with the acting account")
	assert.Zero(t, after.ActiveReleases, "the deletion revokes every release")
	assert.Equal(t, 3, after.RevokedByActor, "each release is revoked by the acting account")
	assert.Equal(t, before.Releases, after.Releases, "the releases stay as the record of the sharing")
	assert.Equal(t, before.Reads, after.Reads, "the reads stay")
	assert.Equal(t, before.Downloads, after.Downloads, "the download ledger stays")
	_, accessLogAfter := f.downloadEntries(t, f.office.organization, document.ID)
	assert.Equal(t, accessLogBefore, accessLogAfter, "the schools' access log stays")

	assert.Equal(t, []int64{kept.ID}, documentIDs(f.documents(t, f.office.token)), "the Träger lists only the other document")
	assert.Equal(t, []int64{kept.ID}, documentIDs(f.schoolDocuments(t, nordLead)))
	assert.Equal(t, []int64{kept.ID}, documentIDs(f.schoolDocuments(t, suedLead)))
	rec = f.schoolGet(t, nordLead, link)
	assert.Equal(t, http.StatusNotFound, rec.Code, "an existing link of the school opens nothing")
	assert.Equal(t, "traeger.shared_document_not_found", refusalOf(t, rec).Code)
	rec = traegerJSON(t, f.router, f.office.token, http.MethodGet, fmt.Sprintf("/traeger/documents/%d/file", document.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "the Träger-Büro downloads the deleted file no more")
	assert.Equal(t, "traeger.document_not_found", refusalOf(t, rec).Code)
	listed, err = parentOpens()
	assert.Empty(t, listed, "the parent lists nothing")
	assert.ErrorIs(t, err, organizationtenancy.ErrTraegerDocumentNotFound, "an existing link of the parent opens nothing")

	rec = f.deleteDocument(t, f.office.token, document.ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a document is deleted once")
	assert.Equal(t, "traeger.document_not_found", refusalOf(t, rec).Code)
	rec = f.release(t, f.office.token, document.ID, map[string]any{"audience": "parents"})
	assert.Equal(t, http.StatusNotFound, rec.Code, "a deleted document takes no new release")
	assert.Zero(t, f.trail(t, document.ID).ActiveReleases)
	assert.Equal(t, 1, f.trail(t, kept.ID).ActiveReleases, "the other document keeps its release")
}

// TestDeletingATraegerDocumentNeedsTheRightAndStaysWithinTheTraeger pins the
// boundaries of #5993: a Träger role without "Dokumente verwalten" gets 403,
// another Träger 404, and neither changes the document.
func TestDeletingATraegerDocumentNeedsTheRightAndStaysWithinTheTraeger(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Elternbrief")
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})

	readOnly := f.member(t, f.office, identityaccess.TraegerRoleReadOnly)
	assert.Equal(t, http.StatusForbidden, f.deleteDocument(t, readOnly, document.ID).Code)
	other := f.newOffice(t, "Träger Fremd Löschen", identityaccess.TraegerRoleDocuments)
	rec := f.deleteDocument(t, other.token, document.ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "another Träger reaches no document of this one")
	assert.Equal(t, "traeger.document_not_found", refusalOf(t, rec).Code)

	trail := f.trail(t, document.ID)
	assert.True(t, trail.HasFile)
	assert.Nil(t, trail.DeletedAt)
	assert.Equal(t, 1, trail.ActiveReleases, "the refused deletions changed nothing")
}

// failingTraegerRuntime runs the Träger transaction and fails it after the
// operation succeeded, as a lost connection before the commit would.
type failingTraegerRuntime struct{ traegerDocumentRuntime }

var errTraegerCommitLost = errors.New("the commit was lost")

func (r failingTraegerRuntime) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	return r.traegerDocumentRuntime.WithinTraeger(ctx, organizationID, func(txCtx context.Context) error {
		if err := fn(txCtx); err != nil {
			return err
		}
		return errTraegerCommitLost
	})
}

// TestAFailedTraegerDocumentDeletionChangesNothing pins the atomicity of
// #5993: the revocations, the removed file and the deletion mark sit in one
// Träger transaction, so a deletion that fails keeps the file and every
// release.
func TestAFailedTraegerDocumentDeletionChangesNothing(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Hausordnung")
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents"})

	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, f.db))
	require.NoError(t, err)
	documents := organizationtenancyCompose.NewTraegerDocuments(organizationtenancy.TraegerDocumentPorts{
		Schools: traegerDocumentSchools{organizations: organizations}, Groups: traegerDocumentGroups{},
		Runtime: failingTraegerRuntime{}, Access: traegerDocumentAccessLog{}, Downloads: traegerDocumentDownloadLedger{},
		Notices: owners.TraegerDocumentReleaseNotices{},
	})
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	err = documents.DeleteDocument(unit, f.office.organization, 0, document.ID)
	require.ErrorIs(t, err, errTraegerCommitLost)

	trail := f.trail(t, document.ID)
	assert.True(t, trail.HasFile, "the file stays")
	assert.Nil(t, trail.DeletedAt)
	assert.Equal(t, 2, trail.ActiveReleases, "every release stays")
	assert.Equal(t, []int64{document.ID}, documentIDs(f.documents(t, f.office.token)))
}

// TestADeletedTraegerDocumentKeepsItsReadsForTheirRetention pins the
// retention of #5993 with a clock in the past: after the deletion the
// hourly cleanup still deletes the reads older than three years and keeps
// the younger ones; the deletion invents no period of its own and keeps the
// deleted document's row as their referent.
func TestADeletedTraegerDocumentKeepsItsReadsForTheirRetention(t *testing.T) {
	t.Parallel()
	// The retention sweeps the reads of every Träger; its own database keeps
	// it from deleting another test's reads and theirs from deleting its own.
	f := newTraegerDocumentsFixtureOn(t, testpkg.SetupIsolatedTestDB(t))
	ctx := context.Background()
	document := f.uploaded(t, f.office.token, "Hausordnung")
	_, err := f.db.ExecContext(ctx, `INSERT INTO platform.traeger_document_reads (organization_id, document_id, reader, read_at)
		VALUES (?0, ?1, 'school', NOW() - INTERVAL '3 years 1 day'), (?0, ?1, 'parent', NOW() - INTERVAL '2 years 11 months')`,
		f.office.organization, document.ID)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, f.deleteDocument(t, f.office.token, document.ID).Code)
	require.Equal(t, 2, f.trail(t, document.ID).Reads, "the deletion keeps the reads")

	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db))
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		_, err := organizationtenancyCompose.NewSharedTraegerDocuments().DeleteExpiredTraegerDocumentReads(adminCtx)
		return err
	}))
	trail := f.trail(t, document.ID)
	assert.Equal(t, 1, trail.Reads, "the expired read goes, the younger one stays")
	assert.NotNil(t, trail.DeletedAt, "the deleted document's row stays as their referent")
}

// waitUntilBlocked waits until a backend waits for a lock in a statement
// containing marker, or until done delivers, whichever comes first; it
// reports whether the backend blocked.
func waitUntilBlocked[T any](t *testing.T, f traegerDocumentsFixture, marker string, done <-chan T) (T, bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case result := <-done:
			return result, false
		default:
		}
		var waiting int
		require.NoError(t, f.db.NewRaw(`SELECT COUNT(*) FROM pg_stat_activity
			WHERE wait_event_type = 'Lock' AND query LIKE ?`, "%"+marker+"%").Scan(t.Context(), &waiting))
		if waiting > 0 {
			var zero T
			return zero, true
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("the second transaction neither finished nor waited for a lock")
	var zero T
	return zero, false
}

// TestATraegerDocumentReleaseNeverSurvivesAConcurrentDeletion pins the lock of
// #5993 with two real transactions in parallel, in both orders: a release
// that starts while a deletion holds the document waits for it and is then
// refused; a deletion that starts while a release is still open waits for it
// and then revokes it. Without the document's row lock either order leaves
// an active release of a deleted document.
func TestATraegerDocumentReleaseNeverSurvivesAConcurrentDeletion(t *testing.T) {
	t.Parallel()
	store := organizationtenancyCompose.NewSharedTraegerDocuments()
	release := func(ctx context.Context, organization, document int64) error {
		_, err := store.InsertRelease(ctx, organization, 0, document,
			organizationtenancy.TraegerDocumentReleaseInput{Audience: organizationtenancy.TraegerDocumentForParents}, "")
		return err
	}
	deleteDocument := func(ctx context.Context, organization, document int64) error {
		deleted, err := store.DeleteDocument(ctx, organization, 0, document)
		if err == nil && !deleted {
			err = organizationtenancy.ErrTraegerDocumentNotFound
		}
		return err
	}
	// race holds first open in its Träger transaction while second runs in
	// another one, then commits first and returns second's result.
	race := func(t *testing.T, f traegerDocumentsFixture, document int64, marker string, first, second func(context.Context, int64, int64) error) error {
		unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
		held, commit, firstDone := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		go func() {
			firstDone <- tenant.WithinTraeger(unit, f.office.organization, func(ctx context.Context) error {
				if err := first(ctx, f.office.organization, document); err != nil {
					close(held)
					return err
				}
				close(held)
				<-commit
				return nil
			})
		}()
		<-held
		secondDone := make(chan error, 1)
		go func() {
			secondDone <- tenant.WithinTraeger(unit, f.office.organization, func(ctx context.Context) error {
				return second(ctx, f.office.organization, document)
			})
		}()
		early, blocked := waitUntilBlocked(t, f, marker, secondDone)
		close(commit)
		require.NoError(t, <-firstDone)
		if !blocked {
			return early
		}
		return <-secondDone
	}

	t.Run("a release during a deletion is refused", func(t *testing.T) {
		t.Parallel()
		f := newTraegerDocumentsFixture(t)
		document := f.uploaded(t, f.office.token, "Hausordnung")
		err := race(t, f, document.ID, "platform.traeger_document", deleteDocument, release)
		assert.ErrorIs(t, err, organizationtenancy.ErrTraegerDocumentNotFound, "the release finds the document deleted")
		assert.Zero(t, f.trail(t, document.ID).ActiveReleases, "no active release of a deleted document")
	})
	t.Run("an opening by a school during a deletion finds nothing", func(t *testing.T) {
		t.Parallel()
		f := newTraegerDocumentsFixture(t)
		school := f.office.schools[0]
		document := f.uploaded(t, f.office.token, "Hausordnung")
		f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools", "school_id": school})
		account := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("ogs-oeffnen-%d@example.org", testpkg.UniqueTestTenantID(t)))
		schoolID, err := tenant.NewTenantID(school)
		require.NoError(t, err)
		unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
		held, commit, deleted := make(chan struct{}), make(chan struct{}), make(chan error, 1)
		go func() {
			deleted <- tenant.WithinTraeger(unit, f.office.organization, func(ctx context.Context) error {
				err := deleteDocument(ctx, f.office.organization, document.ID)
				close(held)
				if err != nil {
					return err
				}
				<-commit
				return nil
			})
		}()
		<-held
		type opening struct {
			file organizationtenancy.TraegerDocumentFile
			err  error
		}
		opened := make(chan opening, 1)
		go func() {
			var result opening
			result.err = tenant.WithinTenant(unit, schoolID, func(ctx context.Context) error {
				var openErr error
				result.file, openErr = store.OpenSchoolDocument(ctx, school, account.ID, document.ID)
				return openErr
			})
			opened <- result
		}()
		early, blocked := waitUntilBlocked(t, f, "open_traeger_school_document", opened)
		close(commit)
		require.NoError(t, <-deleted)
		result := early
		if blocked {
			result = <-opened
		}
		assert.True(t, blocked, "the opening waits for the deletion")
		assert.ErrorIs(t, result.err, organizationtenancy.ErrTraegerDocumentNotFound, "after the deletion there is nothing to open")
		assert.Empty(t, result.file.Data, "never an empty or stale file")
	})
	t.Run("a deletion during a release revokes it", func(t *testing.T) {
		t.Parallel()
		f := newTraegerDocumentsFixture(t)
		document := f.uploaded(t, f.office.token, "Hausordnung")
		err := race(t, f, document.ID, "platform.traeger_document", release, deleteDocument)
		require.NoError(t, err)
		trail := f.trail(t, document.ID)
		assert.NotNil(t, trail.DeletedAt)
		assert.Equal(t, 1, trail.Releases)
		assert.Zero(t, trail.ActiveReleases, "the deletion revokes the release committed meanwhile")
	})
}

// TestADeletedTraegerDocumentReachesNoParentThroughAStaleRelease pins the
// second guard of #5993 for the parents: even if an active release still
// named a deleted document, the parents' list and link pass over it.
func TestADeletedTraegerDocumentReachesNoParentThroughAStaleRelease(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	school := f.office.schools[0]
	document := f.uploaded(t, f.office.token, "Elternbrief")
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents"})
	_, err := f.db.ExecContext(t.Context(), `UPDATE platform.traeger_documents SET content = NULL, deleted_at = NOW() WHERE id = ?`, document.ID)
	require.NoError(t, err, "a deletion whose revocation is missing")
	require.Equal(t, 1, f.trail(t, document.ID).ActiveReleases)

	shared := organizationtenancyCompose.NewSharedTraegerDocuments()
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	account := testpkg.CreateTestAccount(t, f.db, fmt.Sprintf("eltern-veraltet-%d@example.org", testpkg.UniqueTestTenantID(t)))
	parent := organizationtenancy.TraegerDocumentParent{AccountID: account.ID, Schools: map[int64][]int64{school: nil}}
	require.NoError(t, tenant.WithinTraeger(unit, f.office.organization, func(ctx context.Context) error {
		listed, err := shared.ParentDocuments(ctx, f.office.organization, parent)
		require.NoError(t, err)
		assert.Empty(t, listed, "the parent lists no deleted document")
		_, err = shared.OpenParentDocument(ctx, f.office.organization, document.ID, parent)
		assert.ErrorIs(t, err, organizationtenancy.ErrTraegerDocumentNotFound, "the parent's link opens nothing")
		return nil
	}))
}

// TestDeletingAnotherTraegersDocumentFindsNothingBeneathRLS pins the Träger
// filter of the deletion in the store, beneath RLS (#5993 review, as in
// #4679): Träger B deletes a document of Träger A in its own Träger
// transaction, running as the owner of the tables, and finds nothing; A's
// document keeps its file, its release and no deletion mark. RLS alone would
// hide a missing filter, so this goes red when the store locks, revokes or
// deletes without the organization.
func TestDeletingAnotherTraegersDocumentFindsNothingBeneathRLS(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Elternbrief")
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})
	other := f.newOffice(t, "Träger Fremd unter RLS", identityaccess.TraegerRoleDocuments)

	store := organizationtenancyCompose.NewSharedTraegerDocuments()
	unit := tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, f.db))
	var deleted bool
	require.NoError(t, testpkg.WithinTraeger(unit, other.organization, func(txCtx context.Context) error {
		// As the owner of the tables, beneath RLS: only the store's own
		// Träger filter keeps A's document out.
		require.NoError(t, testpkg.WithoutRequestRole(txCtx))
		var err error
		deleted, err = store.DeleteDocument(txCtx, other.organization, 0, document.ID)
		return err
	}))
	assert.False(t, deleted, "another Träger's document is not found")

	trail := f.trail(t, document.ID)
	assert.True(t, trail.HasFile, "A's file stays")
	assert.Nil(t, trail.DeletedAt, "A's document carries no deletion mark")
	assert.Equal(t, 1, trail.ActiveReleases, "A's release stays active")
	assert.Zero(t, trail.RevokedByActor, "no release of A is revoked")
}
