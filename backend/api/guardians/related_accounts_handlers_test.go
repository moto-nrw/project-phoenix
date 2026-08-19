// Package guardians_test — staff invite endpoint tests for the restricted-
// contact upgrade flow (#2172). Full-stack: real services + test DB, driven
// through Router() with a signed admin JWT.
package guardians_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	repositories "github.com/moto-nrw/project-phoenix/database/repositories"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestInviteGuardianToStudent_RestrictedContactRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Staff", "Invite", "8a")
	profile := testpkg.CreateTestGuardianProfile(t, ctx.db, "staff-restricted")
	tenantCtx := testpkg.TenantContext(1)
	defer func() {
		_, _ = ctx.db.NewDelete().TableExpr("auth.guardian_invitations").Where("guardian_profile_id = ?", profile.ID).Exec(context.Background())
		_, _ = ctx.db.NewDelete().TableExpr("users.students_guardians").Where("student_id = ?", student.ID).Exec(context.Background())
		_, _ = ctx.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
		_, _ = ctx.db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(context.Background())
	}()

	link := &userModels.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		EmergencyPriority: 1,
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleEmergency)
	link.SetTenantID(1)
	repos := repositories.NewFactory(ctx.db)
	require.NoError(t, repos.StudentGuardian.Create(tenantCtx, link))

	router := ctx.resource.Router()
	path := "/students/" + strconv.FormatInt(student.ID, 10) + "/invite"

	// Without confirmation → detection outcome, no upgrade, existing_role set.
	req := testutil.NewAuthenticatedRequest(t, "POST", path,
		map[string]any{"email": *profile.Email},
		bearer(t, testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"outcome":"existing_contact_restricted"`)
	assert.Contains(t, rr.Body.String(), `"existing_role":"emergency_contact"`)

	current, err := repos.StudentGuardian.FindByStudentAndGuardianForUpdate(tenantCtx, student.ID, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, authorize.GuardianRoleEmergency, current.GuardianRole, "unconfirmed invite must not change the link")

	// With confirmation → resolve proceeds and the link is upgraded.
	req = testutil.NewAuthenticatedRequest(t, "POST", path,
		map[string]any{"email": *profile.Email, "confirm_role_upgrade": true},
		bearer(t, testutil.DefaultTestClaims()),
	)
	rr = testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"outcome":"invited"`)

	current, err = repos.StudentGuardian.FindByStudentAndGuardianForUpdate(tenantCtx, student.ID, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, authorize.GuardianRoleLegalGuardian, current.GuardianRole)
	assert.True(t, authorize.StudentGuardianHasPermission(current, authorize.GuardianPermissionPortalAccess))
}

func TestInviteGuardianToStudent_SocialWorkerLinkRefused(t *testing.T) {
	t.Parallel()
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Staff", "SocialWorker", "8b")
	profile := testpkg.CreateTestGuardianProfile(t, ctx.db, "staff-social-worker")
	tenantCtx := testpkg.TenantContext(1)
	defer func() {
		_, _ = ctx.db.NewDelete().TableExpr("users.students_guardians").Where("student_id = ?", student.ID).Exec(context.Background())
		_, _ = ctx.db.NewDelete().TableExpr("users.guardian_profiles").Where("id = ?", profile.ID).Exec(context.Background())
		_, _ = ctx.db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(context.Background())
	}()

	link := &userModels.StudentGuardian{
		StudentID:         student.ID,
		GuardianProfileID: profile.ID,
		RelationshipType:  "other",
		EmergencyPriority: 1,
	}
	authorize.ApplyStudentGuardianRole(link, authorize.GuardianRoleSocialWorker)
	link.SetTenantID(1)
	repos := repositories.NewFactory(ctx.db)
	require.NoError(t, repos.StudentGuardian.Create(tenantCtx, link))

	router := ctx.resource.Router()
	path := "/students/" + strconv.FormatInt(student.ID, 10) + "/invite"

	// Even a confirmed upgrade attempt is refused for a social-worker link.
	req := testutil.NewAuthenticatedRequest(t, "POST", path,
		map[string]any{"email": *profile.Email, "confirm_role_upgrade": true},
		bearer(t, testutil.DefaultTestClaims()),
	)
	rr := testutil.ExecuteRequest(router, req)
	require.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())

	current, err := repos.StudentGuardian.FindByStudentAndGuardianForUpdate(tenantCtx, student.ID, profile.ID)
	require.NoError(t, err)
	assert.Equal(t, authorize.GuardianRoleSocialWorker, current.GuardianRole, "link must be untouched")
}
