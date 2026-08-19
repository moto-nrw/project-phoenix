package students_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func emailPtr(s string) *string { return &s }

// assertNoStudentCommittedUnderTenantTx posts a create-student body that must
// fail guardian validation through the production Router(), whose POST / route
// is wrapped by the REAL TenantTxMiddleware. This matters: the middleware only
// rolls back on 5xx and COMMITS on a 400, so a guardian ValidationError surfaced
// as a 400 would commit an already-written student unless validation happens
// before the first write. The test asserts a 400 carrying the given German
// fragment and proves the middleware did NOT commit an orphaned person/student.
func assertNoStudentCommittedUnderTenantTx(
	t *testing.T,
	tc *testContext,
	firstName, lastName string,
	body map[string]interface{},
	wantBodyContains string,
) {
	t.Helper()

	req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	assert.Equal(t, http.StatusBadRequest, rr.Code,
		"invalid guardian input must return 400 (not 500). Body: %s", rr.Body.String())
	assert.Contains(t, rr.Body.String(), wantBodyContains,
		"400 body should name the offending field, not a generic server error")

	ctx := context.Background()
	defer cleanupOrphanByName(t, tc, firstName, lastName)

	personCount, err := tc.db.NewSelect().
		Table("users.persons").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, personCount,
		"student/person must NOT be committed when guardian validation fails under TenantTxMiddleware")
}

// cleanupOrphanByName removes any person/student left behind by a regressed
// rollback or a prior failed run, keyed by the unique test name.
func cleanupOrphanByName(t *testing.T, tc *testContext, firstName, lastName string) {
	t.Helper()
	ctx := context.Background()
	var personIDs []int64
	if err := tc.db.NewSelect().
		Table("users.persons").
		Column("id").
		Where("first_name = ? AND last_name = ?", firstName, lastName).
		Scan(ctx, &personIDs); err != nil {
		return
	}
	for _, pid := range personIDs {
		if _, err := tc.db.NewDelete().Table("users.students").Where("person_id = ?", pid).Exec(ctx); err != nil {
			t.Logf("cleanup students: %v", err)
		}
		if _, err := tc.db.NewDelete().Table("users.persons").Where("id = ?", pid).Exec(ctx); err != nil {
			t.Logf("cleanup persons: %v", err)
		}
	}
}

// TestCreateStudent_BadGuardianDoesNotCommitUnderTenantTx is the regression test
// for the review blocker: through the REAL TenantTxMiddleware, an invalid
// guardian email must return 400 AND leave no orphaned student. Before the fix
// the middleware committed the student because the 400 is not a 5xx.
func TestCreateStudent_BadGuardianDoesNotCommitUnderTenantTx(t *testing.T) {
	tc := setupTestContext(t)

	const firstName = "MiddlewareRollback"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "1d",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Invalid",
				"last_name":          "Email",
				"email":              "not-a-valid-email",
				"relationship_type":  "parent",
				"emergency_priority": 1,
			},
		},
	}

	assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "Erziehungsberechtigte")
}

// TestCreateStudent_GuardianInvalidRelationshipType covers the Critical finding:
// an unsupported relationship_type (passes the non-empty Bind check) must be a
// 400, not a 500, and must roll back under the real middleware. Mirrors the
// allowed set enforced by the detail-page link endpoint.
func TestCreateStudent_GuardianInvalidRelationshipType(t *testing.T) {
	tc := setupTestContext(t)

	for _, relType := range []string{"sibling", "alien", "grandparent"} {
		t.Run(relType, func(t *testing.T) {
			firstName := "RelType"
			lastName := "Reject_" + relType

			body := map[string]interface{}{
				"first_name":   firstName,
				"last_name":    lastName,
				"school_class": "2a",
				"guardians": []map[string]interface{}{
					{
						"first_name":         "Some",
						"last_name":          "Guardian",
						"relationship_type":  relType,
						"emergency_priority": 1,
					},
				},
			}

			assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "Beziehungstyp")
		})
	}
}

// TestCreateStudent_GuardianRelationshipTypeAccepted confirms every allowed
// relationship type still creates the student+guardian, so tightening the
// validation didn't reject valid input. Runs through the real middleware.
func TestCreateStudent_GuardianRelationshipTypeAccepted(t *testing.T) {
	tc := setupTestContext(t)

	for _, relType := range []string{"parent", "guardian", "relative", "other"} {
		t.Run(relType, func(t *testing.T) {
			body := map[string]interface{}{
				"first_name":   "RelOk",
				"last_name":    "Accept_" + relType,
				"school_class": "2b",
				"guardians": []map[string]interface{}{
					{
						"first_name":         "Valid",
						"last_name":          "Guardian",
						"relationship_type":  relType,
						"emergency_priority": 1,
					},
				},
			}

			req := testutil.NewAuthenticatedRequest(t, "POST", "/", body)
			rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})
			require.Equal(t, http.StatusCreated, rr.Code,
				"relationship_type %q must be accepted. Body: %s", relType, rr.Body.String())

			var resp createStudentResponse
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
			defer cleanupStudentWithGuardians(t, tc, resp.Data.ID, resp.Data.PersonID)
		})
	}
}

// TestCreateStudent_GuardianEmergencyPriorityBelowOne covers the Critical
// finding: emergency_priority < 1 must be rejected with a 400 (parity with the
// detail-page link endpoint, which rejects < 1), not silently stored as 0.
func TestCreateStudent_GuardianEmergencyPriorityBelowOne(t *testing.T) {
	tc := setupTestContext(t)

	for _, prio := range []int{0, -1, -5} {
		t.Run(fmt.Sprintf("priority_%d", prio), func(t *testing.T) {
			firstName := "Prio"
			lastName := fmt.Sprintf("Reject_%d", prio)

			body := map[string]interface{}{
				"first_name":   firstName,
				"last_name":    lastName,
				"school_class": "2c",
				"guardians": []map[string]interface{}{
					{
						"first_name":         "Some",
						"last_name":          "Guardian",
						"relationship_type":  "parent",
						"emergency_priority": prio,
					},
				},
			}

			assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "Notfall-Priorität")
		})
	}
}

// TestCreateStudent_DuplicateGuardianEmail covers the Critical sibling case: a
// guardian email already present in the tenant must surface as a clean 400
// (not a 500) and must not commit an orphaned student. Profile reuse is deferred
// to issue #1513; this test pins the interim contract.
func TestCreateStudent_DuplicateGuardianEmail(t *testing.T) {
	tc := setupTestContext(t)
	ctx := context.Background()

	const dupEmail = "existing.sibling.guardian@example.com"

	// Seed a guardian profile that already owns the email in tenant 1.
	existing := &users.GuardianProfile{
		FirstName: "Existing",
		LastName:  "Guardian",
		Email:     emailPtr(dupEmail),
	}
	existing.SetTenantID(testpkg.Tenant(t))
	_, err := tc.db.NewInsert().Model(existing).Exec(ctx)
	require.NoError(t, err)
	defer func() {
		if _, err := tc.db.NewDelete().
			Table("users.guardian_profiles").
			Where("id = ?", existing.ID).
			Exec(ctx); err != nil {
			t.Logf("cleanup seeded guardian: %v", err)
		}
	}()

	const firstName = "DupEmail"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "2d",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "Sibling",
				"last_name":          "Parent",
				"email":              dupEmail,
				"relationship_type":  "parent",
				"emergency_priority": 1,
			},
		},
	}

	assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "bereits vergeben")
}

// TestCreateStudent_DuplicateGuardianEmailWithinRequest covers two new guardians
// sharing one email in a single request: the in-request duplicate must be
// rejected up front rather than colliding on insert.
func TestCreateStudent_DuplicateGuardianEmailWithinRequest(t *testing.T) {
	tc := setupTestContext(t)

	const firstName = "DupEmailInline"
	const lastName = "Orphan"

	body := map[string]interface{}{
		"first_name":   firstName,
		"last_name":    lastName,
		"school_class": "2e",
		"guardians": []map[string]interface{}{
			{
				"first_name":         "First",
				"last_name":          "Parent",
				"email":              "same.email.twice@example.com",
				"relationship_type":  "parent",
				"emergency_priority": 1,
			},
			{
				"first_name":         "Second",
				"last_name":          "Parent",
				"email":              "same.email.twice@example.com",
				"relationship_type":  "guardian",
				"emergency_priority": 2,
			},
		},
	}

	assertNoStudentCommittedUnderTenantTx(t, tc, firstName, lastName, body, "mehrfach angegeben")
}
