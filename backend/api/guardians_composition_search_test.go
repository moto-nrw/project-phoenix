package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The guardian picker search (#1513): a minimal, enumeration-resistant
// projection open to every staff member holding users:read.

type guardianSearchResponse struct {
	Data []struct {
		ID                  int64   `json:"id"`
		FirstName           string  `json:"first_name"`
		LastName            string  `json:"last_name"`
		Email               *string `json:"email"`
		LinkedChildrenCount int     `json:"linked_children_count"`
	} `json:"data"`
	Pagination struct {
		PageSize     int `json:"page_size"`
		TotalRecords int `json:"total_records"`
	} `json:"pagination"`
}

func (c *guardianCompositionContext) searchAs(t *testing.T, query string, reader func() (int, []byte)) guardianSearchResponse {
	t.Helper()
	code, body := reader()
	require.Equal(t, http.StatusOK, code, string(body))
	var response guardianSearchResponse
	require.NoError(t, json.Unmarshal(body, &response))
	return response
}

func (c *guardianCompositionContext) searchDefault(t *testing.T, query string) guardianSearchResponse {
	t.Helper()
	return c.searchAs(t, query, func() (int, []byte) {
		rr := c.do(t, testutil.DefaultTestClaims(), http.MethodGet, "/search?"+query, nil)
		return rr.Code, rr.Body.Bytes()
	})
}

func (r guardianSearchResponse) has(id int64) bool {
	for _, guardian := range r.Data {
		if guardian.ID == id {
			return true
		}
	}
	return false
}

func TestGuardianComposition_SearchFiltersResults(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzmatch%d", testpkg.UniqueSuffix())
	matchID, _ := ctx.createNamedGuardian(token, "Alpha", token+".match")
	otherID, _ := ctx.createNamedGuardian(fmt.Sprintf("Zzother%d", testpkg.UniqueSuffix()), "Beta", "other")

	response := ctx.searchDefault(t, "q="+token)
	assert.True(t, response.has(matchID), "search must return the guardian whose name contains the token")
	assert.False(t, response.has(otherID), "search must NOT return guardians that don't match the token")
}

// A "First Last" query (with a space) finds the guardian even though the two
// names live in different columns; the words are matched against any column
// and AND-ed, so the reversed order hits the same person.
func TestGuardianComposition_SearchMatchesFullNameWithSpace(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	first := fmt.Sprintf("Zzfirst%d", testpkg.UniqueSuffix())
	last := fmt.Sprintf("Zzlast%d", testpkg.UniqueSuffix())
	matchID, _ := ctx.createNamedGuardian(first, last, first+".full")

	for _, q := range []string{first + " " + last, last + " " + first} {
		response := ctx.searchDefault(t, "q="+url.QueryEscape(q))
		assert.True(t, response.has(matchID), "full-name query %q must find the guardian across first/last name columns", q)
	}
}

// LIKE metacharacters are matched literally: a "%%%" query (3 chars, so it
// clears the minimum length) must not return the whole pool.
func TestGuardianComposition_SearchWildcardsAreLiteral(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzwild%d", testpkg.UniqueSuffix())
	seededID, _ := ctx.createNamedGuardian(token, "Alpha", token+".wild")

	response := ctx.searchDefault(t, "q=%25%25%25")
	assert.False(t, response.has(seededID), "a wildcard-only query must not match guardians via LIKE wildcards")
}

// The picker projection is minimal: a COUNT of linked children, never their
// names, and none of the address, notes or preference fields.
func TestGuardianComposition_SearchProjectionIsGDPRSafe(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzlink%d", testpkg.UniqueSuffix())
	guardianID, _ := ctx.createNamedGuardian(token, "Parent", token+".parent")
	childID, _ := ctx.createStudent("Lena", "Zzchild", "1a")
	ctx.link(childID, guardianID, "primary_guardian")

	rr := ctx.do(t, withPerms(testutil.TeacherTestClaims(1), "users:read"), http.MethodGet, "/search?q="+token, nil)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)
	var response guardianSearchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))

	var found bool
	for _, guardian := range response.Data {
		if guardian.ID != guardianID {
			continue
		}
		found = true
		assert.Equal(t, 1, guardian.LinkedChildrenCount, "the matched guardian must report its one linked child as a count")
	}
	require.True(t, found, "the seeded guardian must appear in the search results")
	assert.NotContains(t, rr.Body.String(), "Zzchild", "child names never leave the owner")
	assert.NotContains(t, rr.Body.String(), "address_street")
}

func TestGuardianComposition_SearchGate(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzperm%d", testpkg.UniqueSuffix())
	matchID, _ := ctx.createNamedGuardian(token, "Alpha", token+".perm")

	t.Run("ordinary staff with users:read may search", func(t *testing.T) {
		response := ctx.searchAs(t, "q="+token, func() (int, []byte) {
			rr := ctx.do(t, withPerms(testutil.TeacherTestClaims(1), "users:read"), http.MethodGet, "/search?q="+token, nil)
			return rr.Code, rr.Body.Bytes()
		})
		assert.True(t, response.has(matchID))
	})

	t.Run("without users:read the search is forbidden", func(t *testing.T) {
		testutil.AssertForbidden(t, ctx.do(t, withPerms(testutil.TeacherTestClaims(1), "groups:read"), http.MethodGet, "/search?q=anything", nil))
	})
}

// A query shorter than the server-side minimum returns an empty 200 page,
// never a 400 and never the whole pool.
func TestGuardianComposition_SearchShortQueryReturnsEmpty(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzshort%d", testpkg.UniqueSuffix())
	ctx.createNamedGuardian(token, "Alpha", token+".short")

	response := ctx.searchDefault(t, "q=Zz")
	assert.Empty(t, response.Data)
	assert.Equal(t, 0, response.Pagination.TotalRecords)
}

// The result cap: the envelope reports the clamped page size, not the
// requested one.
func TestGuardianComposition_SearchClampsPageSize(t *testing.T) {
	t.Parallel()
	ctx := setupGuardiansCompositionRoute(t)
	token := fmt.Sprintf("Zzclamp%d", testpkg.UniqueSuffix())
	ctx.createNamedGuardian(token, "Alpha", token+".clamp")

	response := ctx.searchDefault(t, "q="+token+"&page_size=100000")
	assert.Equal(t, 50, response.Pagination.PageSize, "a request for page_size=100000 must be clamped to the 50-result ceiling")
}
