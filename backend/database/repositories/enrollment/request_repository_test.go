package enrollment_test

import (
	"context"
	"fmt"
	"hash/fnv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupRequestRepoTest gives us a request repo + a real phase row to
// reference. The (request_id → phase_id) FK requires a phase to exist
// before a request can be inserted.
func setupRequestRepoTest(t *testing.T) (*bun.DB, enrollmentModels.RequestRepository, int64, int64) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	tenantID := tenantIDForTestName(t.Name())
	testpkg.EnsureTestTenant(t, db, tenantID)

	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	phaseName := uniquePhaseName("request-test")
	phase := makeValidPhase(phaseName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, phase)
	}))
	t.Cleanup(func() { wipePhases(db, tenantID, phaseName) })

	return db, enrollmentRepo.NewRequestRepository(db), tenantID, phase.ID
}

func tenantIDForTestName(name string) int64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(name))
	return 200_000 + int64(h.Sum32()%100_000)
}

// wipeRequests removes every request + child row for the tenant whose
// status_token starts with the given prefix. Per-test isolation.
func wipeRequests(db *bun.DB, tenantID int64, tokenPrefix string) {
	bg := context.Background()
	_, _ = db.NewDelete().
		TableExpr("enrollment.request_children rc").
		Where(`rc.tenant_id = ? AND rc.request_id IN (SELECT id FROM enrollment.requests WHERE tenant_id = ? AND status_token LIKE ?)`,
			tenantID, tenantID, tokenPrefix+"%").
		Exec(bg)
	_, _ = db.NewDelete().
		TableExpr("enrollment.requests").
		Where("tenant_id = ? AND status_token LIKE ?", tenantID, tokenPrefix+"%").
		Exec(bg)
}

func uniqueToken(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func makeRequest(phaseID int64, token, email string) *enrollmentModels.Request {
	return &enrollmentModels.Request{
		PhaseID:           phaseID,
		GuardianFirstName: "Anna",
		GuardianLastName:  "Beispiel",
		GuardianEmail:     email,
		ConsentFlags:      map[string]any{},
		CustomData:        map[string]any{},
		StatusToken:       token,
		SubmittedAt:       time.Now().UTC(),
	}
}

// insertRequestChild bypasses the request_child repo to keep this
// test file independent of that repo's contract. Tenant + phase
// already set up by the caller.
func insertRequestChild(t *testing.T, db *bun.DB, tenantID, requestID int64, firstName, lastName, status string) {
	t.Helper()
	bg := context.Background()
	_, err := db.NewRaw(`
		INSERT INTO enrollment.request_children
		  (tenant_id, request_id, first_name, last_name, date_of_birth,
		   status, activation_mode, sort_order, custom_data)
		VALUES (?, ?, ?, ?, '2018-04-15', ?, 'scheduled', 0, '{}'::jsonb)
	`, tenantID, requestID, firstName, lastName, status).Exec(bg)
	require.NoError(t, err)
}

// --- Create + FindByID -------------------------------------------------

func TestRequestRepository_Create_PersistsAndReturnsID(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("create")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	})
	require.NoError(t, err)
	assert.NotZero(t, req.ID)
	assert.Equal(t, tenantID, req.TenantID)
}

func TestRequestRepository_FindByID_HappyPath(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("find")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	var got *enrollmentModels.Request
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, req.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, req.ID, got.ID)
	assert.Equal(t, token, got.StatusToken)
}

func TestRequestRepository_FindByID_NotFound(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)

	var got *enrollmentModels.Request
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByID(ctx, 9_999_999)
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
}

func TestRequestRepository_PinDecisionNotificationMode_FirstPinWins(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("pinNotificationMode")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	var first, second string
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		first, err = repo.PinDecisionNotificationMode(ctx, req.ID, configModels.EnrollmentNotifyPerDecisionDigest)
		return err
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		second, err = repo.PinDecisionNotificationMode(ctx, req.ID, configModels.EnrollmentNotifyPerDecisionImmediate)
		return err
	}))

	assert.Equal(t, configModels.EnrollmentNotifyPerDecisionDigest, first)
	assert.Equal(t, first, second)
	var stored *enrollmentModels.Request
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var err error
		stored, err = repo.FindByID(ctx, req.ID)
		return err
	}))
	require.NotNil(t, stored.DecisionNotificationMode)
	assert.Equal(t, configModels.EnrollmentNotifyPerDecisionDigest, *stored.DecisionNotificationMode)
}

func TestRequestRepository_PinDecisionNotificationMode_RejectsInvalidMode(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("invalidNotificationMode")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, pinErr := repo.PinDecisionNotificationMode(ctx, req.ID, "batch")
		return pinErr
	})
	require.Error(t, err)
	assert.ErrorContains(t, err, "invalid enrollment decision notification mode")
}

// --- FindByStatusToken -------------------------------------------------

func TestRequestRepository_FindByStatusToken_HappyPath(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("findtoken")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	var got *enrollmentModels.Request
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByStatusToken(ctx, token)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, req.ID, got.ID)
}

func TestRequestRepository_FindByStatusToken_UnknownTokenErrors(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)

	var got *enrollmentModels.Request
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = repo.FindByStatusToken(ctx, "no-such-token-"+t.Name())
		return fbErr
	})
	require.Error(t, err)
	assert.Nil(t, got)
}

// --- ListAdmin ---------------------------------------------------------

func TestRequestRepository_ListAdmin_NoFiltersReturnsAll(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("listAll")
	defer wipeRequests(db, tenantID, token)

	r1 := makeRequest(phaseID, token+"-a", "anna@example.test")
	r2 := makeRequest(phaseID, token+"-b", "bert@example.test")
	r1.SubmittedAt = time.Now().Add(-2 * time.Hour).UTC()
	r2.SubmittedAt = time.Now().UTC()

	for _, r := range []*enrollmentModels.Request{r1, r2} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var list []*enrollmentModels.Request
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListAdmin(ctx, enrollmentModels.RequestListFilters{})
		return lErr
	})
	require.NoError(t, err)

	// Filter to ours and check order.
	var ours []*enrollmentModels.Request
	for _, r := range list {
		if r.ID == r1.ID || r.ID == r2.ID {
			ours = append(ours, r)
		}
	}
	require.Len(t, ours, 2)
	assert.Equal(t, r2.ID, ours[0].ID, "newest submitted_at first")
	assert.Equal(t, r1.ID, ours[1].ID)
}

func TestRequestRepository_ListAdmin_PhaseFilter(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("listPhase")
	defer wipeRequests(db, tenantID, token)

	// A second phase so we can verify the filter actually narrows.
	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	otherPhase := makeValidPhase(uniquePhaseName("other"))
	otherPhase.ServiceStartDate = timezone.NewDate(2027, 9, 1)
	otherPhase.ServiceEndDate = timezone.NewDate(2028, 7, 31)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, otherPhase)
	}))
	t.Cleanup(func() { wipePhases(db, tenantID, otherPhase.Name) })

	reqMain := makeRequest(phaseID, token+"-main", "anna@example.test")
	reqOther := makeRequest(otherPhase.ID, token+"-other", "bert@example.test")
	for _, r := range []*enrollmentModels.Request{reqMain, reqOther} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}

	var list []*enrollmentModels.Request
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListAdmin(ctx, enrollmentModels.RequestListFilters{PhaseID: phaseID})
		return lErr
	}))

	for _, r := range list {
		assert.Equal(t, phaseID, r.PhaseID,
			"phase filter must exclude every other phase (saw request %d with phase %d)", r.ID, r.PhaseID)
	}
}

func TestRequestRepository_ListAdmin_ChildStatusFilter(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("listChildStatus")
	defer wipeRequests(db, tenantID, token)

	// Two requests, each with one child. One child is approved, the
	// other waitlisted. Filter on waitlisted → only one should come
	// back.
	rApproved := makeRequest(phaseID, token+"-approved", "a@example.test")
	rWaitlisted := makeRequest(phaseID, token+"-waitlisted", "w@example.test")
	for _, r := range []*enrollmentModels.Request{rApproved, rWaitlisted} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
	}
	insertRequestChild(t, db, tenantID, rApproved.ID, "Anna", "A", enrollmentModels.ChildStatusApproved)
	insertRequestChild(t, db, tenantID, rWaitlisted.ID, "Bert", "B", enrollmentModels.ChildStatusWaitlisted)

	var list []*enrollmentModels.Request
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = repo.ListAdmin(ctx, enrollmentModels.RequestListFilters{
			ChildStatus: enrollmentModels.ChildStatusWaitlisted,
		})
		return lErr
	}))

	seenWaitlisted := false
	for _, r := range list {
		if r.ID == rWaitlisted.ID {
			seenWaitlisted = true
		}
		if r.ID == rApproved.ID {
			t.Errorf("approved-only request must be excluded when filtering on waitlisted")
		}
	}
	assert.True(t, seenWaitlisted, "waitlisted request must appear when filter matches")
}

// --- FindActiveDuplicate -----------------------------------------------

func TestRequestRepository_FindActiveDuplicate_BlocksRepeatSubmission(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("dupe")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token+"-orig", "Anna@Example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))
	insertRequestChild(t, db, tenantID, req.ID, "  Lara  ", "Beispiel", enrollmentModels.ChildStatusSubmitted)

	// Re-submit attempt — same email (different case + whitespace),
	// same child name (different case + whitespace) → MUST match.
	var dupes []enrollmentModels.DuplicateChildKey
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, phaseID, "  anna@example.TEST  ",
			[]enrollmentModels.DuplicateChildKey{{FirstName: "LARA", LastName: " beispiel "}})
		return dErr
	})
	require.NoError(t, err)
	require.Len(t, dupes, 1, "case + whitespace normalization must catch the duplicate")
	assert.Equal(t, "lara", dupes[0].FirstName)
	assert.Equal(t, "beispiel", dupes[0].LastName)
}

func TestRequestRepository_FindActiveDuplicate_IgnoresWithdrawnAndRejected(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("dupeTerminal")
	defer wipeRequests(db, tenantID, token)

	// Two prior requests: one withdrawn, one rejected. A fresh
	// submission with the same name + same email must NOT match either
	// — the parent should be allowed to re-submit.
	for i, status := range []string{enrollmentModels.ChildStatusWithdrawn, enrollmentModels.ChildStatusRejected} {
		r := makeRequest(phaseID, fmt.Sprintf("%s-%d", token, i), "anna@example.test")
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
		insertRequestChild(t, db, tenantID, r.ID, "Lara", "Beispiel", status)
	}

	var dupes []enrollmentModels.DuplicateChildKey
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, phaseID, "anna@example.test",
			[]enrollmentModels.DuplicateChildKey{{FirstName: "Lara", LastName: "Beispiel"}})
		return dErr
	}))
	assert.Empty(t, dupes, "withdrawn + rejected statuses MUST NOT block re-submission")
}

func TestRequestRepository_FindActiveDuplicate_DifferentParentSameChildOK(t *testing.T) {
	// Same child name but different guardian email → not a duplicate.
	// Two unrelated families with kids of the same name must both be
	// able to enrol.
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("dupeOtherParent")
	defer wipeRequests(db, tenantID, token)

	r := makeRequest(phaseID, token+"-orig", "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))
	insertRequestChild(t, db, tenantID, r.ID, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted)

	var dupes []enrollmentModels.DuplicateChildKey
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, phaseID, "bert@example.test",
			[]enrollmentModels.DuplicateChildKey{{FirstName: "Lara", LastName: "Beispiel"}})
		return dErr
	}))
	assert.Empty(t, dupes, "different guardian email MUST NOT be flagged as duplicate")
}

func TestRequestRepository_FindActiveDuplicate_DifferentPhaseSameChildOK(t *testing.T) {
	// A child enrolled in phase A should be allowed to enrol in phase B.
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("dupeOtherPhase")
	defer wipeRequests(db, tenantID, token)

	r := makeRequest(phaseID, token+"-orig", "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))
	insertRequestChild(t, db, tenantID, r.ID, "Lara", "Beispiel", enrollmentModels.ChildStatusSubmitted)

	// Different phase id → fresh submission must be allowed.
	otherPhaseID := phaseID + 99_999_999 // synthetic, no row → 0 matches expected
	var dupes []enrollmentModels.DuplicateChildKey
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, otherPhaseID, "anna@example.test",
			[]enrollmentModels.DuplicateChildKey{{FirstName: "Lara", LastName: "Beispiel"}})
		return dErr
	}))
	assert.Empty(t, dupes, "different phase MUST NOT be flagged as duplicate")
}

func TestRequestRepository_FindActiveDuplicate_RejectsZeroPhase(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, dErr := repo.FindActiveDuplicate(ctx, 0, "x@y.z", nil)
		return dErr
	})
	require.Error(t, err)
}

func TestRequestRepository_FindActiveDuplicate_EmptyEmailIsNotADuplicate(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	var dupes []enrollmentModels.DuplicateChildKey
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, phaseID, "  ",
			[]enrollmentModels.DuplicateChildKey{{FirstName: "Lara", LastName: "Beispiel"}})
		return dErr
	})
	require.NoError(t, err)
	assert.Empty(t, dupes, "blank email short-circuits to empty result (no match possible)")
}

func TestRequestRepository_FindActiveDuplicate_EmptyChildListIsNotADuplicate(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	var dupes []enrollmentModels.DuplicateChildKey
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var dErr error
		dupes, dErr = repo.FindActiveDuplicate(ctx, phaseID, "a@b.c", nil)
		return dErr
	})
	require.NoError(t, err)
	assert.Empty(t, dupes)
}

// --- ExistsByPhaseID + ExistsBySchemaID --------------------------------

func TestRequestRepository_ExistsByPhaseID_TrueWhenReferenced(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("existsByPhase")
	defer wipeRequests(db, tenantID, token)

	req := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsByPhaseID(ctx, phaseID)
		return eErr
	})
	require.NoError(t, err)
	assert.True(t, exists, "phase referenced by a request must surface as exists=true")
}

func TestRequestRepository_ExistsByPhaseID_FalseWhenUnreferenced(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)
	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsByPhaseID(ctx, 9_999_999)
		return eErr
	})
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRequestRepository_ExistsBySchemaID_TrueWhenReferenced(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("existsBySchema")
	defer wipeRequests(db, tenantID, token)

	// Need a real schema row first.
	account := testpkg.CreateTestAccount(t, db, "req-exists-schema")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})
	schemaRepo := enrollmentRepo.NewFormSchemaRepository(db)
	schema := &enrollmentModels.FormSchema{
		Name: uniqueSchemaName("req-exists-schema"), Version: 1,
		Fields: validFields(), IsActive: true, CreatedBy: account.ID,
	}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return schemaRepo.Create(ctx, schema)
	}))
	t.Cleanup(func() { wipeSchemas(db, tenantID, account.ID) })

	req := makeRequest(phaseID, token, "anna@example.test")
	req.SchemaID = &schema.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, req)
	}))

	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsBySchemaID(ctx, schema.ID)
		return eErr
	})
	require.NoError(t, err)
	assert.True(t, exists, "schema referenced by a request must surface as exists=true")
}

func TestRequestRepository_ExistsBySchemaID_FalseWhenUnreferenced(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)
	var exists bool
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var eErr error
		exists, eErr = repo.ExistsBySchemaID(ctx, 9_999_999)
		return eErr
	})
	require.NoError(t, err)
	assert.False(t, exists)
}

// --- HasActiveRequestForMatchedStudent (#1663) -------------------------

// insertRequestChildMatched mirrors insertRequestChild but pins a
// matched_student_id — the existing-student re-enrollment signal that
// HasActiveRequestForMatchedStudent keys on.
func insertRequestChildMatched(t *testing.T, db *bun.DB, tenantID, requestID, matchedStudentID int64, status string) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO enrollment.request_children
		  (tenant_id, request_id, first_name, last_name, date_of_birth,
		   status, activation_mode, sort_order, custom_data, matched_student_id)
		VALUES (?, ?, 'Lara', 'Beispiel', '2018-04-15', ?, 'scheduled', 0, '{}'::jsonb, ?)
	`, tenantID, requestID, status, matchedStudentID).Exec(context.Background())
	require.NoError(t, err)
}

// createMatchTestStudent creates a real enrolled student (matched_student_id
// has a FK to users.students) and registers hard-delete cleanup.
func createMatchTestStudent(t *testing.T, db *bun.DB, tenantID int64) int64 {
	t.Helper()
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Lara", "Beispiel", "1a")
	t.Cleanup(func() {
		bg := context.Background()
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(bg)
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", student.PersonID).Exec(bg)
	})
	return student.ID
}

func TestRequestRepository_HasActiveRequestForMatchedStudent_TrueForActivePin(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("matchActive")
	defer wipeRequests(db, tenantID, token)

	studentID := createMatchTestStudent(t, db, tenantID)
	r := makeRequest(phaseID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.Create(ctx, r)
	}))
	insertRequestChildMatched(t, db, tenantID, r.ID, studentID, enrollmentModels.ChildStatusSubmitted)

	var has bool
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var hErr error
		has, hErr = repo.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID)
		return hErr
	}))
	assert.True(t, has, "an active request pinned to the student must be detected")

	// A different student id in the same phase is not a collision.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var hErr error
		has, hErr = repo.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID+1)
		return hErr
	}))
	assert.False(t, has, "an unrelated student id must not be flagged")
}

func TestRequestRepository_HasActiveRequestForMatchedStudent_IgnoresTerminalAndOtherPhase(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	token := uniqueToken("matchTerminal")
	defer wipeRequests(db, tenantID, token)

	studentID := createMatchTestStudent(t, db, tenantID)
	// Withdrawn + rejected pins for the same student must NOT block: those
	// requests are no longer live, so re-enrolling the student is fine.
	for i, status := range []string{enrollmentModels.ChildStatusWithdrawn, enrollmentModels.ChildStatusRejected} {
		r := makeRequest(phaseID, fmt.Sprintf("%s-%d", token, i), "anna@example.test")
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.Create(ctx, r)
		}))
		insertRequestChildMatched(t, db, tenantID, r.ID, studentID, status)
	}

	var has bool
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var hErr error
		has, hErr = repo.HasActiveRequestForMatchedStudent(ctx, phaseID, studentID)
		return hErr
	}))
	assert.False(t, has, "withdrawn + rejected pins must not block re-enrollment")

	// A different phase never collides with this phase's pins.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var hErr error
		has, hErr = repo.HasActiveRequestForMatchedStudent(ctx, phaseID+99_999_999, studentID)
		return hErr
	}))
	assert.False(t, has, "a different phase must not be flagged")
}

func TestRequestRepository_AcquireExistingStudentMatchLock_RejectsBadPhase(t *testing.T) {
	db, repo, tenantID, _ := setupRequestRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.AcquireExistingStudentMatchLock(ctx, 0)
	})
	require.Error(t, err, "a non-positive phase id is out of advisory-lock range")
}

func TestRequestRepository_AcquireExistingStudentMatchLock_SucceedsInTx(t *testing.T) {
	db, repo, tenantID, phaseID := setupRequestRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.AcquireExistingStudentMatchLock(ctx, phaseID)
	})
	require.NoError(t, err)
}
