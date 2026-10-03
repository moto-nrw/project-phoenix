// The Kinderkontingent at the CSV import (#3571): the preview names how many
// children the import would add next to the free places, and an import that
// does not fit never starts. An import that only updates children always runs.
package compose_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	importAPI "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound"
	importCompose "github.com/moto-nrw/project-phoenix/modules/dataimport/inbound/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

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
	testpkg.SetTestChildQuota(t, tc.db, 1, 5)

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
	testpkg.SetTestChildQuota(t, tc.db, 1, 2)

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
	testpkg.SetTestChildQuota(t, tc.db, 1, 2)

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
	testpkg.SetTestChildQuota(t, tc.db, 1, 3)

	rr := testutil.ExecuteRequest(tc.resource.Router(), newMultipartRequestWithMode(t, "/students/import", "k.csv",
		newChildrenCSV(2), "create", adminBearer(t, account.ID)))

	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"CreatedCount":2`)
	assert.Equal(t, 3, liveMemberships(t, tc))
}

func TestImportStudents_UpdatesChildrenWhenTheChildQuotaIsFull(t *testing.T) {
	t.Parallel()
	for _, mode := range []string{"upsert", "update"} {
		t.Run(mode, func(t *testing.T) {
			testpkg.OwnTenant(t)
			tc := setupImportRoute(t)
			_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Update")
			lastName := fmt.Sprintf("Voll%d", time.Now().UnixNano())
			testpkg.CreateTestStudent(t, tc.db, "Schon", lastName, "1a")
			testpkg.SetTestChildQuota(t, tc.db, 1, 1)
			csv := fmt.Sprintf("Vorname,Nachname,Klasse,Ort\nSchon,%s,1a,Bonn\n", lastName)
			router := tc.resource.Router()

			rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv",
				csv, mode, adminBearer(t, account.ID)))
			preview := decodePreview(t, rr.Code, rr.Body.Bytes())
			assert.Equal(t, 1, preview.Data.UpdatedCount)
			require.NotNil(t, preview.Data.ChildQuota)
			assert.Equal(t, childQuotaPreview{BookedPlaces: 1, OccupiedPlaces: 1, RequestedPlaces: 0, FreePlaces: 0, Fits: true},
				*preview.Data.ChildQuota)

			rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv",
				csv, mode, adminBearer(t, account.ID)))
			require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
			assert.Contains(t, rr.Body.String(), `"UpdatedCount":1`)
			assert.Equal(t, 1, liveMemberships(t, tc))
		})
	}
}

// endCare lets a child's care end in the past: the membership stays active
// but no longer counts, as after `Betreuung beenden`.
func endCare(t *testing.T, tc *testContext, studentID int64) {
	t.Helper()
	_, err := tc.db.NewUpdate().TableExpr("users.student_school_memberships").
		Set("enrolled_until = ?::date", "2020-07-31").
		Where("student_profile_id = ?", studentID).Where("tenant_id = ?", testpkg.Tenant(t)).Exec(testpkg.Ctx(t))
	require.NoError(t, err)
}

// An update that brings a child whose care ended back into care raises the
// Kontingentzahl like a new child, so it counts in the preview and the start.
func TestImportStudents_CountsAChildWhoseCareResumes(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Resume")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	lastName := fmt.Sprintf("Ende%d", time.Now().UnixNano())
	ended := testpkg.CreateTestStudent(t, tc.db, "Wieder", lastName, "1a")
	endCare(t, tc, ended.ID)
	testpkg.SetTestChildQuota(t, tc.db, 1, 1)
	csv := fmt.Sprintf("Vorname,Nachname,Klasse,Einschreibung bis\nWieder,%s,1a,31.07.2099\n", lastName)
	router := tc.resource.Router()

	rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		csv, "update", adminBearer(t, account.ID)))
	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	assert.Equal(t, 1, preview.Data.UpdatedCount)
	require.NotNil(t, preview.Data.ChildQuota)
	assert.Equal(t, childQuotaPreview{BookedPlaces: 1, OccupiedPlaces: 1, RequestedPlaces: 1, FreePlaces: 0, Fits: false},
		*preview.Data.ChildQuota)

	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv",
		csv, "update", adminBearer(t, account.ID)))
	require.Equal(t, http.StatusConflict, rr.Code, "Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"code":"students.child_quota_reached"`)
	record, err := tc.student(testpkg.Ctx(t), ended.ID)
	require.NoError(t, err)
	assert.Equal(t, "2020-07-31", record.EnrolledUntil, "the refused import changes nothing")
}

// A new child whose care already ended never enters the Kontingentzahl, so a
// full Kinderkontingent does not block it.
func TestImportStudents_DoesNotCountANewChildWhoseCareAlreadyEnded(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "Quota", "Past")
	testpkg.CreateTestStudent(t, tc.db, "Schon", "Da", "1a")
	testpkg.SetTestChildQuota(t, tc.db, 1, 1)
	csv := fmt.Sprintf("Vorname,Nachname,Klasse,Einschreibung von,Einschreibung bis\nFrüher,Ehemalig%d,1a,01.08.2020,31.07.2021\n",
		time.Now().UnixNano())
	router := tc.resource.Router()

	rr := testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/preview", "k.csv",
		csv, "create", adminBearer(t, account.ID)))
	preview := decodePreview(t, rr.Code, rr.Body.Bytes())
	assert.Equal(t, 1, preview.Data.CreatedCount)
	require.NotNil(t, preview.Data.ChildQuota)
	assert.Equal(t, 0, preview.Data.ChildQuota.RequestedPlaces)
	assert.True(t, preview.Data.ChildQuota.Fits)

	rr = testutil.ExecuteRequest(router, newMultipartRequestWithMode(t, "/students/import", "k.csv",
		csv, "create", adminBearer(t, account.ID)))
	require.Equal(t, http.StatusOK, rr.Code, "Body: %s", rr.Body.String())
	assert.Equal(t, 2, liveMemberships(t, tc))
}

// A batch the Kinderkontingent refuses after earlier batches committed keeps
// that progress in the answer and names the refusal next to it.
func TestBatchFailure_KeepsProgressAndNamesTheChildQuotaRefusal(t *testing.T) {
	t.Parallel()
	runtime := importCompose.HTTPRuntime(nil, nil, nil, nil)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/students/import", nil)
	progress := map[string]int{"CreatedCount": 100, "UpdatedCount": 0, "ErrorCount": 0, "TotalRows": 160}

	runtime.Failure(rr, req, importAPI.Failure{Status: http.StatusInternalServerError, Message: "Import fehlgeschlagen",
		Code: "import.import_batch_failed", Result: progress,
		Cause: fmt.Errorf("batch 2: %w", quotaRefusal{})})

	require.Equal(t, http.StatusConflict, rr.Code, "Body: %s", rr.Body.String())
	var body struct {
		Code    string `json:"code"`
		Details struct {
			Result    map[string]int `json:"result"`
			Rejection struct {
				Code    string         `json:"code"`
				Details map[string]int `json:"details"`
			} `json:"rejection"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "import.import_batch_failed", body.Code)
	assert.Equal(t, progress, body.Details.Result)
	assert.Equal(t, "students.child_quota_reached", body.Details.Rejection.Code)
	assert.Equal(t, map[string]int{"booked_places": 150, "occupied_places": 150, "requested_places": 1}, body.Details.Rejection.Details)
}

// quotaRefusal is a business rejection shaped like School Membership's full
// Kinderkontingent, which this package may not import.
type quotaRefusal struct{}

func (quotaRefusal) Error() string     { return "child quota reached" }
func (quotaRefusal) ErrorCode() string { return "students.child_quota_reached" }
func (quotaRefusal) ErrorDetails() any {
	return map[string]int{"booked_places": 150, "occupied_places": 150, "requested_places": 1}
}
