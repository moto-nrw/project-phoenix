package presence_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	active "github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/application/presence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// lockFault makes the n-th call fail with a real lock_not_available (55P03):
// it asks NOWAIT for a session row another transaction holds, so the
// statement aborts the caller's transaction state like any database error.
type lockFault struct {
	tx      bun.Tx
	blocker int64
	at      int
	calls   int
	hit     int64
}

func (f *lockFault) fail(ctx context.Context, staffID int64) error {
	call := f.calls
	f.calls++
	if call != f.at {
		return nil
	}
	f.hit = staffID
	var id int64
	err := f.tx.NewRaw("SELECT id FROM active.groups WHERE id = ? FOR UPDATE NOWAIT", f.blocker).Scan(ctx, &id)
	if err == nil {
		return fmt.Errorf("lock fault: row %d was not held", f.blocker)
	}
	return err
}

type faultingSupervisors struct {
	ports.GroupSupervisorRepository
	fault *lockFault
}

func (r faultingSupervisors) Create(ctx context.Context, row *ports.GroupSupervisor) error {
	if err := r.fault.fail(ctx, row.StaffID); err != nil {
		return err
	}
	return r.GroupSupervisorRepository.Create(ctx, row)
}

type faultingWorkSessions struct {
	fault *lockFault
	staff []int64
}

func (w *faultingWorkSessions) EnsureCheckedIn(ctx context.Context, staffID int64, _ studentpresence.StampSource) (bool, error) {
	w.staff = append(w.staff, staffID)
	if w.fault == nil {
		return true, nil
	}
	return true, w.fault.fail(ctx, staffID)
}

// TestSessionSupervisorAssignmentRecoversEachSupervisorIndependently pins the
// best-effort supervisor assignment of a session start (#5873) against a
// real one-time lock_not_available at every insert and every NFC check-in:
//  1. a failed insert loses exactly that supervisor; every other one is
//     stored exactly once;
//  2. a failed NFC check-in loses no supervision;
//  3. the call never fails, the transaction stays usable and commits, and
//     every supervisor gets its NFC check-in, also the one whose insert failed;
//  4. every assignment and every NFC check-in has a savepoint of its own,
//     and each supervisor's insert is attempted once.
func TestSessionSupervisorAssignmentRecoversEachSupervisorIndependently(t *testing.T) {
	t.Parallel()
	testpkg.SetupTestDB(t)
	type faultPoint struct {
		where string
		at    int
	}
	points := []faultPoint{{where: "none", at: -1}}
	for at := range 3 {
		points = append(points, faultPoint{where: "insert", at: at}, faultPoint{where: "nfc", at: at})
	}
	for _, point := range points {
		t.Run(fmt.Sprintf("%s_%d", point.where, point.at), func(t *testing.T) {
			testpkg.OwnTenant(t)
			db := testpkg.SetupIsolatedTestDB(t)
			ctx := testpkg.Ctx(t)
			module := testSchoolPresence(t, db)
			group := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			blocker := testpkg.CreateTestActiveGroupForTenant(t, db, testpkg.Tenant(t))
			staff := make([]int64, 0, 3)
			for i := range 3 {
				staff = append(staff, testpkg.CreateTestStaff(t, db, "Recovery", fmt.Sprintf("Supervisor%d", i)).ID)
			}
			holder := holdStayRow(t, db, blocker.ID)
			defer holder.commit(t)

			records := owners.ComposePresenceSessionRecords(presenceCompose.SessionRecordDependencies{DB: db})
			counter := testpkg.CaptureQueries(t, db)
			var fault *lockFault
			require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
				repo := sessionSupervisors(records)
				workSessions := &faultingWorkSessions{}
				if point.where != "none" {
					fault = &lockFault{tx: tx, blocker: blocker.ID, at: point.at}
				}
				switch point.where {
				case "insert":
					repo = faultingSupervisors{GroupSupervisorRepository: repo, fault: fault}
				case "nfc":
					workSessions.fault = fault
				}
				svc := active.NewService(active.ServiceDependencies{SupervisorRepo: repo, WorkSessionService: workSessions})
				counter.Reset()
				active.AssignSessionSupervisorsForTest(svc, txCtx, group.ID, append([]int64{staff[2]}, staff...), time.Now())
				counter.Stop()
				assert.ElementsMatch(t, staff, workSessions.staff, "invariant 3: one NFC check-in per supervisor")
				var one int
				require.NoError(t, tx.NewRaw("SELECT 1").Scan(txCtx, &one), "invariant 3: the transaction stays usable")
				return nil
			}), "invariant 3: the assignment never fails the transaction")

			want := staff
			if point.where == "insert" {
				require.NotZero(t, fault.hit)
				want = make([]int64, 0, len(staff))
				for _, id := range staff {
					if id != fault.hit {
						want = append(want, id)
					}
				}
			}
			rows, err := module.QueryGroupSupervisions(ctx, studentpresence.GroupSupervisionFilter{GroupIDs: []int64{group.ID}})
			require.NoError(t, err)
			stored := make([]int64, 0, len(rows))
			for _, row := range rows {
				stored = append(stored, row.StaffID)
			}
			assert.ElementsMatch(t, want, stored, "invariants 1 and 2: exactly the supervisors whose insert succeeded, once each")

			savepoints := counter.Matching(func(sql string) bool {
				return strings.HasPrefix(strings.TrimSpace(sql), "savepoint")
			})
			inserts := counter.Matching(func(sql string) bool {
				return strings.HasPrefix(strings.TrimSpace(sql), "insert into active.group_supervisors")
			})
			assert.Len(t, savepoints, 2*len(staff), "invariant 4: a savepoint per assignment and per NFC check-in")
			assert.Len(t, inserts, len(want), "invariant 4: each successful supervisor is inserted once")
		})
	}
}
