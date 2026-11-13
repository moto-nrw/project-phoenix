package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	apiCommon "github.com/moto-ogs/project-phoenix/api/common"
	"github.com/moto-ogs/project-phoenix/api/testutil"
	identityaccessCompose "github.com/moto-ogs/project-phoenix/modules/identityaccess/compose"
	owners "github.com/moto-ogs/project-phoenix/roots/owners"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// DELETE /api/users/{id} retires a bare person only. A person that still
// carries a live child or staff profile answers 409 with a registered code
// and keeps its row: leaving the school runs through the student deletion
// or the staff offboarding (#6840).
func TestDeletePersonRefusesPersonsWithLiveProfiles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	people, err := owners.NewPeopleDirectory(testpkg.NewDatabase(t, db))
	require.NoError(t, err)
	identity, err := identityaccessCompose.New(identityaccessCompose.Dependencies{DB: db, Observe: func(identityaccessCompose.Observation) {}})
	require.NoError(t, err)
	resource := NewUsersResource(people, identity.ListAccountEmails, func(ctx context.Context, tagID string) (bool, error) {
		_, _, found, err := identity.LookupRFIDCard(ctx, tagID)
		return found, err
	})
	router := chi.NewRouter()
	router.Use(testpkg.TenantRuntimeMiddleware(t, db))
	router.Mount("/api/users", resource.Router())

	claims := testutil.DefaultTestClaims()
	claims.Roles = []string{"user"}
	claims.IsAdmin = false
	claims.Permissions = []string{"users:delete"}
	token := testutil.MintTestJWT(t, claims)
	deletePerson := func(personID int64) *httptest.ResponseRecorder {
		request := testutil.NewAuthenticatedRequest(t, http.MethodDelete, "/api/users/"+strconv.FormatInt(personID, 10), nil,
			testutil.WithJWTBearer(token))
		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, request)
		return recorder
	}

	student := testpkg.CreateTestStudent(t, db, "Live", "Child", "3a")
	staff := testpkg.CreateTestStaff(t, db, "Live", "Staff")
	for name, personID := range map[string]int64{"child": student.PersonID, "staff": staff.PersonID} {
		recorder := deletePerson(personID)
		require.Equal(t, http.StatusConflict, recorder.Code, "%s: %s", name, recorder.Body.String())
		var body struct {
			Code string `json:"code"`
		}
		require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
		assert.Equal(t, apiCommon.CodeStudentsPersonHasActiveProfile, body.Code, name)
		assert.False(t, personDeleted(t, db, personID), "%s: the person must stay", name)
	}

	former := testpkg.CreateTestStaff(t, db, "Former", "Staff")
	retireStaffMembership(t, db, former.ID)
	bare := testpkg.CreateTestPerson(t, db, "Bare", "Person")
	for name, personID := range map[string]int64{"former staff": former.PersonID, "bare person": bare.ID} {
		recorder := deletePerson(personID)
		require.Equal(t, http.StatusNoContent, recorder.Code, "%s: %s", name, recorder.Body.String())
		assert.True(t, personDeleted(t, db, personID), name)
	}
}

func personDeleted(t *testing.T, db *bun.DB, personID int64) bool {
	t.Helper()
	var deleted bool
	require.NoError(t, db.NewSelect().TableExpr("users.persons").
		ColumnExpr("deleted_at IS NOT NULL").
		Where("id = ?", personID).
		Scan(testpkg.Ctx(t), &deleted))
	return deleted
}
