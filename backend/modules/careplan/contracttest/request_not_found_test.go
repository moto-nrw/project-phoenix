package contracttest_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func TestFamilyRequestNotFound(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	f := newRequestLockFixture(t, db)
	ctx := f.care.staffCtx(f.care.staffAccount)
	for _, kind := range []string{"care", "excused", "master"} {
		t.Run(kind, func(t *testing.T) {
			var missingID int64
			require.NoError(t, db.NewRaw("SELECT nextval(pg_get_serial_sequence(?, 'id'))", f.table(kind)).Scan(ctx, &missingID))
			var foreignID int64
			t.Run("foreign fixture", func(t *testing.T) {
				testpkg.OwnTenant(t)
				foreignID = newRequestLockFixture(t, db).id(kind)
			})
			for name, id := range map[string]int64{"missing": missingID, "foreign": foreignID} {
				t.Run(name, func(t *testing.T) {
					for _, operation := range requestErrorOperations(kind) {
						t.Run(operation, func(t *testing.T) {
							err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
								return f.call(kind, operation, id)(txCtx)
							})
							require.ErrorIs(t, err, requestNotFound(kind))
						})
					}
				})
			}
		})
	}
}

// Deleting the request while review waits on that row preserves the old
// not-found outcome both before and after the child-first correction.
func TestFamilyRequestDeletedWhileWaiting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, kind := range []string{"care", "excused", "master"} {
		for _, operation := range requestErrorOperations(kind) {
			t.Run(kind+"/"+operation, func(t *testing.T) {
				f := newRequestLockFixture(t, db)
				if operation == "correct" {
					_, err := db.NewUpdate().TableExpr(f.table(kind)).Set("status = 'rejected'").Where("id = ?", f.id(kind)).Exec(t.Context())
					require.NoError(t, err)
					if kind == "care" {
						_, err = db.NewUpdate().TableExpr(f.table(kind)).Set("request_kind = 'pickup_change'").Where("id = ?", f.id(kind)).Exec(t.Context())
						require.NoError(t, err)
					}
				}
				ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
				defer cancel()
				done := make(chan error, 1)
				err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
					var id, pid int64
					if err := tx.NewSelect().TableExpr(f.table(kind)).Column("id").Where("id = ?", f.id(kind)).For("UPDATE").Scan(deleteCtx, &id); err != nil {
						return err
					}
					require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
					go func() {
						done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return f.call(kind, operation, id)(writeCtx) })
					}()
					waitForRequestWriter(t, deleteCtx, tx, int(pid))
					_, err := tx.NewDelete().TableExpr(f.table(kind)).Where("id = ?", id).Exec(deleteCtx)
					return err
				})
				require.NoError(t, err)
				require.ErrorIs(t, awaitRequestWriter(t, ctx, done), requestNotFound(kind))
			})
		}
	}
}

// The new preliminary read can also lose the child and its cascading requests
// while waiting. This must not leak the directory's generic/database error.
func TestFamilyRequestChildDeletedWhileWaiting(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, kind := range []string{"care", "excused", "master"} {
		t.Run(kind, func(t *testing.T) {
			f := newRequestLockFixture(t, db)
			ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
			defer cancel()
			people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
			done := make(chan error, 1)
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
				if err := people.LockStudent(deleteCtx, f.care.chain.StudentID); err != nil {
					return err
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
				go func() {
					done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return f.call(kind, "reject", f.id(kind))(writeCtx) })
				}()
				waitForRequestWriter(t, deleteCtx, tx, pid)
				_, err := people.DeleteStudent(deleteCtx, f.care.chain.StudentID)
				return err
			})
			require.NoError(t, err)
			require.ErrorIs(t, awaitRequestWriter(t, ctx, done), requestNotFound(kind))
		})
	}
}

func TestBulkRequestChildDeletedWhileWaitingPreservesRace(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	f := newRequestLockFixture(t, db)
	ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
	defer cancel()
	write := f.writer(t, ctx, "bulk")
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	done := make(chan error, 1)
	err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(deleteCtx context.Context, tx bun.Tx) error {
		if err := people.LockStudent(deleteCtx, f.care.chain.StudentID); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(deleteCtx, &pid))
		go func() {
			done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return write(writeCtx) })
		}()
		waitForRequestWriter(t, deleteCtx, tx, pid)
		_, err := people.DeleteStudent(deleteCtx, f.care.chain.StudentID)
		return err
	})
	require.NoError(t, err)
	require.ErrorIs(t, awaitRequestWriter(t, ctx, done), parentrequests.ErrStale)
}

func requestErrorOperations(kind string) []string {
	if kind == "master" {
		return []string{"approve", "reject", "correct", "conflict"}
	}
	return []string{"approve", "reject", "mark done", "correct", "conflict"}
}

func requestNotFound(kind string) error {
	switch kind {
	case "care":
		return careplan.ErrCareScheduleRequestNotFound
	case "excused":
		return careplan.ErrExcusedRequestNotFound
	default:
		return masterdatarequests.ErrReviewNotFound
	}
}

func (f requestLockFixture) table(kind string) string {
	switch kind {
	case "care":
		return "schedule.care_schedule_change_requests"
	case "excused":
		return "active.excused_absence_requests"
	default:
		return "users.student_data_change_requests"
	}
}

func (f requestLockFixture) id(kind string) int64 {
	switch kind {
	case "care":
		return f.careRow.ID
	case "excused":
		return f.excusedRow.ID
	default:
		return f.masterRow.ID
	}
}

func (f requestLockFixture) call(kind, operation string, id int64, versions ...string) func(context.Context) error {
	version := ""
	if len(versions) > 0 {
		version = versions[0]
	}
	return func(ctx context.Context) error {
		switch kind {
		case "care":
			switch operation {
			case "correct":
				return f.care.svc.(carerequests.Corrections).Correct(ctx, id, false, version, "Corrected", f.care.staffAccount)
			case "mark done":
				return f.care.svc.MarkDone(ctx, id, version, "Done", f.care.staffAccount)
			case "conflict":
				return f.care.svc.(compose.ParentRequestConflicts).LockConflictRequest(ctx, id)
			default:
				_, err := f.care.svc.Decide(ctx, carerequests.DecideInput{RequestID: id, ExpectedVersion: version, Approve: operation == "approve", Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
				return err
			}
		case "excused":
			switch operation {
			case "correct":
				return f.excused.Correct(ctx, id, false, version, "Corrected", f.care.staffAccount)
			case "mark done":
				return f.excused.MarkDone(ctx, id, version, "Done", f.care.staffAccount)
			case "conflict":
				return f.excused.LockConflictRequest(ctx, id)
			default:
				_, err := f.excused.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: id, ExpectedVersion: version, Approve: operation == "approve", Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
				return err
			}
		default:
			switch operation {
			case "correct":
				return f.master.Correct(ctx, id, false, version, "Corrected", f.care.staffAccount)
			case "conflict":
				return f.master.LockConflictRequest(ctx, id)
			default:
				_, err := f.master.Decide(ctx, masterdatarequests.DecideInput{RequestID: id, ExpectedVersion: version, Approve: operation == "approve", Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
				return err
			}
		}
	}
}
