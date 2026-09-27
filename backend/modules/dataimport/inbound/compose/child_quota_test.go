// The Kinderkontingent at the CSV import (#3571): the preview names how many
// children the import would add next to the free places, and an import that
// does not fit never starts. An import that only updates children always runs.
package compose_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func setChildQuota(t *testing.T, tc *testContext, bundles, bundleSize int) {
	t.Helper()
	_, err := tc.db.NewUpdate().TableExpr("platform.schools").
		Set("child_quota_bundles = ?", bundles).Set("child_quota_bundle_size = ?", bundleSize).
		Where("id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

func liveMemberships(t *testing.T, tc *testContext) int {
	t.Helper()
	count, err := tc.db.NewSelect().TableExpr("users.student_school_memberships").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("deleted_at IS NULL").Count(testpkg.Ctx(t))
	require.NoError(t, err)
	return count
}

type childQuotaPreview struct {
	BookedPlaces    int  `json:"booked_places"`
	OccupiedPlaces  int  `json:"occupied_places"`
	RequestedPlaces int  `json:"requested_places"`
	FreePlaces      int  `json:"free_places"`
	Fits            bool `json:"fits"`
}

type previewBody struct {
	Data struct {
		CreatedCount int                `json:"CreatedCount"`
		UpdatedCount int                `json:"UpdatedCount"`
		ChildQuota   *childQuotaPreview `json:"child_quota"`
	} `json:"data"`
}

func decodePreview(t *testing.T, status int, body []byte) previewBody {
	t.Helper()
	require.Equal(t, http.StatusOK, status, "Body: %s", body)
	var decoded previewBody
	require.NoError(t, json.Unmarshal(body, &decoded))
	return decoded
}

// newChildrenCSV lists count children no other test and no fixture shares.
func newChildrenCSV(count int) string {
	csv := "Vorname,Nachname,Klasse\n"
	unique := time.Now().UnixNano()
	for i := range count {
		csv += fmt.Sprintf("Neu%d,Kontingent%d,1a\n", i, unique)
	}
	return csv
}

func TestPreviewStudentImport_NamesTheChildQuotaWhenTheImportFits(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Fits")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 5)

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		newChildrenCSV(2), "create", adminBearer(t, account.ID)))

	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	assert.Equal(t, 2, preview.Data.CreatedCount)
	require.NotNil(t, preview.Data.ChildQuota)
	assert.Equal(t, childQuotaPreview{BookedPlaces: 5, OccupiedPlaces: 1, RequestedPlaces: 2, FreePlaces: 4, Fits: true},
		*preview.Data.ChildQuota)
}

func TestPreviewStudentImport_FlagsAnImportTheChildQuotaCannotHold(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Short")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 2)

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		newChildrenCSV(3), "create", adminBearer(t, account.ID)))

	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	require.NotNil(t, preview.Data.ChildQuota)
	assert.Equal(t, childQuotaPreview{BookedPlaces: 2, OccupiedPlaces: 1, RequestedPlaces: 3, FreePlaces: 1, Fits: false},
		*preview.Data.ChildQuota)
	assert.Equal(t, 1, liveMemberships(t, tc), "the preview writes no child")
}

func TestPreviewStudentImport_OmitsTheChildQuotaWithoutOne(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "None")

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		newChildrenCSV(2), "create", adminBearer(t, account.ID)))

	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	assert.Equal(t, 2, preview.Data.CreatedCount)
	assert.Nil(t, preview.Data.ChildQuota)
	assert.NotContains(t, rr.Body.String(), "child_quota")
}

func TestImportStudents_RefusesAnImportTheChildQuotaCannotHold(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Refused")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 2)

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/import", "k.csv",
		newChildrenCSV(3), "create", adminBearer(t, account.ID)))

	require.Equal(t, http.StatusConflict, rr.Code, "Body: %s", rr.Body.String())
	var decoded struct {
		Code    string         `json:"code"`
		Details map[string]int `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &decoded))
	assert.Equal(t, "students.child_quota_reached", decoded.Code)
	assert.Equal(t, map[string]int{"booked_places": 2, "occupied_places": 1, "requested_places": 3}, decoded.Details)
	// All or nothing: the one child that would still fit is not created either.
	assert.Equal(t, 1, liveMemberships(t, tc))
}

func TestImportStudents_CreatesChildrenTheChildQuotaCanHold(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Created")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	setChildQuota(t, tc, 1, 3)

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/import", "k.csv",
		newChildrenCSV(2), "create", adminBearer(t, account.ID)))

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"CreatedCount":2`)
	assert.Equal(t, 3, liveMemberships(t, tc))
}

func TestImportStudents_UpdatesChildrenWhenTheChildQuotaIsFull(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Update")
	unique := time.Now().UnixNano()
	lastName := fmt.Sprintf("Voll%d", unique)
	testpkg.CreateTestStudent(t, tc.db, "Schon", lastName, "1a")
	setChildQuota(t, tc, 1, 1)
	csv := fmt.Sprintf("Vorname,Nachname,Klasse,Ort\nSchon,%s,1a,Bonn\n", lastName)
	router := tc.resource.Router()

	rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		csv, "upsert", adminBearer(t, account.ID)))
	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	assert.Equal(t, 1, preview.Data.UpdatedCount)
	require.NotNil(t, preview.Data.ChildQuota)
	assert.Equal(t, childQuotaPreview{BookedPlaces: 1, OccupiedPlaces: 1, RequestedPlaces: 0, FreePlaces: 0, Fits: true},
		*preview.Data.ChildQuota)

	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv",
		csv, "upsert", adminBearer(t, account.ID)))
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"UpdatedCount":1`)
	assert.Equal(t, 1, liveMemberships(t, tc))
}
