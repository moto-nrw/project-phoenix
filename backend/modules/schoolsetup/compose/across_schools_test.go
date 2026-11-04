package compose_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// inTenant returns the test context moved to another school.
func inTenant(t *testing.T, tenantID int64) context.Context {
	t.Helper()
	return tenant.WithTenantID(testpkg.Ctx(t), tenantID)
}

// createTraeger returns a new Träger with two schools.
func createTraeger(t *testing.T) (organizationID, schoolA, schoolB int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 10)
	organization := testpkg.CreateTestOrganization(t, db, &testpkg.Organization{
		Name: "Träger " + suffix, Slug: "traeger-" + suffix, Active: true,
	})
	return organization.ID,
		testpkg.CreateTestSchoolInOrganization(t, db, organization.ID, "OGS A"),
		testpkg.CreateTestSchoolInOrganization(t, db, organization.ID, "OGS B")
}

// The Absprung (#5659): hiding the wizard in one school of the Träger hides it
// in the others for that person, and for nobody else.
func TestWizardDismissalFollowsTheAbsprungAcrossTheTraeger(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	traeger, schoolA, schoolB := createTraeger(t)
	otherTraeger, otherTraegerSchool, _ := createTraeger(t)

	office := testpkg.CreateTestAccount(t, h.db, "traeger-office")
	colleague := testpkg.CreateTestAccount(t, h.db, "traeger-colleague")
	testpkg.CreateTestTraegerMember(t, h.db, traeger, office.ID)
	testpkg.CreateTestTraegerMember(t, h.db, otherTraeger, office.ID)
	testpkg.CreateTestTraegerMember(t, h.db, traeger, colleague.ID)

	viaTraeger := func(tenantID int64) context.Context {
		return context.WithValue(inTenant(t, tenantID), viaTraegerKey{}, true)
	}

	_, status := h.doIn(t, viaTraeger(schoolB), office.ID, http.MethodGet, "/", nil)
	require.False(t, status.Dismissed, "nothing hidden yet")

	response, status := h.doIn(t, viaTraeger(schoolA), office.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	require.True(t, status.Dismissed)

	_, status = h.doIn(t, viaTraeger(schoolB), office.ID, http.MethodGet, "/", nil)
	assert.True(t, status.Dismissed, "the sibling school follows")

	response, status = h.doIn(t, viaTraeger(schoolB), office.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": false})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.False(t, status.Dismissed, "restoring through the Absprung reaches the dismissal in the sibling school")
	_, status = h.doIn(t, viaTraeger(schoolA), office.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed, "no dismissal remains in the Träger")

	_, status = h.doIn(t, viaTraeger(schoolB), colleague.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed, "a colleague keeps the checklist")

	_, status = h.doIn(t, inTenant(t, schoolB), office.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed, "a session without the Absprung reads this school's own dismissal")

	_, status = h.doIn(t, viaTraeger(otherTraegerSchool), office.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed, "another Träger stays apart")
}

func TestWizardDismissalInTheTraegerNeedsAnActiveMembership(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	traeger, schoolA, schoolB := createTraeger(t)
	former := testpkg.CreateTestAccount(t, h.db, "traeger-former")
	testpkg.CreateTestTraegerMember(t, h.db, traeger, former.ID)
	_, err := h.db.NewRaw(`UPDATE platform.organization_accounts
		SET status = 'inactive', deactivated_at = NOW()
		WHERE organization_id = ? AND account_id = ?`, traeger, former.ID).Exec(context.Background())
	require.NoError(t, err)

	response, _ := h.doIn(t, inTenant(t, schoolA), former.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	ctx := context.WithValue(inTenant(t, schoolB), viaTraegerKey{}, true)
	_, status := h.doIn(t, ctx, former.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed)
}

func TestWizardDismissalIgnoresDeletedSchools(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	traeger, deletedSchool, activeSchool := createTraeger(t)
	office := testpkg.CreateTestAccount(t, h.db, "traeger-deleted-school")
	testpkg.CreateTestTraegerMember(t, h.db, traeger, office.ID)

	viaTraeger := func(tenantID int64) context.Context {
		return context.WithValue(inTenant(t, tenantID), viaTraegerKey{}, true)
	}
	response, _ := h.doIn(t, viaTraeger(deletedSchool), office.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	_, err := h.db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() WHERE id = ?`, deletedSchool).Exec(context.Background())
	require.NoError(t, err)

	_, status := h.doIn(t, viaTraeger(activeSchool), office.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed, "a deleted school's dismissal no longer hides onboarding")
}

// The first steps are general handling (#5659): progress and hiding follow the
// person into every school they have access to.
func TestStaffFirstStepsFollowThePersonAcrossSchools(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, h.db)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-two-schools")
	testpkg.MapAccountToTenant(t, h.db, worker.ID, schoolA)
	testpkg.MapAccountToTenant(t, h.db, worker.ID, schoolB)

	response, _ := staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodPut, "/steps/students", map[string]any{"state": "done"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	response, _ = staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodPut, "/steps/work_time", map[string]any{"state": "skipped"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	_, status := staff.doIn(t, inTenant(t, schoolB), worker.ID, http.MethodGet, "/", nil)
	assert.Equal(t, []string{"students"}, status.DoneSteps)
	assert.Equal(t, []string{"work_time"}, status.SkippedSteps)

	// Taking the skip back in the other school counts everywhere.
	response, _ = staff.doIn(t, inTenant(t, schoolB), worker.ID, http.MethodPut, "/steps/work_time", map[string]any{"state": "open"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	_, status = staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodGet, "/", nil)
	assert.Empty(t, status.SkippedSteps)
	assert.Equal(t, []string{"students"}, status.DoneSteps)

	response, _ = staff.doIn(t, inTenant(t, schoolB), worker.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	_, status = staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodGet, "/", nil)
	assert.True(t, status.Dismissed, "hidden once is hidden in every school of the person")
}

func TestStaffFirstStepsFollowThePersonThroughTheTraegerSession(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	traeger, schoolA, schoolB := createTraeger(t)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-traeger-session")
	testpkg.CreateTestTraegerMember(t, h.db, traeger, worker.ID)

	var selectedSchoolMappings int
	require.NoError(t, h.db.NewRaw(`SELECT COUNT(*) FROM auth.account_tenants WHERE account_id = ? AND tenant_id IN (?, ?)`, worker.ID, schoolA, schoolB).Scan(context.Background(), &selectedSchoolMappings))
	require.Zero(t, selectedSchoolMappings, "a Träger session has no mapping for either selected school")

	response, _ := staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodPut, "/steps/students", map[string]any{"state": "done"})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	_, status := staff.doIn(t, inTenant(t, schoolB), worker.ID, http.MethodGet, "/", nil)
	assert.Equal(t, []string{"students"}, status.DoneSteps)
}

// A session of a school reads nothing about a person without active access to
// that school, even when the person has progress elsewhere.
func TestStaffFirstStepsStayHiddenWithoutAccessToTheSchool(t *testing.T) {
	t.Parallel()
	staff, h := newStaffHarness(t)
	schoolA := testpkg.Tenant(t)
	schoolB, _ := testpkg.CreateTestTenant(t, h.db)
	worker := testpkg.CreateTestAccount(t, h.db, "first-steps-one-school")
	testpkg.MapAccountToTenant(t, h.db, worker.ID, schoolA)

	response, _ := staff.doIn(t, inTenant(t, schoolA), worker.ID, http.MethodPut, "/dismissal", map[string]any{"dismissed": true})
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())

	_, status := staff.doIn(t, inTenant(t, schoolB), worker.ID, http.MethodGet, "/", nil)
	assert.False(t, status.Dismissed)
}
