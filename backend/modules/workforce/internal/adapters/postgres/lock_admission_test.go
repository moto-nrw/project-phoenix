package postgres

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
	"github.com/moto-ogs/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/tenant"
	testpkg "github.com/moto-ogs/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Every Workforce lock statement is admitted through the unit of work's lock
// order before it can wait (#6702): while another transaction holds the
// conflicting lock and the path waits for it, the path's ledger already
// holds the lock in its class. A path that took the lock without
// tenant.OrderLock would wait outside the order, and a later gate, day or
// staff row of its transaction would not be refused.

// admissionPath is one lock statement: hold takes the conflicting lock in a
// separate transaction, run takes the path's lock in a ledger transaction.
type admissionPath struct {
	class tenant.LockClass
	key   string
	hold  func(context.Context, bun.Tx) error
	run   func(context.Context) error
}

type admittedPathTx struct {
	ctx context.Context
	pid int64
}

func requireAdmittedBeforeWait(t *testing.T, db *bun.DB, path admissionPath) {
	t.Helper()
	ctx := testpkg.Ctx(t)
	holder, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = holder.Rollback() }()
	var holderPID int64
	require.NoError(t, holder.NewRaw("SELECT pg_backend_pid()").Scan(ctx, &holderPID))
	require.NoError(t, path.hold(ctx, holder))

	started, done := make(chan admittedPathTx, 1), make(chan error, 1)
	go func() {
		done <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			var pid int64
			if err := tx.NewRaw("SELECT pg_backend_pid()").Scan(txCtx, &pid); err != nil {
				return err
			}
			started <- admittedPathTx{ctx: txCtx, pid: pid}
			return path.run(txCtx)
		})
	}()
	var waiting admittedPathTx
	select {
	case waiting = <-started:
	case err := <-done:
		require.FailNow(t, "the path failed before it started", "%v", err)
	}
	awaitWaitingFor(t, db, waiting.pid, holderPID, done)
	require.True(t, tenant.HoldsLock(waiting.ctx, path.class, path.key),
		"the path waits for %s %q without having it admitted in the lock order", path.class, path.key)
	require.NoError(t, holder.Rollback())
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		require.FailNow(t, "the path never finished after the holder released its lock")
	}
}

// requireAdmittedSkipLocked covers the SKIP LOCKED statements: they never
// wait, and the lock they took is in the ledger afterwards.
func requireAdmittedSkipLocked(t *testing.T, db *bun.DB, class tenant.LockClass, key string, run func(context.Context) error) {
	t.Helper()
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if err := run(txCtx); err != nil {
			return err
		}
		require.True(t, tenant.HoldsLock(txCtx, class, key), "%s %q is not in the ledger", class, key)
		return nil
	}))
}

func awaitWaitingFor(t *testing.T, db *bun.DB, pid, holder int64, done <-chan error) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		select {
		case err := <-done:
			require.FailNow(t, "the path finished instead of waiting for the holder", "%v", err)
		default:
		}
		var blocked bool
		require.NoError(t, db.NewRaw(`SELECT ?::int = ANY(pg_blocking_pids(?::int))`, holder, pid).Scan(context.Background(), &blocked))
		if blocked {
			return
		}
	}
	require.FailNow(t, "the path never waited for the holder")
}

func holdRow(table string, id int64) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SELECT id FROM "+table+" WHERE id = ? FOR UPDATE", id)
		return err
	}
}

func holdAdvisory(key string) func(context.Context, bun.Tx) error {
	return func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.ExecContext(ctx, "SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key)
		return err
	}
}

func TestWorkforceRowLocksAreAdmittedBeforeTheyWait(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	store := New(DatabaseRuntime(db))
	school := testpkg.Tenant(t)
	// A fixed future school day: the paths take their locks whatever the
	// wall clock says.
	today := calendar.NewDate(2099, 6, 1)
	from := calendar.NewDate(2026, 1, 1)
	ctx := testpkg.Ctx(t)

	t.Run("absence type", func(t *testing.T) {
		absenceType := testpkg.CreateTestStaffAbsenceType(t, db, "Admission")
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "active.staff_absence_types",
			hold: holdRow("active.staff_absence_types", absenceType.ID),
			run: func(ctx context.Context) error {
				_, _, _, err := store.FindStaffAbsenceType(ctx, absenceType.ID, true)
				return err
			}})
	})

	offboardingPaths := []struct {
		name, table string
		row         func(t *testing.T, staffID int64) int64
	}{
		{"offboarding absences", "active.staff_absences", func(t *testing.T, staffID int64) int64 {
			return testpkg.CreateTestStaffAbsenceToday(t, db, staffID, testpkg.CreateTestStaffAbsenceType(t, db, "Offboarding").ID).ID
		}},
		{"offboarding series", "schedule.staff_shift_series", func(t *testing.T, staffID int64) int64 {
			start := calendar.NewDate(2099, 9, 1)
			period := testpkg.CreateTestCalendarPeriod(t, db, "Admission", start, calendar.NewDate(2100, 6, 30))
			var series domain.StaffShiftSeries
			require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, school, func(ctx context.Context) error {
				var err error
				series, _, err = store.CreateStaffShiftSeries(ctx, domain.StaffShiftSeries{
					StaffID: staffID, Weekdays: []int{1}, StartTime: "09:00:00", EndTime: "12:00:00",
					CalendarPeriodID: period.ID, WeekPattern: workforce.WeekPatternEvery, ValidFrom: start.String(), CreatedBy: staffID,
				})
				return err
			}))
			return series.ID
		}},
		{"offboarding shifts", "schedule.staff_shifts", func(t *testing.T, staffID int64) int64 {
			return testpkg.CreateTestStaffShift(t, db, staffID, testpkg.CalendarDate(today.AddDays(7)), testpkg.StaffShiftOpts{}).ID
		}},
		{"offboarding substitutions", "education.group_substitution", func(t *testing.T, staffID int64) int64 {
			group := testpkg.CreateTestEducationGroup(t, db, "Admission")
			return testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staffID, today, today.AddDays(7)).ID
		}},
	}
	for _, path := range offboardingPaths {
		t.Run(path.name, func(t *testing.T) {
			staff := testpkg.CreateTestStaff(t, db, "Offboarding", "Admission")
			id := path.row(t, staff.ID)
			requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: path.table,
				hold: holdRow(path.table, id),
				run: func(ctx context.Context) error {
					_, _, err := store.PreviewStaffOffboarding(ctx, staff.ID, from.String())
					return err
				}})
		})
	}

	t.Run("payroll dispatch sequence", func(t *testing.T) {
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "advisory:payroll_dispatch",
			hold: holdAdvisory(fmt.Sprintf("payroll_dispatch:%d:%04d-%02d", school, 2099, 9)),
			run: func(ctx context.Context) error {
				_, _, err := store.NextPayrollDispatchSequence(ctx, 2099, 9)
				return err
			}})
	})

	t.Run("payroll download", func(t *testing.T) {
		id := insertOpenPayrollDownload(t, db, store, "fp:admission")
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "active.payroll_dispatches",
			hold: holdRow("active.payroll_dispatches", id),
			run: func(ctx context.Context) error {
				_, err := store.OpenPayrollDownload(ctx, "fp:admission", time.Now())
				return err
			}})
	})

	t.Run("payroll rules", func(t *testing.T) {
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockPayrollRules, key: tenant.LockIDKey(school),
			hold: holdAdvisory(fmt.Sprintf("payroll_rules:%d", school)),
			run:  store.LockPayrollRules})
	})

	t.Run("bank data rekey", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Bank", "Admission")
		var financial domain.StaffFinancialData
		require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, school, func(ctx context.Context) error {
			var err error
			financial, _, err = store.CreateStaffFinancialData(ctx, domain.StaffFinancialData{StaffID: staff.ID})
			return err
		}))
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "users.staff_financial_data",
			hold: holdRow("users.staff_financial_data", financial.ID),
			run: func(ctx context.Context) error {
				_, err := NewStaffBankRekeyStore().RewriteStaffBankData(ctx, school, financial.ID-1, 1,
					func(*domain.StaffFinancialData) (bool, error) { return false, nil })
				return err
			}})
	})

	t.Run("staff employment", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Employment", "Admission")
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "users.staff_employment_profiles",
			hold: func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(ctx, "SELECT membership_id FROM users.staff_employment_profiles WHERE membership_id = ? FOR UPDATE", staff.ID)
				return err
			},
			run: func(ctx context.Context) error {
				_, err := store.LockStaffEmployment(ctx, staff.ID)
				return err
			}})
	})

	t.Run("group substitution", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Substitution", "Admission")
		group := testpkg.CreateTestEducationGroup(t, db, "Substitution admission")
		substitution := testpkg.CreateTestGroupSubstitution(t, db, group.ID, nil, staff.ID, today, today.AddDays(1))
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "education.group_substitution",
			hold: holdRow("education.group_substitution", substitution.ID),
			run: func(ctx context.Context) error {
				_, _, _, err := store.FindGroupSubstitution(ctx, substitution.ID, true)
				return err
			}})
	})

	t.Run("group substitution table", func(t *testing.T) {
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "education.group_substitution",
			hold: func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(ctx, "LOCK TABLE education.group_substitution IN ROW EXCLUSIVE MODE")
				return err
			},
			run: func(ctx context.Context) error {
				_, err := store.LockGroupSubstitutions(ctx)
				return err
			}})
	})

	t.Run("work sessions", func(t *testing.T) {
		staff := testpkg.CreateTestStaff(t, db, "Session", "Admission")
		var session workforce.WorkSession
		require.NoError(t, testpkg.WithinTenantContext(t, ctx, db, school, func(ctx context.Context) error {
			var err error
			session, _, err = store.CreateWorkSession(ctx, workforce.WorkSession{
				StaffID: staff.ID, Date: today.String(), Status: workforce.WorkSessionStatusPresent,
				Source: workforce.WorkSessionSourceApp, CheckInTime: today.BerlinMidnight().Add(9 * time.Hour), CreatedBy: staff.ID,
			})
			return err
		}))
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "active.work_sessions",
			hold: holdRow("active.work_sessions", session.ID),
			run: func(ctx context.Context) error {
				_, _, _, err := store.LockOpenWorkSession(ctx, session.ID)
				return err
			}})
		requireAdmittedBeforeWait(t, db, admissionPath{class: tenant.LockLeaf, key: "active.work_sessions",
			hold: holdRow("active.work_sessions", session.ID),
			run: func(ctx context.Context) error {
				_, _, _, err := store.OpenWorkSessionOn(ctx, staff.ID, today.String(), true)
				return err
			}})
	})

	lock := func(ctx context.Context, key string) error { return AcquireXactLock(ctx, key, store.AcquireXactLock) }
	for _, gate := range []struct {
		name  string
		class tenant.LockClass
		key   func(staffID int64) string
		lock  func(context.Context, int64) error
	}{
		{"staff-balance", tenant.LockStaffWorkTime, func(id int64) string { return tenant.StaffWorkTimeKey(id, false) }, func(ctx context.Context, id int64) error { return LockStaffBalanceGate(ctx, id, lock) }},
		{"staff-absence", tenant.LockStaffWorkTime, func(id int64) string { return tenant.StaffWorkTimeKey(id, true) }, func(ctx context.Context, id int64) error { return LockStaffAbsenceGate(ctx, id, lock) }},
		{"staff-qualifications", tenant.LockLeaf, func(int64) string { return "advisory:staff-qualifications" }, func(ctx context.Context, id int64) error { return LockStaffQualificationsGate(ctx, id, lock) }},
	} {
		t.Run(gate.name+" gate", func(t *testing.T) {
			staff := testpkg.CreateTestStaff(t, db, "Gate", "Admission")
			requireAdmittedBeforeWait(t, db, admissionPath{class: gate.class, key: gate.key(staff.ID),
				hold: holdAdvisory(fmt.Sprintf("%s:%d:%d", gate.name, school, staff.ID)),
				run:  func(ctx context.Context) error { return gate.lock(ctx, staff.ID) }})
		})
	}

	t.Run("skip locked claims", func(t *testing.T) {
		requireAdmittedSkipLocked(t, db, tenant.LockLeaf, "users.staff_offboarding_cleanup", func(ctx context.Context) error {
			_, err := store.ClaimDocumentCleanup(ctx, 1, time.Now())
			return err
		})
		requireAdmittedSkipLocked(t, db, tenant.LockLeaf, "active.payroll_dispatches", func(ctx context.Context) error {
			_, err := store.ClaimPayrollDispatches(ctx, time.Now(), time.Minute, 1)
			return err
		})
		requireAdmittedSkipLocked(t, db, tenant.LockLeaf, "users.staff_document_file_cleanup", func(ctx context.Context) error {
			_, _, err := store.ListQueuedStaffDocumentFileCleanups(ctx, 0, time.Now())
			return err
		})
	})
}

// insertOpenPayrollDownload stores a sent dispatch whose e-mail link is still
// open under the token fingerprint.
func insertOpenPayrollDownload(t *testing.T, db *bun.DB, store *Store, tokenHash string) int64 {
	t.Helper()
	actor := testpkg.CreateTestAccount(t, db, "payroll-admission")
	finished := time.Now()
	var id int64
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, testpkg.Tenant(t), func(ctx context.Context, tx bun.Tx) error {
		var err error
		id, err = store.InsertPayrollDispatch(ctx, domain.PayrollDispatch{Year: 2099, Month: 10,
			Channel: workforce.PayrollChannelEmail, Format: "xlsx", Recipient: "lohn@example.org", ActorAccountID: actor.ID, ActorName: "Admission",
			State: workforce.PayrollDispatchSent, NextAttemptAt: time.Now(), FinishedAt: &finished})
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE active.payroll_dispatches SET download_token_hash = ?, file_data = ?,
			filename = 'admission.xlsx', content_type = 'application/octet-stream', byte_size = 1,
			download_expires_at = now() + interval '1 hour' WHERE id = ?`, tokenHash, []byte("x"), id)
		return err
	}))
	return id
}
