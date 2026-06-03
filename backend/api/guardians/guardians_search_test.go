package guardians_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func strptr(s string) *string { return &s }

// guardianSearchResponse is the envelope shape the guardian picker reads back
// from GET /guardians/search. The projection is minimal and GDPR-safe: name,
// email, and only a COUNT of other linked children — never child names.
type guardianSearchResponse struct {
	Data []struct {
		ID                  int64   `json:"id"`
		FirstName           string  `json:"first_name"`
		LastName            string  `json:"last_name"`
		Email               *string `json:"email"`
		LinkedChildrenCount int     `json:"linked_children_count"`
	} `json:"data"`
}

// seedSearchGuardian inserts a guardian profile in tenant 1 and registers its
// cleanup. Names carry a caller-supplied unique token so search assertions stay
// robust against any other data in the shared test database.
func seedSearchGuardian(t *testing.T, tc *testContext, firstName, lastName, email string) *users.GuardianProfile {
	t.Helper()
	g := &users.GuardianProfile{
		FirstName:              firstName,
		LastName:               lastName,
		Email:                  strptr(email),
		PreferredContactMethod: "email",
		LanguagePreference:     "de",
	}
	g.SetTenantID(1)
	_, err := tc.db.NewInsert().Model(g).Exec(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() { cleanupGuardian(t, tc.db, g.ID) })
	return g
}

// TestListGuardians_SearchFiltersResults verifies ?search= actually filters by
// name (case-insensitive substring) instead of returning every guardian.
func TestListGuardians_SearchFiltersResults(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	token := fmt.Sprintf("Zzmatch%d", time.Now().UnixNano())
	match := seedSearchGuardian(t, tc, token, "Alpha", fmt.Sprintf("%s.match@example.com", token))
	other := seedSearchGuardian(t, tc,
		fmt.Sprintf("Zzother%d", time.Now().UnixNano()), "Beta",
		fmt.Sprintf("other%d@example.com", time.Now().UnixNano()))

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q="+token, nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	var sawMatch, sawOther bool
	for _, g := range resp.Data {
		if g.ID == match.ID {
			sawMatch = true
		}
		if g.ID == other.ID {
			sawOther = true
		}
	}
	assert.True(t, sawMatch, "search must return the guardian whose name contains the token")
	assert.False(t, sawOther, "search must NOT return guardians that don't match the token")
}

// TestListGuardians_SearchMatchesFullNameWithSpace verifies a "First Last"
// query (with a space) finds the guardian even though the first name and last
// name live in different columns. A single full-string LIKE would match neither
// column; the tokenized search matches each word against any column and AND-s
// the words, so both "Andrea Bauer" and the reversed "Bauer Andrea" hit the
// same person.
func TestListGuardians_SearchMatchesFullNameWithSpace(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	first := fmt.Sprintf("Zzfirst%d", time.Now().UnixNano())
	last := fmt.Sprintf("Zzlast%d", time.Now().UnixNano())
	match := seedSearchGuardian(t, tc, first, last, fmt.Sprintf("%s.full@example.com", first))

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	for _, q := range []string{first + " " + last, last + " " + first} {
		req := testutil.NewAuthenticatedRequest(t, "GET",
			"/guardians/search?q="+url.QueryEscape(q), nil,
			testutil.WithClaims(testutil.DefaultTestClaims()),
		)
		rr := testutil.ExecuteRequest(router, req)
		testutil.AssertSuccessResponse(t, rr, http.StatusOK)

		var resp guardianSearchResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

		var saw bool
		for _, g := range resp.Data {
			if g.ID == match.ID {
				saw = true
			}
		}
		assert.True(t, saw, "full-name query %q must find the guardian across first/last name columns", q)
	}
}

// TestListGuardians_SearchWildcardsAreLiteral verifies LIKE metacharacters in
// the query are matched literally, not as wildcards. A "%%%" query (3 chars, so
// it clears the minimum-length guard) must NOT behave like LIKE '%%%%%' and
// return the whole pool — otherwise a non-admin could defeat the enumeration
// guard with raw wildcards (#1513).
func TestListGuardians_SearchWildcardsAreLiteral(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	token := fmt.Sprintf("Zzwild%d", time.Now().UnixNano())
	seeded := seedSearchGuardian(t, tc, token, "Alpha", fmt.Sprintf("%s.wild@example.com", token))

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	// "%%%" url-encoded. If wildcards were active this would match everything;
	// treated literally it matches only a guardian whose name/email contains the
	// literal string "%%%" (none do), so the seeded guardian must be absent.
	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q=%25%25%25", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	for _, g := range resp.Data {
		assert.NotEqual(t, seeded.ID, g.ID,
			"a wildcard-only query must not match guardians via LIKE wildcards")
	}
}

// TestListGuardians_SearchProjectionIsGDPRSafe verifies the picker projection
// is minimal: it returns a COUNT of linked children (never their names), so a
// staff member can't profile a family they don't supervise — address, notes,
// language, and contact method are withheld too (#1513).
func TestListGuardians_SearchProjectionIsGDPRSafe(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	ctx := context.Background()
	token := fmt.Sprintf("Zzlink%d", time.Now().UnixNano())
	email := fmt.Sprintf("%s.parent@example.com", token)
	guardian := seedSearchGuardian(t, tc, token, "Parent", email)

	child := testpkg.CreateTestStudent(t, tc.db, "Lena", "Zzchild", "1a")
	defer func() {
		_, _ = tc.db.NewDelete().Table("users.students").Where("id = ?", child.ID).Exec(ctx)
		_, _ = tc.db.NewDelete().Table("users.persons").Where("id = ?", child.PersonID).Exec(ctx)
	}()

	link := &users.StudentGuardian{
		StudentID:         child.ID,
		GuardianProfileID: guardian.ID,
		RelationshipType:  "parent",
		IsPrimary:         true,
		EmergencyPriority: 1,
	}
	link.SetTenantID(1)
	_, err := tc.db.NewInsert().Model(link).
		ModelTableExpr(`users.students_guardians`).
		Exec(ctx)
	require.NoError(t, err)
	// cleanupGuardian (registered by seedSearchGuardian) also removes the
	// students_guardians link for this guardian.

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q="+token, nil,
		testutil.WithClaims(testutil.TeacherTestClaims(1)),
		testutil.WithPermissions("users:read"),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	var found bool
	for _, g := range resp.Data {
		if g.ID != guardian.ID {
			continue
		}
		found = true
		// Only the COUNT of linked children is exposed — never their names. This
		// is the GDPR-relevant guarantee: the searcher learns a guardian has N
		// other children, not who those children are.
		assert.Equal(t, 1, g.LinkedChildrenCount, "the matched guardian must report its one linked child as a count")
	}
	require.True(t, found, "the seeded guardian must appear in the search results")
}

// pickerSearchRouter mounts the picker search behind its REAL route gate
// (RequiresPermission(users:read)) + the tenant-tx middleware, exactly as
// api.go wires it. RequiresPermission reads the permissions injected via
// testutil.WithPermissions from context, so this exercises the actual gate
// without needing a signed JWT.
func pickerSearchRouter(tc *testContext) chi.Router {
	withTx := tenant.TenantTxMiddleware(tc.db)
	router := chi.NewRouter()
	router.With(authorize.RequiresPermission(permissions.UsersRead), withTx).
		Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())
	return router
}

// TestGuardianPickerSearch_AllowedForStaffWithUsersRead is the core of #1513:
// the picker must be reachable by ordinary staff who manage students (the seeded
// "user" role holds users:read), not just admins. Otherwise a group supervisor
// can create a duplicate guardian but can't find the existing one — the exact
// gap this feature closes. The gate matches the other guardian reads (users:read).
func TestGuardianPickerSearch_AllowedForStaffWithUsersRead(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	token := fmt.Sprintf("Zzperm%d", time.Now().UnixNano())
	match := seedSearchGuardian(t, tc, token, "Alpha", fmt.Sprintf("%s.perm@example.com", token))

	router := pickerSearchRouter(tc)

	// An ordinary (non-admin) staff member holding users:read — no admin:*.
	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q="+token, nil,
		testutil.WithClaims(testutil.TeacherTestClaims(1)),
		testutil.WithPermissions("users:read"),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchResponse
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))

	var found bool
	for _, g := range resp.Data {
		if g.ID == match.ID {
			found = true
		}
	}
	assert.True(t, found, "a non-admin with users:read must be able to find an existing guardian")
}

// TestGuardianPickerSearch_ForbiddenWithoutUsersRead verifies the gate actually
// bites: a caller holding neither users:read nor admin:* is rejected with 403
// before the handler runs.
func TestGuardianPickerSearch_ForbiddenWithoutUsersRead(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	router := pickerSearchRouter(tc)

	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q=anything", nil,
		testutil.WithClaims(testutil.TeacherTestClaims(1)),
		// Unrelated permission only — no users:read, no admin:*.
		testutil.WithPermissions("groups:read"),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}

// guardianSearchEnvelope decodes the pagination metadata of the picker search
// response (the page_size the server actually applied), which the data-only
// guardianSearchResponse ignores.
type guardianSearchEnvelope struct {
	Data       []json.RawMessage `json:"data"`
	Pagination struct {
		PageSize     int `json:"page_size"`
		TotalRecords int `json:"total_records"`
	} `json:"pagination"`
}

// TestGuardianPickerSearch_ShortQueryReturnsEmpty is a regression guard on the
// enumeration defense: a query shorter than the server-side minimum
// (minGuardianPickerQueryLength) must return an empty 200 page — never a 400 and
// never the whole guardian pool. This keeps the picker (open to all staff with
// users:read) from being walked one or two characters at a time, and keeps the
// client's "type at least N characters" hint as the single source of truth (it
// returns OK so the field doesn't flash an error while the user is still typing).
func TestGuardianPickerSearch_ShortQueryReturnsEmpty(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	// Seed a guardian whose name would match the 2-char prefix IF the guard were
	// absent — proving the empty result is the guard firing, not just "no match".
	token := fmt.Sprintf("Zzshort%d", time.Now().UnixNano())
	seedSearchGuardian(t, tc, token, "Alpha", fmt.Sprintf("%s.short@example.com", token))

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	// "Zz" — 2 chars, below the 3-char minimum.
	req := testutil.NewAuthenticatedRequest(t, "GET", "/guardians/search?q=Zz", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Empty(t, resp.Data, "a sub-minimum query must return no candidates")
	assert.Equal(t, 0, resp.Pagination.TotalRecords, "total must be zero for a sub-minimum query")
}

// TestGuardianPickerSearch_ClampsPageSize is a regression guard on the result
// cap: common.ParsePagination imposes NO upper bound on page_size, so without the
// handler's clamp a caller could pass ?page_size=100000 and pull most of the
// tenant's guardian pool in one request, defeating the minimal projection. The
// envelope must report the clamped page size (maxGuardianPickerResults = 50), not
// the requested one.
func TestGuardianPickerSearch_ClampsPageSize(t *testing.T) {
	tc := setupTestContext(t)
	defer func() { _ = tc.db.Close() }()

	token := fmt.Sprintf("Zzclamp%d", time.Now().UnixNano())
	seedSearchGuardian(t, tc, token, "Alpha", fmt.Sprintf("%s.clamp@example.com", token))

	router := chi.NewRouter()
	router.Get("/guardians/search", tc.resource.SearchGuardiansForPickerHandler())

	req := testutil.NewAuthenticatedRequest(t, "GET",
		"/guardians/search?q="+token+"&page_size=100000", nil,
		testutil.WithClaims(testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusOK)

	var resp guardianSearchEnvelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, 50, resp.Pagination.PageSize,
		"a request for page_size=100000 must be clamped to the 50-result ceiling")
}
