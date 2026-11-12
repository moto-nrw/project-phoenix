// Router-level tests for the hint on a known child in the review of the
// holiday registrations (#6683, ADR 0214): a registered child that exactly
// one actively managed child of the Träger's schools matches names that
// school; namesakes at two schools, children of another Träger, the right
// "Anmeldung verwalten" and the trail entry before the answer.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/identityaccess"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	querybudgets "github.com/moto-ogs/project-phoenix/test/query-budgets"
)

type knownChildBody struct {
	RegistrationID int64 `json:"registration_id"`
	ChildID        int64 `json:"child_id"`
	SchoolID       int64 `json:"school_id"`
}

// schoolChild creates an actively managed child of a school with the names
// and the date of birth a registration names.
func schoolChild(t *testing.T, f reviewFixture, schoolID int64, firstName, lastName, birthday string) {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, f.db, schoolID, firstName, lastName, "2a")
	_, err := f.db.NewRaw(`UPDATE users.persons SET birthday = ?::date WHERE id = ?`, birthday, student.PersonID).Exec(context.Background())
	require.NoError(t, err)
}

func (f reviewFixture) knownChildren(t *testing.T, token, query string) []knownChildBody {
	t.Helper()
	rec := traegerJSON(t, f.traeger, token, http.MethodGet, f.path("/known-children"+query), nil)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	return decodeSchoolPortalData[struct {
		Children []knownChildBody `json:"children"`
	}](t, rec.Body.Bytes()).Children
}

// knownChildrenEntries are the trail entries of the comparison, oldest
// first, with their details.
func (f reviewFixture) knownChildrenEntries(t *testing.T) []map[string]any {
	t.Helper()
	var rows []struct {
		Details string `bun:"details"`
	}
	require.NoError(t, f.db.NewRaw(`SELECT details::text AS details FROM enrollment.holiday_registration_audit
		WHERE holiday_period_id = ? AND action = 'known_children' AND holiday_registration_id IS NULL ORDER BY id`, f.period.ID).
		Scan(context.Background(), &rows))
	entries := make([]map[string]any, len(rows))
	for i, row := range rows {
		require.NoError(t, json.Unmarshal([]byte(row.Details), &entries[i]))
	}
	return entries
}

func TestTraegerSeesWhichSchoolKnowsARegisteredChild(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bekannt Ferien")
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Bekannt Fremd", identityaccess.TraegerRoleAdministration)
	require.Len(t, f.offer.Schools, 2)
	north, south := f.offer.Schools[0].ID, f.offer.Schools[1].ID

	// Ben came as a holiday guest, but the second school knows him; a
	// namesake at another Träger's school does not count.
	schoolChild(t, f, south, "Ben", "Muster", "2016-09-12")
	schoolChild(t, f, foreign.schools[0], "Ben", "Muster", "2016-09-12")
	// Lia has namesakes at both schools: no hint.
	schoolChild(t, f, north, "Lia", "Muster", "2018-03-01")
	schoolChild(t, f, south, "lia", "MUSTER", "2018-03-01")

	list := f.list(t, "")
	ben := registrationByID(t, list, f.siblings).Children[1]
	require.Equal(t, "Ben", ben.FirstName)

	known := f.knownChildren(t, f.office.token, "")
	assert.Equal(t, []knownChildBody{{RegistrationID: f.siblings, ChildID: ben.ID, SchoolID: south}}, known,
		"only Ben is known, at the one school of the Träger that manages him")
	assert.Equal(t, known, f.knownChildren(t, f.office.token, fmt.Sprintf("?registration_id=%d", f.siblings)))
	assert.Empty(t, f.knownChildren(t, f.office.token, fmt.Sprintf("?registration_id=%d", f.again)),
		"the second registration holds only Lia")

	entries := f.knownChildrenEntries(t)
	require.Len(t, entries, 3, "every comparison is recorded")
	assert.Equal(t, []any{float64(ben.ID)}, entries[0]["children"])
	assert.NotContains(t, entries[0], "registration_id")
	assert.Equal(t, float64(f.siblings), entries[1]["registration_id"])
	assert.Equal(t, []any{}, entries[2]["children"], "a comparison without a hit is recorded too")
	assert.NotContains(t, fmt.Sprint(entries), "Muster", "the trail names no child")

	// A cancelled registration gives no hint any more.
	rec := f.act(t, f.siblings, "cancel")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	assert.Empty(t, f.knownChildren(t, f.office.token, ""))
}

func TestTraegerKnownChildrenNeedTheRightAndStayWithTheTraeger(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bekannt Grenze")
	readOnly := newTraegerOffice(t, f.db, f.traeger, "Träger Bekannt Lesend", identityaccess.TraegerRoleReadOnly)
	foreign := newTraegerOffice(t, f.db, f.traeger, "Träger Bekannt Andere", identityaccess.TraegerRoleAdministration)
	schoolChild(t, f, f.offer.Schools[1].ID, "Ben", "Muster", "2016-09-12")

	rec := traegerJSON(t, f.traeger, readOnly.token, http.MethodGet, f.path("/known-children"), nil)
	assert.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	rec = traegerJSON(t, f.traeger, foreign.token, http.MethodGet, f.path("/known-children"), nil)
	assert.Equal(t, http.StatusNotFound, rec.Code, "the period of another Träger: %s", rec.Body.String())
	rec = traegerJSON(t, f.traeger, f.office.token, http.MethodGet, f.path("/known-children?registration_id=abc"), nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Empty(t, f.knownChildrenEntries(t), "refused requests record nothing")
}

func TestTraegerKnownChildrenQueryBudgetIsFlat(t *testing.T) {
	t.Parallel()
	f := newReviewFixture(t, "Träger Bekannt Budget")
	schoolChild(t, f, f.offer.Schools[1].ID, "Ben", "Muster", "2016-09-12")
	known := func() testpkg.QueryMeasurement {
		counter := testpkg.CaptureQueriesForContext(t, f.db)
		req := httptest.NewRequest(http.MethodGet, f.path("/known-children"), nil)
		req.Header.Set("Authorization", "Bearer "+f.office.token)
		rec := httptest.NewRecorder()
		testpkg.MeasuredHTTPRequest(counter, f.traeger, rec, req.WithContext(counter.Context(req.Context())))
		require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
		return counter.Measurement(counter.Selects(""))
	}
	one := known()
	rec := publicJSON(t, f.public, http.MethodPost, registrationPath(f.period.PublicToken), siblingsRegistration(f.offer, "bekannt@example.org"))
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	schoolChild(t, f, f.offer.Schools[0].ID, "Ben", "Muster", "2016-09-13")
	assert.Len(t, known().Statements, len(one.Statements), "the comparison reads the same statements for any number of children")
	querybudgets.AssertQueryBudget(t, "api.traeger.holiday_known_children", one)
}
