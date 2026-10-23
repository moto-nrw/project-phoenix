package contracttest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan/excusedrequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/masterdatarequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/parentrequests"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

const requestLockToday calendar.Date = "2026-08-24"

// Each case uses the real decision and care-end commands on two connections.
// PostgreSQL must report the decision waiting on care end before confirmation
// continues. Request-before-child closes a real 40P01 cycle at that point.
func TestRequestDecisionsAndCareExitDoNotDeadlock(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, kind := range []string{"care", "excused", "master data"} {
		for _, approve := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/approve=%t", kind, approve), func(t *testing.T) {
				f := newRequestLockFixture(t, db)
				write := f.decision(t, kind, approve)
				database := testpkg.NewDatabase(t, db)
				recurrence := owners.MustNewTimetableRecurrenceLock(database)
				membership := owners.MustNewSchoolMembership(database)
				people := owners.MustNewPeopleDirectory(database)
				repos, err := api.NewCareLifecycleTestRepositories(database, nil)
				require.NoError(t, err)
				lifecycle, err := repos.NewCareLifecycle(api.CareLifecycleTestConfig{
					Audit:                 api.NewPeopleDirectorySuiteStudentAudit(testpkg.RequestAuditActor, api.NewStudentAudit(database)),
					LockCareBookingWrites: recurrence.LockRecurrenceWrites,
					BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil },
					Today:                 func() calendar.Date { return requestLockToday },
				})
				require.NoError(t, err)
				ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
				defer cancel()
				input := careplan.CareExitInput{StudentIDs: []int64{f.care.chain.StudentID}, LastCareDay: requestLockToday.AddDays(10), Reason: careplan.CareExitReasonMovedAway}
				preview, err := testpkg.TenantQuery1(t, ctx, db, lifecycle.Preview, input)
				require.NoError(t, err)
				require.False(t, preview.Blocked)
				done := make(chan error, 1)
				careErr := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(careCtx context.Context, tx bun.Tx) error {
					if err := membership.LockStudentClassWrites(careCtx, false); err != nil {
						return err
					}
					if err := recurrence.LockRecurrenceWrites(careCtx); err != nil {
						return err
					}
					if err := people.LockStudent(careCtx, f.care.chain.StudentID); err != nil {
						return err
					}
					var pid int
					require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(careCtx, &pid))
					go func() {
						done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return write(writeCtx) })
					}()
					waitForRequestWriter(t, careCtx, tx, pid)
					_, err := lifecycle.Confirm(careCtx, preview.Token, input, f.care.staffAccount)
					return err
				})
				writeErr := awaitRequestWriter(t, ctx, done)
				require.NoError(t, careErr, "care end must not deadlock")
				require.NoError(t, writeErr, "decision must finish after scheduled care end")
			})
		}
	}
}

type requestLockFixture struct {
	care       *careFixture
	excused    *compose.ExcusedAbsenceRequests
	master     *compose.MasterDataDecisions
	careRow    carerequests.Request
	excusedRow careplan.ExcusedAbsenceRequest
	masterRow  careplan.StudentDataChangeRequest
}

func newRequestLockFixture(t *testing.T, db *bun.DB) requestLockFixture {
	t.Helper()
	f := newCareFixture(t, owners.WithCareRequestToday(func() calendar.Date { return requestLockToday }))
	ctx := f.staffCtx(f.staffAccount)
	owner := f.reposCare.CarePlan()
	excused, err := api.NewTestExcusedAbsenceRequests(api.ExcusedRequestTestOptions{CarePlan: owner, Today: func() calendar.Date { return requestLockToday }},
		api.ExcusedRequestTestPeople{Students: f.reposPeople.Student, Persons: f.reposPeople.Person}, nil, nil)
	require.NoError(t, err)
	master, err := api.NewTestMasterDataDecisions(api.MasterDataDecisionTestOptions{CarePlan: owner, Today: func() calendar.Date { return requestLockToday }},
		api.MasterDataDecisionTestPeople{People: f.persons, Students: f.reposPeople.Student, Persons: f.reposPeople.Person}, nil, nil)
	require.NoError(t, err)
	var result requestLockFixture
	require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		result.careRow, err = owner.CreateCareScheduleRequest(txCtx, carerequests.Request{StudentID: f.chain.StudentID, SubmittedBy: f.chain.AccountID, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"14:00"}]}`)})
		if err != nil {
			return err
		}
		result.excusedRow, err = owner.CreateExcusedAbsenceRequest(txCtx, careplan.ExcusedAbsenceRequest{StudentID: f.chain.StudentID, SubmittedBy: f.chain.AccountID, Dates: []excusedrequests.Date{excusedrequests.Date(requestLockToday.AddDays(1))}, AbsenceStatus: "excused", Status: "pending"})
		if err != nil {
			return err
		}
		result.masterRow, err = owner.CreateStudentDataRequest(txCtx, careplan.StudentDataChangeRequest{StudentID: f.chain.StudentID, SubmittedBy: f.chain.AccountID, Target: "person", FieldKey: "first_name", OldValue: json.RawMessage(`"Felix"`), NewValue: json.RawMessage(`"Max"`), Status: "pending"})
		return err
	}))
	result.care, result.excused, result.master = f, excused, master
	return result
}

func (f requestLockFixture) decision(t *testing.T, kind string, approve bool) func(context.Context) error {
	t.Helper()
	return func(ctx context.Context) error {
		switch kind {
		case "care":
			_, err := f.care.svc.Decide(ctx, carerequests.DecideInput{RequestID: f.careRow.ID, Approve: approve, Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
			return err
		case "excused":
			_, err := f.excused.Decide(ctx, careplan.ExcusedRequestDecideInput{RequestID: f.excusedRow.ID, Approve: approve, Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
			return err
		default:
			_, err := f.master.Decide(ctx, masterdatarequests.DecideInput{RequestID: f.masterRow.ID, Approve: approve, Reason: "Reviewed", ReviewedBy: f.care.staffAccount})
			return err
		}
	}
}

func waitForRequestWriter(t *testing.T, ctx context.Context, tx bun.Tx, pid int) {
	t.Helper()
	require.Eventually(t, func() bool {
		var waiting bool
		err := tx.NewRaw("SELECT EXISTS (SELECT 1 FROM pg_locks l WHERE NOT l.granted AND ? = ANY(pg_blocking_pids(l.pid)))", pid).Scan(ctx, &waiting)
		return err == nil && waiting
	}, 5*time.Second, 10*time.Millisecond, "request writer must be blocked by the first connection")
}

func awaitRequestWriter(t *testing.T, ctx context.Context, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		t.Fatal("request writer did not finish")
		return ctx.Err()
	}
}

func TestRequestDecisionRechecksStateAfterWaitingForChild(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, kind := range []string{"care", "excused", "master"} {
		for _, change := range []string{"version", "status"} {
			t.Run(kind+"/"+change, func(t *testing.T) {
				f := newRequestLockFixture(t, db)
				ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
				defer cancel()
				people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
				version := careplan.ParentRequestVersion(f.careRow.UpdatedAt)
				if kind == "excused" {
					version = careplan.ParentRequestVersion(f.excusedRow.UpdatedAt)
				}
				if kind == "master" {
					version = careplan.ParentRequestVersion(f.masterRow.UpdatedAt)
				}
				done := make(chan error, 1)
				err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(editCtx context.Context, tx bun.Tx) error {
					if err := people.LockStudent(editCtx, f.care.chain.StudentID); err != nil {
						return err
					}
					var pid int
					require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(editCtx, &pid))
					go func() {
						done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error {
							return f.call(kind, "reject", f.id(kind), version)(writeCtx)
						})
					}()
					waitForRequestWriter(t, editCtx, tx, pid)
					query := tx.NewUpdate().TableExpr(f.table(kind)).Where("id = ?", f.id(kind))
					if change == "version" {
						query = query.Set("updated_at = updated_at + interval '1 second'")
					} else {
						query = query.Set("status = 'rejected'")
					}
					_, err := query.Exec(editCtx)
					return err
				})
				require.NoError(t, err)
				want := parentrequests.ErrStale
				if kind == "excused" {
					want = careplan.ErrParentRequestStale
				}
				if change == "status" {
					switch kind {
					case "care":
						want = careplan.ErrCareScheduleRequestNotPending
					case "excused":
						want = careplan.ErrExcusedRequestNotPending
					default:
						want = masterdatarequests.ErrReviewNotPending
					}
				}
				require.ErrorIs(t, awaitRequestWriter(t, ctx, done), want)
			})
		}
	}
}

// Kind/id order deliberately starts with the higher child's excused request.
// The whole sorted child frontier must precede ALL requests, not just the
// child belonging to each request when that request happens to be visited.
func TestBulkRequestLocksAllChildrenBeforeRequests(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	low, high := newRequestLockFixture(t, db), newRequestLockFixture(t, db)
	require.Less(t, low.care.chain.StudentID, high.care.chain.StudentID)
	ctx, cancel := context.WithTimeout(low.care.staffCtx(low.care.staffAccount), 20*time.Second)
	defer cancel()
	coordinator, err := compose.NewParentRequestCoordinator(compose.ParentRequestCoordinatorDependencies{
		Rights: func(context.Context) compose.ParentRequestRights {
			return compose.ParentRequestRights{WriteQueues: true, Absences: true}
		},
		MasterData: low.master, Excused: low.excused, Care: low.care.svc.(compose.ParentRequestConflicts), Offering: unusedConflictPort{},
	})
	require.NoError(t, err)
	done := make(chan error, 1)
	people := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db))
	err = testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(lockCtx context.Context, tx bun.Tx) error {
		if err := people.LockStudent(lockCtx, low.care.chain.StudentID); err != nil {
			return err
		}
		var pid int
		require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(lockCtx, &pid))
		go func() {
			done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error {
				return coordinator.BulkApprove(writeCtx, parentrequests.BulkApproveInput{Requests: []parentrequests.Ref{
					{Kind: parentrequests.KindMasterData, ID: low.masterRow.ID, ExpectedVersion: careplan.ParentRequestVersion(low.masterRow.UpdatedAt)},
					{Kind: parentrequests.KindExcused, ID: high.excusedRow.ID, ExpectedVersion: careplan.ParentRequestVersion(high.excusedRow.UpdatedAt)},
				}, Reason: "Reviewed", ReviewerID: low.care.staffAccount})
			})
		}()
		waitForRequestWriter(t, lockCtx, tx, pid)
		// Use the owner's NOWAIT command, not a second hand-written child lock.
		_, err := people.FindStudentRecordForMutationNoWait(lockCtx, high.care.chain.StudentID)
		return err
	})
	writeErr := awaitRequestWriter(t, ctx, done)
	require.NoError(t, err, "the higher child must stay free until the lower child is locked")
	require.NoError(t, writeErr)
}

// The suffix contenders cover the other public writers, including the
// coordinator's pre-locks. A decided correction is not directly locked by
// care end; NOWAIT proves it also leaves the request free while waiting on
// the child, preventing a future request/child inversion.
func TestRequestWritersTakeChildBeforeRequest(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	testpkg.SetupTestDB(t)
	for _, operation := range []string{"care mark done", "care correction", "care conflict", "excused mark done", "excused correction", "excused conflict", "excused edit", "master correction", "master conflict", "bulk", "resolve care", "resolve excused", "resolve master"} {
		t.Run(operation, func(t *testing.T) {
			f := newRequestLockFixture(t, db)
			ctx, cancel := context.WithTimeout(f.care.staffCtx(f.care.staffAccount), 20*time.Second)
			defer cancel()
			write := f.writer(t, ctx, operation)
			done := make(chan error, 1)
			err := testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(lockCtx context.Context, tx bun.Tx) error {
				if err := owners.MustNewPeopleDirectory(testpkg.NewDatabase(t, db)).LockStudent(lockCtx, f.care.chain.StudentID); err != nil {
					return err
				}
				var pid int
				require.NoError(t, tx.NewRaw("SELECT pg_backend_pid()").Scan(lockCtx, &pid))
				go func() {
					done <- testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(writeCtx context.Context, _ bun.Tx) error { return write(writeCtx) })
				}()
				waitForRequestWriter(t, lockCtx, tx, pid)
				for _, row := range []struct {
					table string
					id    int64
				}{
					{"schedule.care_schedule_change_requests", f.careRow.ID}, {"active.excused_absence_requests", f.excusedRow.ID}, {"users.student_data_change_requests", f.masterRow.ID},
				} {
					var id int64
					if err := tx.NewSelect().TableExpr(row.table).Column("id").Where("id = ?", row.id).For("UPDATE NOWAIT").Scan(lockCtx, &id); err != nil {
						return err
					}
				}
				return nil
			})
			writeErr := awaitRequestWriter(t, ctx, done)
			require.NoError(t, err, "a child-waiting writer must not hold any request")
			require.NoError(t, writeErr)
		})
	}
}

func (f requestLockFixture) writer(t *testing.T, ctx context.Context, operation string) func(context.Context) error {
	t.Helper()
	owner := f.care.reposCare.CarePlan()
	mutate := func(table, set string) {
		_, err := f.care.db.NewUpdate().TableExpr(table).Set(set).Where("student_id = ?", f.care.chain.StudentID).Exec(ctx)
		require.NoError(t, err)
	}
	switch operation {
	case "care mark done":
		mutate("schedule.care_schedule_change_requests", `request_kind = 'pickup_change', payload = '{"date":"2026-08-20","pickup_time":"14:00"}'`)
		return func(ctx context.Context) error {
			return f.care.svc.MarkDone(ctx, f.careRow.ID, "", "Done", f.care.staffAccount)
		}
	case "care correction":
		mutate("schedule.care_schedule_change_requests", `request_kind = 'pickup_change', status = 'rejected', payload = '{"date":"2026-08-25","pickup_time":"14:00"}'`)
		return func(ctx context.Context) error {
			return f.care.svc.(carerequests.Corrections).Correct(ctx, f.careRow.ID, false, "", "Corrected", f.care.staffAccount)
		}
	case "care conflict":
		return func(ctx context.Context) error {
			return f.care.svc.(compose.ParentRequestConflicts).LockConflictRequest(ctx, f.careRow.ID)
		}
	case "excused mark done":
		mutate("active.excused_absence_requests", `dates = '["2026-08-20"]'::jsonb`)
		return func(ctx context.Context) error {
			return f.excused.MarkDone(ctx, f.excusedRow.ID, "", "Done", f.care.staffAccount)
		}
	case "excused correction":
		mutate("active.excused_absence_requests", `status = 'rejected'`)
		return func(ctx context.Context) error {
			return f.excused.Correct(ctx, f.excusedRow.ID, false, "", "Corrected", f.care.staffAccount)
		}
	case "excused conflict":
		return func(ctx context.Context) error { return f.excused.LockConflictRequest(ctx, f.excusedRow.ID) }
	case "excused edit":
		return func(ctx context.Context) error {
			_, err := f.excused.EditRequest(ctx, careplan.ExcusedRequestEditInput{RequestID: f.excusedRow.ID, StudentID: f.care.chain.StudentID, GuardianAccountID: f.care.chain.AccountID, Dates: []excusedrequests.Date{"2026-08-26"}, Note: "Edited"})
			return err
		}
	case "master correction":
		mutate("users.student_data_change_requests", `status = 'rejected'`)
		return func(ctx context.Context) error {
			return f.master.Correct(ctx, f.masterRow.ID, false, "", "Corrected", f.care.staffAccount)
		}
	case "master conflict":
		return func(ctx context.Context) error { return f.master.LockConflictRequest(ctx, f.masterRow.ID) }
	}
	coordinator, err := compose.NewParentRequestCoordinator(compose.ParentRequestCoordinatorDependencies{
		Rights: func(context.Context) compose.ParentRequestRights {
			return compose.ParentRequestRights{WriteQueues: true, Absences: true}
		},
		MasterData: f.master, Excused: f.excused, Care: f.care.svc.(compose.ParentRequestConflicts), Offering: unusedConflictPort{},
	})
	require.NoError(t, err)
	if operation == "bulk" {
		return func(ctx context.Context) error {
			return coordinator.BulkApprove(ctx, parentrequests.BulkApproveInput{Requests: []parentrequests.Ref{
				{Kind: parentrequests.KindMasterData, ID: f.masterRow.ID, ExpectedVersion: careplan.ParentRequestVersion(f.masterRow.UpdatedAt)},
				{Kind: parentrequests.KindExcused, ID: f.excusedRow.ID, ExpectedVersion: careplan.ParentRequestVersion(f.excusedRow.UpdatedAt)},
			}, Reason: "Reviewed", ReviewerID: f.care.staffAccount})
		}
	}
	var input parentrequests.ResolveConflictInput
	input.Reason, input.ReviewerID, input.None = "Resolved", f.care.staffAccount, true
	require.NoError(t, testpkg.WithTenantTx(t, ctx, f.care.db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		switch operation {
		case "resolve care":
			row := f.careRow
			row.ID = 0
			row.RequestKind = "pickup_change"
			row.Payload = json.RawMessage(`{"date":"2026-08-25","pickup_time":"14:00"}`)
			other, err := owner.CreateCareScheduleRequest(txCtx, row)
			if err != nil {
				return err
			}
			input.Kind, input.ConflictKey = parentrequests.KindCareSchedule, "care:pickup:1"
			input.RequestIDs, input.ExpectedVersions = []int64{other.ID, f.careRow.ID}, []string{careplan.ParentRequestVersion(other.UpdatedAt), careplan.ParentRequestVersion(f.careRow.UpdatedAt)}
		case "resolve excused":
			row := f.excusedRow
			row.ID = 0
			other, err := owner.CreateExcusedAbsenceRequest(txCtx, row)
			if err != nil {
				return err
			}
			input.Kind, input.ConflictKey = parentrequests.KindExcused, "absence:2026-08-25"
			input.RequestIDs, input.ExpectedVersions = []int64{other.ID, f.excusedRow.ID}, []string{careplan.ParentRequestVersion(other.UpdatedAt), careplan.ParentRequestVersion(f.excusedRow.UpdatedAt)}
		case "resolve master":
			row := f.masterRow
			row.ID = 0
			// Distinct fields exercise the shared same-child locking protocol;
			// the database prevents two pending master-data rows for one field.
			row.SubmittedBy = f.care.staffAccount
			row.FieldKey, row.OldValue, row.NewValue = "last_name", json.RawMessage(`"Schneider"`), json.RawMessage(`"Becker"`)
			other, err := owner.CreateStudentDataRequest(txCtx, row)
			if err != nil {
				return err
			}
			input.Kind, input.ConflictKey = parentrequests.KindMasterData, "master_data:person:first_name"
			input.RequestIDs, input.ExpectedVersions = []int64{other.ID, f.masterRow.ID}, []string{careplan.ParentRequestVersion(other.UpdatedAt), careplan.ParentRequestVersion(f.masterRow.UpdatedAt)}
		}
		return nil
	}))
	return func(ctx context.Context) error { return coordinator.ResolveConflict(ctx, input) }
}
