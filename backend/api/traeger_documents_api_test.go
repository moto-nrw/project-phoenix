// Router-level tests for the documents a Träger shares with parents and
// schools (#4692): the Verwaltung uploads a file and releases it for the
// parents of all schools, of a school or of a group, or for the schools; a
// release is revoked, which ends the access at once. A school's OGS-Leitung
// reads only what a release covers for its school; a role without the right
// gets 403; no Träger reaches the documents of another; every opening by a
// school is recorded.
package api

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	organizationtenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	querybudgets "github.com/moto-nrw/project-phoenix/test/query-budgets"
)

// samplePDF is the smallest file the content check takes for a PDF.
var samplePDF = []byte("%PDF-1.4\n1 0 obj << >> endobj\ntrailer << >>\n%%EOF\n")

type traegerDocumentBody struct {
	ID       int64                        `json:"id"`
	Title    string                       `json:"title"`
	FileName string                       `json:"file_name"`
	Reads    int                          `json:"reads"`
	Releases []traegerDocumentReleaseBody `json:"releases"`
}

type traegerDocumentReleaseBody struct {
	ID        int64  `json:"id"`
	Audience  string `json:"audience"`
	SchoolID  *int64 `json:"school_id"`
	GroupID   *int64 `json:"group_id"`
	GroupName string `json:"group_name"`
}

type traegerDocumentSchoolBody struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Groups []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"groups"`
}

// traegerDocumentsFixture is the Träger portal for the sign-in, the routes
// of the shared documents as the serving root mounts them, and an office of
// the Verwaltung with two schools.
type traegerDocumentsFixture struct {
	db     *bun.DB
	portal *traegerPortalHandler
	router http.Handler
	office traegerOffice
}

func newTraegerDocumentsFixture(t *testing.T) traegerDocumentsFixture {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	unit := testpkg.TenantRuntime(t, db)
	portal, _ := traegerPortalOn(t, db, unit)
	organizations, err := owners.NewOrganizationTenancy(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r.WithContext(tenant.WithUnitOfWork(r.Context(), unit)))
		})
	})
	tenantRouter := chi.NewRouter()
	require.NoError(t, mountTraegerDocumentRoutes(router, tenantRouter, testpkg.NewDatabase(t, db), organizations))
	router.Mount("/api", tenantRouter)
	f := traegerDocumentsFixture{db: db, portal: portal, router: testpkg.SessionVerifier(router)}
	f.office = f.newOffice(t, "Träger Dokumente", identityaccess.TraegerRoleAdministration)
	return f
}

func (f traegerDocumentsFixture) newOffice(t *testing.T, name, role string) traegerOffice {
	t.Helper()
	office := newTraegerOffice(t, f.db, f.portal, name, role)
	testpkg.OwnTraegerDocuments(t, f.db, office.organization)
	return office
}

// member signs another account of the office's Träger in with role.
func (f traegerDocumentsFixture) member(t *testing.T, office traegerOffice, role string) string {
	t.Helper()
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, office.organization, account.ID, role)
	code, answer := traegerSignIn(t, f.portal, account.Email, password, office.organization)
	require.Equal(t, http.StatusOK, code)
	return answer.AccessToken
}

// group adds a group to a school of the Träger.
func (f traegerDocumentsFixture) group(t *testing.T, schoolID int64, name string) int64 {
	t.Helper()
	var id int64
	require.NoError(t, f.db.NewRaw(`INSERT INTO education.groups (tenant_id, name) VALUES (?, ?) RETURNING id`, schoolID, name).Scan(t.Context(), &id))
	return id
}

// staff signs a staff member of the school in with the given permissions.
func (f traegerDocumentsFixture) staff(t *testing.T, schoolID int64, perms ...string) string {
	t.Helper()
	_, account := testpkg.CreateTestStaffWithAccountForTenant(t, f.db, schoolID, "Olga", "Leitung")
	return testpkg.MintTestJWT(t, sessiontoken.AppClaims{
		ID: int(account.ID), Sub: account.Email, Username: "ogs-leitung", FirstName: "Olga", LastName: "Leitung",
		Roles: []string{"admin"}, TenantID: schoolID, Permissions: perms,
	})
}

func (f traegerDocumentsFixture) upload(t *testing.T, token, title, fileName string, data []byte) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("title", title))
	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = part.Write(data)
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, "/traeger/documents/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f traegerDocumentsFixture) uploaded(t *testing.T, token, title string) traegerDocumentBody {
	t.Helper()
	rec := f.upload(t, token, title, "hausordnung.pdf", samplePDF)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerDocumentBody](t, rec.Body.Bytes())
}

func (f traegerDocumentsFixture) release(t *testing.T, token string, document int64, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.router, token, http.MethodPost, fmt.Sprintf("/traeger/documents/%d/releases", document), body)
}

func (f traegerDocumentsFixture) released(t *testing.T, token string, document int64, body map[string]any) traegerDocumentReleaseBody {
	t.Helper()
	rec := f.release(t, token, document, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[traegerDocumentReleaseBody](t, rec.Body.Bytes())
}

func (f traegerDocumentsFixture) revoke(t *testing.T, token string, document, release int64) *httptest.ResponseRecorder {
	t.Helper()
	return traegerJSON(t, f.router, token, http.MethodDelete, fmt.Sprintf("/traeger/documents/%d/releases/%d", document, release), nil)
}

func (f traegerDocumentsFixture) documents(t *testing.T, token string) []traegerDocumentBody {
	t.Helper()
	rec := traegerJSON(t, f.router, token, http.MethodGet, "/traeger/documents/", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]traegerDocumentBody](t, rec.Body.Bytes())
}

func (f traegerDocumentsFixture) schoolGet(t *testing.T, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/traeger-documents"+path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	return rec
}

func (f traegerDocumentsFixture) schoolDocuments(t *testing.T, token string) []traegerDocumentBody {
	t.Helper()
	rec := f.schoolGet(t, token, "/")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[[]traegerDocumentBody](t, rec.Body.Bytes())
}

func documentIDs(documents []traegerDocumentBody) []int64 {
	ids := make([]int64, 0, len(documents))
	for _, document := range documents {
		ids = append(ids, document.ID)
	}
	return ids
}

// TestTraegerDocumentsReachOnlyTheReleasedSchool pins the flow of #4692: a
// released document reaches the OGS-Leitung of the released school and no
// other school; a release for the parents of a group shows at that group's
// school too; revoking every release that covers a school ends its access
// at once; a school's opening is recorded.
func TestTraegerDocumentsReachOnlyTheReleasedSchool(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	nord, sued := f.office.schools[0], f.office.schools[1]
	sonnen := f.group(t, nord, "Sonnenblumen")
	mond := f.group(t, sued, "Mondgruppe")

	document := f.uploaded(t, f.office.token, "Hausordnung 2026/27")
	assert.Equal(t, "Hausordnung 2026/27", document.Title)
	assert.Equal(t, "hausordnung.pdf", document.FileName)

	rec := traegerJSON(t, f.router, f.office.token, http.MethodGet, "/traeger/documents/audiences", nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	audiences := decodeSchoolPortalData[[]traegerDocumentSchoolBody](t, rec.Body.Bytes())
	require.Len(t, audiences, 2)
	for _, school := range audiences {
		require.Len(t, school.Groups, 1, "every school offers its own groups")
		if school.ID == nord {
			assert.Equal(t, sonnen, school.Groups[0].ID)
		}
	}

	forNord := f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools", "school_id": nord})
	rec = f.release(t, f.office.token, document.ID, map[string]any{"audience": "schools", "school_id": nord})
	assert.Equal(t, http.StatusConflict, rec.Code, "one active release per audience")
	assert.Equal(t, "traeger.document_already_released", refusalOf(t, rec).Code)
	forGroup := f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents", "school_id": nord, "group_id": sonnen})
	assert.Equal(t, sonnen, *forGroup.GroupID)
	assert.Contains(t, forGroup.GroupName, "Sonnenblumen", "the release keeps the group's name")

	rec = f.release(t, f.office.token, document.ID, map[string]any{"audience": "parents", "school_id": nord, "group_id": mond})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a group of another school is refused")
	assert.Equal(t, "traeger.document_audience_invalid", refusalOf(t, rec).Code)
	rec = f.release(t, f.office.token, document.ID, map[string]any{"audience": "schools", "group_id": sonnen})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a school release names no group")

	nordLead, suedLead := f.staff(t, nord, permissions.FilesManage), f.staff(t, sued, permissions.FilesManage)
	seen := f.schoolDocuments(t, nordLead)
	require.Equal(t, []int64{document.ID}, documentIDs(seen))
	assert.Len(t, seen[0].Releases, 2, "the school sees whom the Träger released it for")
	assert.Empty(t, f.schoolDocuments(t, suedLead), "another school of the Träger sees nothing")

	path := fmt.Sprintf("/%d/file", document.ID)
	rec = f.schoolGet(t, nordLead, path)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Equal(t, samplePDF, rec.Body.Bytes())
	assert.Equal(t, "application/pdf", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Header().Get("Content-Disposition"), "attachment")
	assert.Equal(t, "private, no-store", rec.Header().Get("Cache-Control"))
	rec = f.schoolGet(t, suedLead, path)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "traeger.shared_document_not_found", refusalOf(t, rec).Code)
	assert.Equal(t, 1, f.documents(t, f.office.token)[0].Reads, "the school's opening is recorded, the refused one is not")

	require.Equal(t, http.StatusOK, f.revoke(t, f.office.token, document.ID, forNord.ID).Code)
	assert.Equal(t, []int64{document.ID}, documentIDs(f.schoolDocuments(t, nordLead)), "the group release still covers the school")
	require.Equal(t, http.StatusOK, f.revoke(t, f.office.token, document.ID, forGroup.ID).Code)
	assert.Empty(t, f.schoolDocuments(t, nordLead), "revoking ends the access at once")
	assert.Equal(t, http.StatusNotFound, f.schoolGet(t, nordLead, path).Code)
	rec = f.revoke(t, f.office.token, document.ID, forGroup.ID)
	assert.Equal(t, http.StatusNotFound, rec.Code, "a release is revoked once")
	assert.Equal(t, "traeger.document_release_not_found", refusalOf(t, rec).Code)
	assert.Empty(t, f.documents(t, f.office.token)[0].Releases)

	// A release for the parents of all schools shows at every school.
	f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents"})
	assert.Len(t, f.schoolDocuments(t, suedLead), 1)
}

// TestTraegerDocumentsNeedTheirRights pins the rights of #4692: a Träger role
// without "Anmeldung verwalten" reads but neither uploads, releases nor
// revokes; school staff without the right to manage the school's files gets
// 403; another Träger reaches no document, release or file of this one.
func TestTraegerDocumentsNeedTheirRights(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Elternbrief")
	release := f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})

	readOnly := f.member(t, f.office, identityaccess.TraegerRoleReadOnly)
	assert.Equal(t, []int64{document.ID}, documentIDs(f.documents(t, readOnly)), "reading needs only the overview")
	assert.Equal(t, http.StatusForbidden, f.upload(t, readOnly, "Fremd", "brief.pdf", samplePDF).Code)
	assert.Equal(t, http.StatusForbidden, f.release(t, readOnly, document.ID, map[string]any{"audience": "parents"}).Code)
	assert.Equal(t, http.StatusForbidden, f.revoke(t, readOnly, document.ID, release.ID).Code)
	assert.Equal(t, http.StatusForbidden, traegerJSON(t, f.router, readOnly, http.MethodGet, "/traeger/documents/audiences", nil).Code)

	teacher := f.staff(t, f.office.schools[0], permissions.GroupsRead)
	assert.Equal(t, http.StatusForbidden, f.schoolGet(t, teacher, "/").Code, "the school's documents need files:manage")
	assert.Equal(t, http.StatusForbidden, f.schoolGet(t, teacher, fmt.Sprintf("/%d/file", document.ID)).Code)

	other := f.newOffice(t, "Träger Fremd", identityaccess.TraegerRoleAdministration)
	assert.Empty(t, f.documents(t, other.token), "another Träger lists nothing of this one")
	rec := traegerJSON(t, f.router, other.token, http.MethodGet, fmt.Sprintf("/traeger/documents/%d/file", document.ID), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "traeger.document_not_found", refusalOf(t, rec).Code)
	assert.Equal(t, http.StatusNotFound, f.release(t, other.token, document.ID, map[string]any{"audience": "parents"}).Code)
	assert.Equal(t, http.StatusNotFound, f.revoke(t, other.token, document.ID, release.ID).Code)
	rec = f.release(t, f.office.token, document.ID, map[string]any{"audience": "schools", "school_id": other.schools[0]})
	assert.Equal(t, http.StatusBadRequest, rec.Code, "a school of another Träger is refused")
	foreignLead := f.staff(t, other.schools[0], permissions.FilesManage)
	assert.Empty(t, f.schoolDocuments(t, foreignLead), "a school of another Träger sees nothing")
	assert.Len(t, f.documents(t, f.office.token)[0].Releases, 1, "the refused writes changed nothing")
}

// TestTraegerDocumentUploadChecksTheFile pins the upload check: the title is
// required, the file's content must match its admitted type, and an Office
// file is accepted only as the archive it claims to be.
func TestTraegerDocumentUploadChecksTheFile(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	rec := f.upload(t, f.office.token, "Skript", "liste.html", []byte("<html><script>alert(1)</script></html>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "files.file_type_not_allowed", refusalOf(t, rec).Code)
	rec = f.upload(t, f.office.token, "Getarnt", "liste.pdf", []byte("<html><script>alert(1)</script></html>"))
	assert.Equal(t, http.StatusBadRequest, rec.Code, "the name alone never decides")
	rec = f.upload(t, f.office.token, "Getarnt", "liste.xlsx", []byte("PK\x03\x04not an archive"))
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	rec = f.upload(t, f.office.token, "Endung", "file."+strings.Repeat("a", 256), samplePDF)
	assert.Equal(t, http.StatusBadRequest, rec.Code, "an overlong extension is refused, never a crash")
	assert.Equal(t, "files.file_type_not_allowed", refusalOf(t, rec).Code)
	rec = f.upload(t, f.office.token, "  ", "brief.pdf", samplePDF)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Equal(t, "traeger.document_invalid", refusalOf(t, rec).Code)
	assert.Empty(t, f.documents(t, f.office.token), "nothing refused is stored")
}

// TestTraegerDocumentListsReadFlat pins the query budgets of the Träger's and
// the school's lists: their statements do not grow with the documents.
func TestTraegerDocumentListsReadFlat(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	lead := f.staff(t, f.office.schools[0], permissions.FilesManage)
	read := func(token, path string) []string {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.router.ServeHTTP(rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Selects("")
	}
	add := func() {
		document := f.uploaded(t, f.office.token, "Liste")
		f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})
		f.released(t, f.office.token, document.ID, map[string]any{"audience": "parents"})
	}
	add()
	traeger, school := read(f.office.token, "/traeger/documents/"), read(lead, "/api/traeger-documents/")
	add()
	add()
	assert.Len(t, read(f.office.token, "/traeger/documents/"), len(traeger))
	assert.Len(t, read(lead, "/api/traeger-documents/"), len(school))
	querybudgets.AssertQueryBudget(t, "api.traeger.documents", traeger)
	querybudgets.AssertQueryBudget(t, "api.school.traeger_documents", school)
}

// TestTraegerDocumentReadsExpireWithoutAnyOpening pins the retention of the
// reads (#4692 review): the hourly cleanup deletes every Träger's reads older
// than three years, even when nobody opens a document of that Träger any
// more, and keeps the younger ones. Outside the administrative transaction
// the deletion refuses.
func TestTraegerDocumentReadsExpireWithoutAnyOpening(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	ctx := context.Background()
	document := f.uploaded(t, f.office.token, "Hausordnung")
	_, err := f.db.ExecContext(ctx, `INSERT INTO platform.traeger_document_reads (organization_id, document_id, reader, read_at)
		VALUES (?0, ?1, 'school', NOW() - INTERVAL '3 years 1 day'), (?0, ?1, 'parent', NOW() - INTERVAL '2 years 11 months')`,
		f.office.organization, document.ID)
	require.NoError(t, err)
	reads := func() []string {
		var ages []string
		require.NoError(t, f.db.NewRaw(`SELECT CASE WHEN read_at < NOW() - INTERVAL '3 years' THEN 'expired' ELSE 'kept' END
			FROM platform.traeger_document_reads WHERE organization_id = ? ORDER BY read_at`, f.office.organization).Scan(ctx, &ages))
		return ages
	}
	require.Equal(t, []string{"expired", "kept"}, reads())

	retention := organizationtenancyCompose.NewSharedTraegerDocuments()
	unit := tenant.WithUnitOfWork(ctx, testpkg.TenantRuntime(t, f.db))
	require.Error(t, tenant.WithinTraeger(unit, f.office.organization, func(traegerCtx context.Context) error {
		_, err := retention.DeleteExpiredTraegerDocumentReads(traegerCtx)
		return err
	}), "only the administrative transaction deletes across Träger")
	require.NoError(t, tenant.WithinAdmin(unit, func(adminCtx context.Context) error {
		deleted, err := retention.DeleteExpiredTraegerDocumentReads(adminCtx)
		assert.GreaterOrEqual(t, deleted, 1)
		return err
	}))
	assert.Equal(t, []string{"kept"}, reads(), "the expired read goes, the younger one stays")
}
