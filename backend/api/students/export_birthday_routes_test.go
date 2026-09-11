// Package students_test exercises POST /students/export end-to-end through the
// production Resource.Router(), so the full middleware chain (Verifier →
// Authenticator → TenantMiddleware → RequiresPermission → TenantTxMiddleware)
// runs exactly as it does on the real server.
//
// The unit tests in export_birthday_test.go (package students) pin the
// filtering, sorting and cell rendering without a database. This file pins what
// they cannot see: the permission gate on the route and the HTTP contract the
// browser download depends on.
package students_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func birthdayExportRequest(t *testing.T, body string) *http.Request {
	t.Helper()
	req, err := http.NewRequest("POST", "/export", strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	return req
}

func birthdayExportClaims(tb testing.TB, accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:        int(accountID),
		Sub:       "birthday-export@example.com",
		Username:  "birthday-export",
		FirstName: "Birthday",
		LastName:  "Export",
		Roles:     []string{"user"},
		TenantID:  testpkg.RebaseTenantID(tb, 1),
	}
}

// Berechtigung: the birthday list carries personal data, so it must stay behind
// the same users:read gate as every other student export.
func TestBirthdayExportRequiresUsersReadPermission(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export@example.com")

	body := `{"format":"xlsx","preset":"birthday_list","filters":{"months":["09"]}}`

	t.Run("rejected without users:read", func(t *testing.T) {
		rr := authExec(t, tc, birthdayExportRequest(t, body), birthdayExportClaims(t, account.ID), []string{})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("rejected with an unrelated permission", func(t *testing.T) {
		rr := authExec(t, tc, birthdayExportRequest(t, body), birthdayExportClaims(t, account.ID),
			[]string{permissions.RoomsRead})
		assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	})

	t.Run("allowed with users:read", func(t *testing.T) {
		rr := authExec(t, tc, birthdayExportRequest(t, body), birthdayExportClaims(t, account.ID),
			[]string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	})
}

func TestBirthdayExportRejectsUnauthenticated(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)

	body := `{"format":"xlsx","preset":"birthday_list"}`
	rr := testutil.ExecuteRequest(tc.resource.Router(), birthdayExportRequest(t, body))

	assert.Equal(t, http.StatusUnauthorized, rr.Code, rr.Body.String())
}

// Leere Ergebnisse: a month nobody was born in must still produce a valid,
// downloadable document rather than an error or a zero-byte body.
func TestBirthdayExportEmptyResultStillRendersDocument(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export@example.com")

	// Every fixture child is created without a birthday, so a month filter can
	// never match one — the empty-list path.
	testpkg.CreateTestStudent(t, tc.db, "Birthday", "Nobody", "9z")

	body := `{"format":"xlsx","preset":"birthday_list","filters":{"months":["09"]}}`
	rr := authExec(t, tc, birthdayExportRequest(t, body), birthdayExportClaims(t, account.ID),
		[]string{permissions.UsersRead})

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Header().Get("Content-Disposition"), "geburtstagsliste.xlsx")
	assert.NotEmpty(t, rr.Body.Bytes(), "an empty birthday list must still render a document")
}

// An unknown month must fail loudly. Silently ignoring it would hand the user a
// list that looks complete while covering the wrong period.
func TestBirthdayExportRejectsInvalidMonth(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	account := testpkg.CreateTestAccount(t, tc.db, "birthday-export@example.com")

	body := `{"format":"xlsx","preset":"birthday_list","filters":{"months":["13"]}}`
	rr := authExec(t, tc, birthdayExportRequest(t, body), birthdayExportClaims(t, account.ID),
		[]string{permissions.UsersRead})

	require.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &payload))
	assert.Contains(t, strings.ToLower(rr.Body.String()), "birthday month")
}
