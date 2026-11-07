// Router-level tests for the right of its own that #5992 gives the
// documents a Träger shares: "Dokumente verwalten" (traeger_documents:manage)
// opens every writing route, "Anmeldung verwalten" alone no longer does.
package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestTraegerDocumentWritesNeedTheDocumentsRight pins #5992: without
// "Dokumente verwalten" every writing document route answers 403, also for
// the Verwaltung with "Anmeldung verwalten", and the refused writes change
// nothing. The role Dokumente alone opens them.
func TestTraegerDocumentWritesNeedTheDocumentsRight(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	document := f.uploaded(t, f.office.token, "Elternbrief")
	release := f.released(t, f.office.token, document.ID, map[string]any{"audience": "schools"})

	administration := f.member(t, f.office, identityaccess.TraegerRoleAdministration)
	assert.Contains(t, traegerTokenClaims(t, administration)["permissions"], identityaccess.TraegerRightEnrollmentManage)
	assert.NotContains(t, traegerTokenClaims(t, administration)["permissions"], identityaccess.TraegerRightDocumentsManage)
	readOnly := f.member(t, f.office, identityaccess.TraegerRoleReadOnly)
	for name, token := range map[string]string{"Verwaltung": administration, "Nur lesen": readOnly} {
		assert.Equal(t, []int64{document.ID}, documentIDs(f.documents(t, token)), "%s reads the documents", name)
		writes := map[string]int{
			"upload":    f.upload(t, token, "Fremd", "brief.pdf", samplePDF).Code,
			"audiences": traegerJSON(t, f.router, token, http.MethodGet, "/traeger/documents/audiences", nil).Code,
			"release":   f.release(t, token, document.ID, map[string]any{"audience": "parents"}).Code,
			"revoke":    f.revoke(t, token, document.ID, release.ID).Code,
			"delete":    f.deleteDocument(t, token, document.ID).Code,
		}
		for route, code := range writes {
			assert.Equal(t, http.StatusForbidden, code, "%s without Dokumente verwalten: %s", name, route)
		}
	}
	listed := f.documents(t, f.office.token)
	require.Len(t, listed, 1, "the refused uploads and the refused deletion changed nothing")
	assert.Len(t, listed[0].Releases, 1, "the refused release and revocation changed nothing")

	documents := f.member(t, f.office, identityaccess.TraegerRoleDocuments)
	assert.NotContains(t, traegerTokenClaims(t, documents)["permissions"], identityaccess.TraegerRightEnrollmentManage,
		"the role Dokumente manages no Anmeldung")
	other := f.uploaded(t, documents, "Hausordnung")
	audiences := traegerJSON(t, f.router, documents, http.MethodGet, "/traeger/documents/audiences", nil)
	assert.Equal(t, http.StatusOK, audiences.Code, audiences.Body.String())
	revocable := f.released(t, documents, other.ID, map[string]any{"audience": "parents"})
	rec := f.revoke(t, documents, other.ID, revocable.ID)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	rec = f.deleteDocument(t, documents, other.ID)
	assert.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// TestTraegerDocumentsKeepTheMigratedVerwaltung pins the transition of
// #5992: a person of the Verwaltung holds Dokumente after the migration
// (1.15.799) and signs in with both rights, so she manages the Anmeldung and
// the documents as before.
func TestTraegerDocumentsKeepTheMigratedVerwaltung(t *testing.T) {
	t.Parallel()
	f := newTraegerDocumentsFixture(t)
	password := testpkg.NewTestPassword()
	account := traegerAccount(t, f.db, password)
	testpkg.CreateTestTraegerMember(t, f.db, f.office.organization, account.ID,
		identityaccess.TraegerRoleAdministration, identityaccess.TraegerRoleDocuments)
	code, answer := traegerSignIn(t, f.portal, account.Email, password, f.office.organization)
	require.Equal(t, http.StatusOK, code)

	rights := traegerTokenClaims(t, answer.AccessToken)["permissions"]
	assert.Contains(t, rights, identityaccess.TraegerRightEnrollmentManage)
	assert.Contains(t, rights, identityaccess.TraegerRightDocumentsManage)
	document := f.uploaded(t, answer.AccessToken, "Elternbrief")
	rec := f.release(t, answer.AccessToken, document.ID, map[string]any{"audience": "schools"})
	assert.Equal(t, http.StatusCreated, rec.Code, fmt.Sprintf("the migrated Verwaltung releases: %s", rec.Body.String()))
}
