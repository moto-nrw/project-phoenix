package enrollment_test

import (
	"context"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// setupRequestChildRepoTest spins up a tenant + a phase + a request
// row so children can be inserted against a real FK chain.
func setupRequestChildRepoTest(t *testing.T) (
	*bun.DB,
	*capability.Module,
	int64,
	int64,
	int64,
) {
	t.Helper()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)

	phaseRepo := enrollmentCompose.New()
	phaseName := uniquePhaseName("child-test")
	phase := makeOwnerEligibilityPhase(phaseName)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.InsertPhase(ctx, phase)
	}))

	requestRepo := enrollmentCompose.New()
	token := uniqueToken("child-test")
	req := makeOwnerRequest(phase.ID, token, "anna@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.InsertRequest(ctx, req)
	}))

	t.Cleanup(func() {
		wipeRequests(db, tenantID, token)
		wipePhases(db, tenantID, phaseName)
	})

	return db, enrollmentCompose.New(), tenantID, phase.ID, req.ID
}

func makeChild(requestID int64, firstName, lastName string) *capability.RequestChild {
	return &capability.RequestChild{
		RequestID:      requestID,
		FirstName:      firstName,
		LastName:       lastName,
		DateOfBirth:    capability.Date("2018-04-15"),
		Status:         enrollmentModels.ChildStatusSubmitted,
		ActivationMode: enrollmentModels.ChildActivationScheduled,
		CustomData:     []byte("{}"),
	}
}

// --- ListByRequestIDs (batch loader for the phase export) --------------

func TestRequestChildRepository_ListByRequestIDs_BatchesAcrossRequests(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, phaseID, requestID := setupRequestChildRepoTest(t)

	// Three children under the first request (sort order shuffled).
	for i, name := range []string{"Cara", "Aino", "Bea"} {
		c := makeChild(requestID, name, "Eins")
		c.SortOrder = i
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertChild(ctx, c)
		}))
	}

	// A second request in the same phase, with one child.
	reqRepo := enrollmentCompose.New()
	token2 := uniqueToken("child-test-2")
	req2 := makeOwnerRequest(phaseID, token2, "bea@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return reqRepo.InsertRequest(ctx, req2)
	}))
	t.Cleanup(func() { wipeRequests(db, tenantID, token2) })
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, makeChild(req2.ID, "Solo", "Zwei"))
	}))

	var list []*capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenForRequests(ctx, []int64{requestID, req2.ID})
		return lErr
	}))

	require.Len(t, list, 4, "all children across both requests in one query")
	// Ordered by request_id, then sort_order: first request's children
	// come back in sort_order (Cara, Aino, Bea), then the second request.
	assert.Equal(t, requestID, list[0].RequestID)
	assert.Equal(t, "Cara", list[0].FirstName)
	assert.Equal(t, "Aino", list[1].FirstName)
	assert.Equal(t, "Bea", list[2].FirstName)
	assert.Equal(t, req2.ID, list[3].RequestID)
}

func TestRequestChildRepository_ListByRequestIDs_EmptyInputShortCircuits(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	var list []*capability.RequestChild
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenForRequests(ctx, nil)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list, "empty id list must not run an IN () query")
}

// --- Create + FindByID -------------------------------------------------

func TestRequestChildRepository_Create_PersistsAndReturnsID(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	child := makeChild(requestID, "Lara", "Beispiel")
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	})
	require.NoError(t, err)
	assert.NotZero(t, child.ID)
	assert.Equal(t, tenantID, child.TenantID)
}

func TestRequestChildRepository_FindByID_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	child := makeChild(requestID, "Lara", "Beispiel")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	var got *capability.RequestChild
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return fbErr
	})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, child.ID, got.ID)
	assert.Equal(t, "Lara", got.FirstName)
}

// --- ListByRequestID ---------------------------------------------------

func TestRequestChildRepository_ListByRequestID_OrdersBySortOrder(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	// Insert children out of order to verify sort_order ordering.
	c1 := makeChild(requestID, "Bert", "B")
	c1.SortOrder = 2
	c2 := makeChild(requestID, "Anna", "A")
	c2.SortOrder = 0
	c3 := makeChild(requestID, "Cara", "C")
	c3.SortOrder = 1

	for _, c := range []*capability.RequestChild{c1, c2, c3} {
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertChild(ctx, c)
		}))
	}

	var list []*capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenForRequest(ctx, requestID, false)
		return lErr
	}))
	require.Len(t, list, 3)
	assert.Equal(t, "Anna", list[0].FirstName)
	assert.Equal(t, "Cara", list[1].FirstName)
	assert.Equal(t, "Bert", list[2].FirstName)
}

func TestRequestChildRepository_ListByRequestID_EmptyResultNoError(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)

	var list []*capability.RequestChild
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenForRequest(ctx, 9_999_999, false)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list)
}

// --- UpdateStatus ------------------------------------------------------

func TestRequestChildRepository_UpdateStatus_StampsReviewerAndReason(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	// Need a real reviewer account so the FK passes.
	account := testpkg.CreateTestAccount(t, db, "reviewer")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	child := makeChild(requestID, "Lara", "B")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	reason := "Kapazität überschritten"
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusWaitlisted, &reason, account.ID)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return fbErr
	}))
	assert.Equal(t, enrollmentModels.ChildStatusWaitlisted, got.Status)
	require.NotNil(t, got.StatusReason)
	assert.Equal(t, reason, *got.StatusReason)
	require.NotNil(t, got.ReviewedAt, "reviewed_at must be stamped")
	require.NotNil(t, got.ReviewedBy, "reviewed_by must be set when reviewer > 0")
	assert.Equal(t, account.ID, *got.ReviewedBy)
}

func TestRequestChildRepository_UpdateStatus_ParentInitiatedNullsReviewer(t *testing.T) {
	t.Parallel()

	// reviewedBy=0 path — parent self-withdraw shouldn't trip the
	// auth.accounts FK. The repo must explicitly set reviewed_by=NULL.
	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	child := makeChild(requestID, "Lara", "B")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, child.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return fbErr
	}))
	assert.Equal(t, enrollmentModels.ChildStatusWithdrawn, got.Status)
	assert.Nil(t, got.ReviewedBy, "parent-initiated transition must set reviewed_by=NULL")
	assert.Nil(t, got.StatusReason, "nil reason must round-trip as NULL")
}

func TestRequestChildRepository_UpdateStatus_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, 9_999_999, enrollmentModels.ChildStatusApproved, nil, 0)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- LinkCreatedStudent ------------------------------------------------

func TestRequestChildRepository_LinkCreatedStudent_StampsBackLink(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	// Need a real student row for the FK on created_student_id.
	student := testpkg.CreateTestStudent(t, db, "Lara", "Beispiel", "1a")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("users.students").Where("id = ?", student.ID).Exec(context.Background())
		_, _ = db.NewDelete().TableExpr("users.persons").Where("id = ?", student.PersonID).Exec(context.Background())
	})

	child := makeChild(requestID, "Lara", "Beispiel")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().LinkCreatedStudent(ctx, child.ID, student.ID)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return fbErr
	}))
	require.NotNil(t, got.CreatedStudentID)
	assert.Equal(t, student.ID, *got.CreatedStudentID)
}

func TestRequestChildRepository_LinkCreatedStudent_MissingChildErrors(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().LinkCreatedStudent(ctx, 9_999_999, 12345)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRequestChildRepository_UpdateActivationPlan_StampsImmediate(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	child := makeChild(requestID, "Immo", "Diate")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildActivationPlan(ctx, child.ID, enrollmentModels.ChildActivationImmediate, nil)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var findErr error
		got, findErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return findErr
	}))
	assert.Equal(t, enrollmentModels.ChildActivationImmediate, got.ActivationMode)
	assert.Nil(t, got.ActivateOn)
}

func TestRequestChildRepository_UpdateActivationPlan_StampsScheduledDate(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	child := makeChild(requestID, "Sched", "Uled")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, child)
	}))

	activateOn := capability.Date("2027-09-01")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildActivationPlan(ctx, child.ID, enrollmentModels.ChildActivationScheduled, &activateOn)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var findErr error
		got, findErr = enrollmentCompose.New().ChildByID(ctx, child.ID)
		return findErr
	}))
	assert.Equal(t, enrollmentModels.ChildActivationScheduled, got.ActivationMode)
	require.NotNil(t, got.ActivateOn)
	assert.Equal(t, string(activateOn), string(*got.ActivateOn))
}

func TestRequestChildRepository_UpdateActivationPlan_MissingChildErrors(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildActivationPlan(ctx, 9_999_999, enrollmentModels.ChildActivationImmediate, nil)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// --- ListByPhaseAndStatuses --------------------------------------------

func TestRequestChildRepository_ListByPhaseAndStatuses_Filters(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, phaseID, requestID := setupRequestChildRepoTest(t)

	// Three children: two pending_admin_review, one approved.
	for _, status := range []string{
		enrollmentModels.ChildStatusPendingAdminReview,
		enrollmentModels.ChildStatusPendingAdminReview,
		enrollmentModels.ChildStatusApproved,
	} {
		c := makeChild(requestID, "Kid", status)
		c.Status = status
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertChild(ctx, c)
		}))
	}

	var list []*capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenByPhaseStatuses(ctx, phaseID,
			[]string{enrollmentModels.ChildStatusPendingAdminReview})
		return lErr
	}))
	require.Len(t, list, 2, "filter must return only pending_admin_review rows")
	for _, c := range list {
		assert.Equal(t, enrollmentModels.ChildStatusPendingAdminReview, c.Status)
	}
}

func TestRequestChildRepository_ListByPhaseAndStatuses_EmptyStatusesShortCircuit(t *testing.T) {
	t.Parallel()

	db, _, tenantID, phaseID, _ := setupRequestChildRepoTest(t)
	var list []*capability.RequestChild
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenByPhaseStatuses(ctx, phaseID, nil)
		return lErr
	})
	require.NoError(t, err)
	assert.Empty(t, list, "empty status list must short-circuit to empty result, not return all rows")
}

func TestRequestChildRepository_ListByPhaseAndStatuses_RejectsZeroPhase(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, lErr := enrollmentCompose.New().ChildrenByPhaseStatuses(ctx, 0,
			[]string{enrollmentModels.ChildStatusApproved})
		return lErr
	})
	require.Error(t, err)
}

// --- BulkUpdateStatusByPhaseAndStatus -----------------------------------

func TestRequestChildRepository_BulkUpdateStatusByPhaseAndStatus_TransitionsAll(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, phaseID, requestID := setupRequestChildRepoTest(t)

	// Two children in pending_renewal + one in auto_renewed.
	for _, status := range []string{
		enrollmentModels.ChildStatusPendingRenewal,
		enrollmentModels.ChildStatusPendingRenewal,
		enrollmentModels.ChildStatusAutoRenewed,
	} {
		c := makeChild(requestID, "K", status)
		c.Status = status
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			return repo.InsertChild(ctx, c)
		}))
	}

	// Transition pending_renewal → withdrawn. Should affect 2 rows.
	var n int
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var bErr error
		n, bErr = enrollmentCompose.New().TransitionPhaseChildren(ctx, phaseID,
			enrollmentModels.ChildStatusPendingRenewal,
			enrollmentModels.ChildStatusWithdrawn)
		return bErr
	}))
	assert.Equal(t, 2, n, "bulk update must report rows-affected count")

	// auto_renewed must be untouched.
	var list []*capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var lErr error
		list, lErr = enrollmentCompose.New().ChildrenByPhaseStatuses(ctx, phaseID,
			[]string{enrollmentModels.ChildStatusAutoRenewed})
		return lErr
	}))
	assert.Len(t, list, 1, "auto_renewed rows MUST NOT be touched by a pending_renewal transition")
}

func TestRequestChildRepository_BulkUpdateStatusByPhaseAndStatus_RejectsZeroPhase(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, bErr := enrollmentCompose.New().TransitionPhaseChildren(ctx, 0,
			enrollmentModels.ChildStatusPendingRenewal,
			enrollmentModels.ChildStatusWithdrawn)
		return bErr
	})
	require.Error(t, err)
}

func TestRequestChildRepository_BulkUpdateStatusByPhaseAndStatus_RejectsBlankStatuses(t *testing.T) {
	t.Parallel()

	db, _, tenantID, phaseID, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, bErr := enrollmentCompose.New().TransitionPhaseChildren(ctx, phaseID, "", "withdrawn")
		return bErr
	})
	require.Error(t, err)
	err = runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, bErr := enrollmentCompose.New().TransitionPhaseChildren(ctx, phaseID, "pending_renewal", "")
		return bErr
	})
	require.Error(t, err)
}

func TestRequestChildRepository_BulkUpdateStatusByPhaseAndStatus_ZeroAffectedNoError(t *testing.T) {
	t.Parallel()

	db, _, tenantID, phaseID, _ := setupRequestChildRepoTest(t)
	var n int
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var bErr error
		n, bErr = enrollmentCompose.New().TransitionPhaseChildren(ctx, phaseID,
			enrollmentModels.ChildStatusPendingRenewal,
			enrollmentModels.ChildStatusWithdrawn)
		return bErr
	})
	require.NoError(t, err)
	assert.Equal(t, 0, n, "zero rows affected returns (0, nil) — not an error")
}

// --- UpdateRolloverReview ----------------------------------------------

func TestRequestChildRepository_UpdateRolloverReview_HappyPath(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	account := testpkg.CreateTestAccount(t, db, "rolloverreviewer")
	t.Cleanup(func() {
		_, _ = db.NewDelete().TableExpr("auth.accounts").
			Where("id = ?", account.ID).Exec(context.Background())
	})

	// Start in pending_admin_review with a review_reason set.
	c := makeChild(requestID, "Lara", "B")
	c.Status = enrollmentModels.ChildStatusPendingAdminReview
	reason := "Grade above max"
	c.ReviewReason = &reason
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, c)
	}))

	// "Behalten" with a grade override.
	newGrade := int16(2)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().ReviewRolloverChild(ctx, c.ID, enrollmentModels.ChildStatusSubmitted, nil, &newGrade, account.ID)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, c.ID)
		return fbErr
	}))
	assert.Equal(t, enrollmentModels.ChildStatusSubmitted, got.Status)
	require.NotNil(t, got.TargetGradeLevel)
	assert.Equal(t, int16(2), *got.TargetGradeLevel)
	assert.Nil(t, got.ReviewReason, "review_reason MUST be cleared once the row leaves admin review")
	require.NotNil(t, got.ReviewedBy)
	assert.Equal(t, account.ID, *got.ReviewedBy)
}

func TestRequestChildRepository_UpdateRolloverReview_NilGradePreservesExisting(t *testing.T) {
	t.Parallel()

	db, repo, tenantID, _, requestID := setupRequestChildRepoTest(t)

	// Child with a grade level already set.
	existingGrade := int16(3)
	c := makeChild(requestID, "Lara", "B")
	c.Status = enrollmentModels.ChildStatusPendingAdminReview
	c.TargetGradeLevel = &existingGrade
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return repo.InsertChild(ctx, c)
	}))

	// Decide without overriding grade.
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().ReviewRolloverChild(ctx, c.ID, enrollmentModels.ChildStatusSubmitted, nil, nil, 0)
	}))

	var got *capability.RequestChild
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		var fbErr error
		got, fbErr = enrollmentCompose.New().ChildByID(ctx, c.ID)
		return fbErr
	}))
	require.NotNil(t, got.TargetGradeLevel)
	assert.Equal(t, int16(3), *got.TargetGradeLevel, "nil newGradeLevel must NOT touch the existing target_grade_level")
}

func TestRequestChildRepository_UpdateRolloverReview_MissingIDErrors(t *testing.T) {
	t.Parallel()

	db, _, tenantID, _, _ := setupRequestChildRepoTest(t)
	err := runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().ReviewRolloverChild(ctx, 9_999_999,
			enrollmentModels.ChildStatusSubmitted, nil, nil, 0)
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}
