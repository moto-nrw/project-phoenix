package integration

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Both change-request tables are row-level secured for the request role: a
// raw query without an application tenant predicate must still see only the
// current tenant's rows, and a raw update must not reach a foreign row.
func TestChangeRequestTablesAreRowLevelSecured(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	var tenants, changeIDs, messageIDs []int64
	for _, name := range []string{"first", "second"} {
		t.Run(name, func(t *testing.T) {
			testpkg.OwnTenant(t)
			ctx := testpkg.Ctx(t)
			phase := testpkg.CreateTestEnrollmentPhase(t, db)
			request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: "rls-" + name + t.Name()}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertRequest, request))
			row := &enrollment.ChangeRequest{RequestID: request.ID}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequest, row))
			message := &enrollment.ChangeRequestMessage{ChangeRequestID: row.ID, AuthorType: "parent", Body: name}
			require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequestMessage, message))
			tenants = append(tenants, testpkg.Tenant(t))
			changeIDs = append(changeIDs, row.ID)
			messageIDs = append(messageIDs, message.ID)
		})
	}
	for index, tenantID := range tenants {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenantID)
		foreign := 1 - index
		err := testpkg.WithTenantTx(t, ctx, db, tenantID, func(txCtx context.Context, tx bun.Tx) error {
			var bypass bool
			require.NoError(t, tx.NewRaw("SELECT rolsuper OR rolbypassrls FROM pg_roles WHERE rolname = current_user").Scan(txCtx, &bypass))
			require.False(t, bypass, "RLS evidence must use a non-bypass role")
			var ids []int64
			require.NoError(t, tx.NewRaw("SELECT id FROM enrollment.change_requests WHERE id IN (?, ?)", changeIDs[index], changeIDs[foreign]).Scan(txCtx, &ids))
			require.Equal(t, []int64{changeIDs[index]}, ids, "RLS must filter change requests without an application tenant predicate")
			ids = nil
			require.NoError(t, tx.NewRaw("SELECT id FROM enrollment.change_request_messages WHERE id IN (?, ?)", messageIDs[index], messageIDs[foreign]).Scan(txCtx, &ids))
			require.Equal(t, []int64{messageIDs[index]}, ids, "RLS must filter dialogue messages without an application tenant predicate")
			for _, statement := range []string{
				"UPDATE enrollment.change_requests SET status = 'rejected' WHERE id = ?",
				"UPDATE enrollment.change_request_messages SET body = 'must not change' WHERE id = ?",
			} {
				id := changeIDs[foreign]
				if strings.Contains(statement, "messages") {
					id = messageIDs[foreign]
				}
				result, err := tx.NewRaw(statement, id).Exec(txCtx)
				require.NoError(t, err)
				affected, err := result.RowsAffected()
				require.NoError(t, err)
				require.Zero(t, affected, "RLS must hide the foreign row from writes")
			}
			return nil
		})
		require.NoError(t, err)
	}
	for index, tenantID := range tenants {
		ctx := testpkg.ContextForTenant(testpkg.Ctx(t), tenantID)
		stored, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestByID, changeIDs[index])
		require.NoError(t, err)
		require.Equal(t, "pending_review", stored.Status)
		messages, err := testpkg.TenantQuery2(t, ctx, db, owner.ChangeRequestMessages, []int64{changeIDs[index]}, true)
		require.NoError(t, err)
		require.Len(t, messages, 1)
		require.NotEqual(t, "must not change", messages[0].Body)
	}
}

// The change-request and dialogue reads must surface store failures as
// errors with their stable operation prefix, never as an empty result: a
// review queue that silently reads as empty would hide open requests.
func TestChangeRequestReadsPreserveStoreFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	owner := enrollmentCompose.New()
	ctx := testpkg.Ctx(t)
	phase := testpkg.CreateTestEnrollmentPhase(t, db)
	request := &enrollment.Request{PhaseID: phase.ID, GuardianFirstName: "First", GuardianLastName: "Last", GuardianEmail: "guardian@example.test", StatusToken: "failure-" + t.Name()}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertRequest, request))
	row := &enrollment.ChangeRequest{RequestID: request.ID}
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequest, row))
	require.NoError(t, testpkg.TenantCommand1(t, ctx, db, owner.InsertChangeRequestMessage, &enrollment.ChangeRequestMessage{ChangeRequestID: row.ID, AuthorType: "parent", Body: "hello"}))

	// Without a tenant there is no transaction to bind to; the owner refuses
	// instead of reading from an unscoped pool.
	_, err := testpkg.TenantQuery1(t, context.Background(), db, owner.ChangeRequestByID, row.ID)
	require.Error(t, err)
	_, err = testpkg.TenantQuery2(t, context.Background(), db, owner.ChangeRequestMessages, []int64{row.ID}, true)
	require.Error(t, err)

	reads := []struct {
		name   string
		prefix string
		read   func(context.Context) error
	}{
		{"by id", "failed to find enrollment change request", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestByID, row.ID)
			return err
		}},
		{"for request", "failed to list enrollment change requests by request", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestsForRequest, request.ID)
			return err
		}},
		{"admin list", "failed to list enrollment change requests", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery1(t, ctx, db, owner.ListChangeRequests, enrollment.ChangeRequestListFilters{RequestID: request.ID})
			return err
		}},
		{"review list", "failed to list enrollment change requests for review", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestsForReview, enrollment.ChangeRequestReviewFilters{Statuses: []string{"pending_review"}, Limit: 5})
			return err
		}},
		{"review count", "failed to count enrollment change requests for review", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery1(t, ctx, db, owner.CountChangeRequestsForReview, []string{"pending_review"})
			return err
		}},
		{"messages", "failed to list enrollment change request messages", func(ctx context.Context) error {
			_, err := testpkg.TenantQuery2(t, ctx, db, owner.ChangeRequestMessages, []int64{row.ID}, true)
			return err
		}},
	}
	for _, read := range reads {
		t.Run(read.name, func(t *testing.T) {
			// A cancelled statement is the cheapest real driver failure; each
			// read runs in its own transaction because the failure poisons it.
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
				cancelled, cancel := context.WithCancel(txCtx)
				cancel()
				return read.read(cancelled)
			})
			require.ErrorIs(t, err, context.Canceled)
			require.ErrorContains(t, err, read.prefix)
		})
	}

	// The failures above left the rows and the healthy path untouched.
	stored, err := testpkg.TenantQuery1(t, ctx, db, owner.ChangeRequestByID, row.ID)
	require.NoError(t, err)
	require.Equal(t, row.ID, stored.ID)
	messages, err := testpkg.TenantQuery2(t, ctx, db, owner.ChangeRequestMessages, []int64{row.ID}, true)
	require.NoError(t, err)
	require.Len(t, messages, 1)
}
