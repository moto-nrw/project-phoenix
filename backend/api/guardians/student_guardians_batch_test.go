// Package guardians_test — tests for the atomic add-guardians-to-student
// endpoint (POST /students/{studentId}/guardians/batch, #819).
//
// These exercise the handler through the real Resource.Router() with a minted
// JWT so the production middleware chain (Verifier → Authenticator →
// TenantMiddleware → TenantTxMiddleware) runs, rather than via a test-export
// handler wrapper (backend conventions Rule 5).
package guardians_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func init() {
	// Ensure a JWT secret exists so MintTestJWT and Router()'s MustNewTokenAuth
	// agree (no-op when AUTH_JWT_SECRET is already in the environment).
	testutil.SeedTestJWTConfig()
}

// TestCreateStudentGuardians_Admin_Success creates a brand-new guardian (with a
// phone number) for an existing student in one atomic request and verifies it is
// persisted and linked.
func TestCreateStudentGuardians_Admin_Success(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Batch", "Child", "1a")

	router := ctx.resource.Router()

	body := map[string]any{
		"guardians": []map[string]any{
			{
				"first_name":         "Atomic",
				"last_name":          "Guardian",
				"email":              "batch-guardian-" + time.Now().Format("150405.000000") + "@test.com",
				"relationship_type":  "parent",
				"emergency_priority": 1,
				"phone_numbers": []map[string]any{
					{"phone_number": "+49 123 456", "phone_type": "mobile", "is_primary": true},
				},
			},
		},
	}

	claims := testutil.AdminTestClaims(999)
	req := testutil.NewAuthenticatedRequest(t, "POST",
		fmt.Sprintf("/students/%d/guardians/batch", student.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, claims)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertSuccessResponse(t, rr, http.StatusCreated)

	// The guardian is created and linked to the student.
	linked, err := ctx.services.Guardian.GetStudentGuardians(testpkg.TenantContext(1), student.ID)
	require.NoError(t, err)
	require.Len(t, linked, 1, "expected exactly one linked guardian")
	assert.Equal(t, "Atomic", linked[0].Profile.FirstName)
	defer cleanupGuardian(t, ctx.db, linked[0].Profile.ID)
}

// TestCreateStudentGuardians_EmptyGuardians_BadRequest rejects an empty batch.
func TestCreateStudentGuardians_EmptyGuardians_BadRequest(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Empty", "Batch", "1a")

	router := ctx.resource.Router()

	body := map[string]any{"guardians": []map[string]any{}}

	req := testutil.NewAuthenticatedRequest(t, "POST",
		fmt.Sprintf("/students/%d/guardians/batch", student.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, testutil.AdminTestClaims(999))),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertBadRequest(t, rr)
}

// TestCreateStudentGuardians_Forbidden_NonStaff verifies a non-admin caller with
// no staff record cannot attach guardians (same supervisor gate as the
// single-link endpoint).
func TestCreateStudentGuardians_Forbidden_NonStaff(t *testing.T) {
	ctx := setupTestContext(t)

	student := testpkg.CreateTestStudent(t, ctx.db, "Forbidden", "Batch", "1a")

	router := ctx.resource.Router()

	body := map[string]any{
		"guardians": []map[string]any{
			{"first_name": "X", "last_name": "Y", "relationship_type": "parent", "emergency_priority": 1},
		},
	}

	// Authenticated but not admin and not a staff member → canModifyStudent fails.
	nonStaff := jwt.AppClaims{ID: 555, Sub: "nonstaff@test.com", TenantID: 1, Roles: []string{"user"}, Permissions: []string{"users:read"}}
	req := testutil.NewAuthenticatedRequest(t, "POST",
		fmt.Sprintf("/students/%d/guardians/batch", student.ID), body,
		testutil.WithJWTBearer(testutil.MintTestJWT(t, nonStaff)),
	)

	rr := testutil.ExecuteRequest(router, req)
	testutil.AssertForbidden(t, rr)
}
