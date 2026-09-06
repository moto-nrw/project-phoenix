package integration

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestChangeRequestReviewCountIsTenantAndStatusScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	var tenants []int64
	var pendingIDs []int64
	var requestIDs []int64
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: "review-count-" + name}
			require.NoError(t, owner.InsertRequest(ctx, request))
			requestIDs = append(requestIDs, request.ID)
			for _, status := range []string{"pending_review", "rejected"} {
				row := &enrollment.ChangeRequest{RequestID: request.ID, Status: status}
				failure := errors.New("after change request insert")
				err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
					if err := owner.InsertChangeRequest(txCtx, row); err != nil {
						return err
					}
					return failure
				})
				require.ErrorIs(t, err, failure)
				count, err := owner.CountChangeRequestsForReview(ctx, []string{status})
				require.NoError(t, err)
				require.Zero(t, count)
				require.NoError(t, owner.InsertChangeRequest(ctx, row))
				require.Positive(t, row.ID)
				require.Equal(t, testpkg.Tenant(t), row.TenantID)
				require.False(t, row.CreatedAt.IsZero())
				require.Equal(t, "parent", row.Origin)
				require.JSONEq(t, "{}", string(row.BaseSnapshot))
				require.JSONEq(t, "{}", string(row.ProposedSnapshot))
				require.JSONEq(t, "{}", string(row.Diff))
				if status == "pending_review" {
					pendingIDs = append(pendingIDs, row.ID)
				}
			}
			tenants = append(tenants, testpkg.Tenant(t))
		})
	}
	for _, tenantID := range tenants {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenantID)
		for _, statuses := range [][]string{nil, {"pending_review"}, {"pending_review", "rejected"}} {
			count, err := owner.CountChangeRequestsForReview(ctx, statuses)
			require.NoError(t, err)
			require.Equal(t, len(statuses), count)
		}
	}
	ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenants[0])
	storedRequest, err := owner.ChangeRequestByID(ctx, pendingIDs[0])
	require.NoError(t, err)
	require.Equal(t, tenants[0], storedRequest.TenantID)
	require.JSONEq(t, "{}", string(storedRequest.Diff))
	_, err = owner.ChangeRequestByID(ctx, pendingIDs[1])
	require.ErrorContains(t, err, "not found")
	_, err = owner.ChangeRequestByIDForUpdate(ctx, pendingIDs[1])
	require.ErrorContains(t, err, "not found")
	foreignRows, err := owner.ChangeRequestsForRequest(ctx, requestIDs[1])
	require.NoError(t, err)
	require.Empty(t, foreignRows)
	foreignRows, err = owner.OpenChangeRequestsForRequestForUpdate(ctx, requestIDs[1])
	require.NoError(t, err)
	require.Empty(t, foreignRows)
	adminRows, err := owner.ListChangeRequests(ctx, enrollment.ChangeRequestListFilters{})
	require.NoError(t, err)
	require.Len(t, adminRows, 2)
	for _, row := range adminRows {
		require.Equal(t, tenants[0], row.TenantID)
	}
	reviewRows, err := owner.ChangeRequestsForReview(ctx, enrollment.ChangeRequestReviewFilters{Statuses: []string{"pending_review"}, Limit: 10})
	require.NoError(t, err)
	require.Len(t, reviewRows, 1)
	require.Equal(t, pendingIDs[0], reviewRows[0].ID)
	require.Error(t, owner.InsertChangeRequest(ctx, &enrollment.ChangeRequest{RequestID: requestIDs[1]}))
	failure := errors.New("after change request update")
	for _, review := range []bool{false, true} {
		err := testpkg.WithTenantTx(t, ctx, db, tenants[0], func(txCtx context.Context, _ bun.Tx) error {
			if review {
				note := "rolled back review"
				if err := owner.MarkChangeRequestReviewed(txCtx, pendingIDs[0], "rejected", &note, 0, time.Now()); err != nil {
					return err
				}
			} else {
				if err := owner.SetChangeRequestStatus(txCtx, pendingIDs[0], "needs_parent_response"); err != nil {
					return err
				}
			}
			return failure
		})
		require.ErrorIs(t, err, failure)
		var stored struct {
			Status              string
			AdminDecisionNote   *string
			ReviewedAt          *time.Time
			ReviewedByAccountID *int64
		}
		require.NoError(t, db.NewRaw(`SELECT status, admin_decision_note, reviewed_at, reviewed_by_account_id FROM enrollment.change_requests WHERE tenant_id = ? AND id = ?`, tenants[0], pendingIDs[0]).Scan(ctx, &stored))
		require.Equal(t, "pending_review", stored.Status)
		require.Nil(t, stored.AdminDecisionNote)
		require.Nil(t, stored.ReviewedAt)
		require.Nil(t, stored.ReviewedByAccountID)
	}
	require.ErrorContains(t, owner.SetChangeRequestStatus(ctx, pendingIDs[1], "rejected"), "not found")
	require.ErrorContains(t, owner.MarkChangeRequestReviewed(ctx, pendingIDs[1], "rejected", nil, 0, time.Now()), "not found")
	foreignMessage := &enrollment.ChangeRequestMessage{ChangeRequestID: pendingIDs[1], AuthorType: "parent", Body: "foreign"}
	require.Error(t, owner.InsertChangeRequestMessage(ctx, foreignMessage))
	message := &enrollment.ChangeRequestMessage{ChangeRequestID: pendingIDs[0], AuthorType: "staff", Body: "local", InternalOnly: true}
	err = testpkg.WithTenantTx(t, ctx, db, tenants[0], func(txCtx context.Context, _ bun.Tx) error {
		if err := owner.InsertChangeRequestMessage(txCtx, message); err != nil {
			return err
		}
		return failure
	})
	require.ErrorIs(t, err, failure)
	rolledBack, err := owner.ChangeRequestMessages(ctx, []int64{pendingIDs[0]}, true)
	require.NoError(t, err)
	require.Empty(t, rolledBack)
	message.ID = 0
	require.NoError(t, owner.InsertChangeRequestMessage(ctx, message))
	require.Positive(t, message.ID)
	require.Equal(t, tenants[0], message.TenantID)
	require.False(t, message.CreatedAt.IsZero())
	visible, err := owner.ChangeRequestMessages(ctx, pendingIDs, false)
	require.NoError(t, err)
	require.Empty(t, visible)
	internal, err := owner.ChangeRequestMessages(ctx, pendingIDs, true)
	require.NoError(t, err)
	require.Len(t, internal, 1)
	require.Equal(t, message.ID, internal[0].ID)
	require.Equal(t, message.Body, internal[0].Body)
	foreignMessages, err := owner.ChangeRequestMessages(testpkg.ContextForTenant(testpkg.Ctx(t), tenants[1]), pendingIDs, true)
	require.NoError(t, err)
	require.Empty(t, foreignMessages)
	require.NoError(t, owner.SetChangeRequestStatus(ctx, pendingIDs[0], "needs_parent_response"))
	note := "review fixture"
	at := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	require.NoError(t, owner.MarkChangeRequestReviewed(ctx, pendingIDs[0], "rejected", &note, 0, at))
	count, err := owner.CountChangeRequestsForReview(ctx, []string{"rejected"})
	require.NoError(t, err)
	require.Equal(t, 2, count)
	foreignCtx := testpkg.ContextForTenant(testpkg.Ctx(t), tenants[1])
	count, err = owner.CountChangeRequestsForReview(foreignCtx, []string{"pending_review"})
	require.NoError(t, err)
	require.Equal(t, 1, count)
}
