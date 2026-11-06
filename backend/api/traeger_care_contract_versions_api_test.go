package api

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The detail lists every version of the contract, and each version has its
// PDF (#4681). The PDF of a version needs the right "Verträge verwalten" and
// stays with its Träger.
func TestCareContractVersionsAreListedAndEachHasItsPDFForTheTraegerOnly(t *testing.T) {
	t.Parallel()
	f := newCareContractFixture(t, "Träger Vertrag Versionen")
	require.Equal(t, http.StatusOK, traegerJSON(t, f.traeger, f.office.token, http.MethodPut, templatePath(f.school, f.offering), careContractTemplate).Code)
	rec := traegerJSON(t, f.traeger, f.office.token, http.MethodPost, "/traeger/contracts", map[string]any{"contracts": []any{f.key()}})
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	contract := decodeSchoolPortalData[careContractCreateBody](t, rec.Body.Bytes()).Created[0]

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(contract.ID, ""), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	detail := decodeSchoolPortalData[struct {
		Versions []struct {
			Number int    `json:"number"`
			Source string `json:"source"`
		} `json:"versions"`
	}](t, rec.Body.Bytes())
	require.Len(t, detail.Versions, 1)
	assert.Equal(t, 1, detail.Versions[0].Number)
	assert.Equal(t, "abschluss", detail.Versions[0].Source)

	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(contract.ID, "/versions/1/document"), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.True(t, bytes.HasPrefix(rec.Body.Bytes(), []byte("%PDF")))
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(contract.ID, "/versions/2/document"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, contractPath(contract.ID, "/versions/null/document"), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	billing := traegerColleague(t, f.reviewFixture, identityaccess.TraegerRoleBilling)
	rec = traegerJSON(t, f.traeger, billing, http.MethodGet, contractPath(contract.ID, "/versions/1/document"), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())

	other := newTraegerOffice(t, f.db, f.traeger, "Träger Vertrag Versionen Fremd", identityaccess.TraegerRoleAdministration)
	testpkg.OwnTraegerCareContracts(t, f.db, other.organization)
	rec = traegerJSON(t, f.traeger, other.token, http.MethodGet, contractPath(contract.ID, "/versions/1/document"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}
