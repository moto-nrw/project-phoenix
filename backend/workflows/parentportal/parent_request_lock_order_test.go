package parentportal_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	parentService "github.com/moto-nrw/project-phoenix/workflows/parentportal"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestParentMasterDataEditLocksChildBeforeRequest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, operation := range []string{"edit", "delete request", "delete child"} {
		t.Run(operation, func(t *testing.T) {
			svc, _, _ := buildEditableMasterDataService(t)
			chain := testpkg.CreateTestParentGuardianChain(t, db)
			ctx, cancel := context.WithTimeout(testpkg.Ctx(t), 20*time.Second)
			defer cancel()
			rows, err := svc.SubmitMasterDataChangeRequest(ctx, chain.AccountID, chain.StudentID, []parentService.MasterDataFieldChange{
				{Target: parentService.DataChangeTargetPerson, FieldKey: "first_name", Value: json.RawMessage(`"Maximilian"`)},
			}, nil)
			require.NoError(t, err)
			require.Len(t, rows, 1)
			people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
			done := make(chan error, 1)
			err = testpkg.WithTenantTx(t, ctx, db, chain.TenantID, func(lockCtx context.Context, tx bun.Tx) error {
				if err := people.LockStudent(lockCtx, chain.StudentID); err != nil {
					return err
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(lockCtx, &pid))
				go func() {
					_, err := svc.EditMasterDataRequest(ctx, chain.AccountID, chain.StudentID, rows[0].ID, json.RawMessage(`"Maxi"`), "")
					done <- err
				}()
				require.Eventually(t, func() bool {
					var waiting bool
					err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(lockCtx, &waiting)
					return err == nil && waiting
				}, 5*time.Second, 10*time.Millisecond, "parent edit must wait on the child")
				var id int64
				if err := tx.NewSelect().TableExpr("users.student_data_change_requests").Column("id").Where("id = ?", rows[0].ID).For("UPDATE NOWAIT").Scan(lockCtx, &id); err != nil {
					return err
				}
				if operation == "delete request" {
					_, err := tx.NewDelete().TableExpr("users.student_data_change_requests").Where("id = ?", id).Exec(lockCtx)
					return err
				}
				if operation == "delete child" {
					_, err := people.DeleteStudent(lockCtx, chain.StudentID)
					return err
				}
				return nil
			})
			var editErr error
			select {
			case editErr = <-done:
			case <-ctx.Done():
				t.Fatal("parent edit did not finish")
			}
			require.NoError(t, err, "the edit must not hold the request while waiting for its child")
			if operation == "edit" {
				require.NoError(t, editErr)
			} else {
				require.ErrorIs(t, editErr, parentService.ErrMasterDataRequestNotFound)
			}
		})
	}
}
