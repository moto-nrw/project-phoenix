package enrollmenthttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	transactionFailures "github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// HTTP status of the enrollment error mappings for real tenant-runtime
// failures (#6316, PR #6380 tables A–D): a savepoint control the database
// failed while the request was alive answers 500 ahead of every business,
// constraint and transient rule (E19); a cancelled or expired request
// context keeps its answer; an attached ROLLBACK failure decides nothing.
// Retry counts and persistence of the same rows are measured in
// modules/transactionruntime/compose and the decision command's tests.

type statusRig struct {
	probe  *testpkg.FaultProbe
	ctx    context.Context
	tenant tenant.TenantID
}

func newStatusRig(t *testing.T) *statusRig {
	t.Helper()
	probe := testpkg.NewFaultProbe(t)
	id, err := tenant.NewTenantID(testpkg.Tenant(t))
	require.NoError(t, err)
	return &statusRig{probe: probe, ctx: tenant.WithUnitOfWork(context.Background(), testpkg.TenantRuntime(t, probe.Pool)), tenant: id}
}

type txStep func(ctx context.Context, tx bun.Tx) error

// unitError runs one tenant unit as the routes do (no runtime replay): it
// writes a probe row, then runs script, and returns the unit's error.
func (r *statusRig) unitError(t *testing.T, ctx context.Context, script txStep) error {
	t.Helper()
	r.probe.Reset(t)
	err := tenant.WithinTenant(ctx, r.tenant, func(ctx context.Context) error {
		raw, ok := tenant.TransactionFromContext(ctx)
		if !ok {
			return errors.New("no transaction")
		}
		tx := raw.(bun.Tx)
		if err := r.probe.Write(ctx, tx, 1); err != nil {
			return err
		}
		if script == nil {
			return nil
		}
		return script(ctx, tx)
	})
	return err
}

func inSavepoint(step txStep) txStep {
	return func(ctx context.Context, tx bun.Tx) error {
		return tenant.WithSavepoint(ctx, func(ctx context.Context) error { return step(ctx, tx) })
	}
}

func raiseMain(kind string) txStep {
	return func(ctx context.Context, tx bun.Tx) error {
		if kind == "" {
			return errors.New("operation failed without SQLSTATE")
		}
		return testpkg.RaiseSQLState(ctx, tx, kind)
	}
}

func statusFailures() DatabaseFailures {
	return DatabaseFailures{
		Transient:         transactionFailures.IsTransientDatabaseError,
		Constraint:        transactionFailures.IsConstraintViolation,
		SavepointRollback: func(err error) bool { return errors.Is(err, tenant.ErrSavepointRollback) },
	}
}

// statusMappings answer err through each enrollment mapping that has a
// savepoint-control rule and return the HTTP status per mapping.
func statusMappings(t *testing.T, ctx context.Context, err error) map[string]int {
	t.Helper()
	rs := &Resource{Failures: statusFailures()}
	answer := func(write func(w http.ResponseWriter, r *http.Request)) int {
		w := httptest.NewRecorder()
		write(w, httptest.NewRequestWithContext(ctx, http.MethodPost, "/", nil))
		return w.Code
	}
	care := &mockCareOfferingService{deleteErr: err}
	careRS := &Resource{Failures: statusFailures(), CareOfferingService: care}
	router := chi.NewRouter()
	router.Use(render.SetContentType(render.ContentTypeJSON))
	router.Delete("/enrollment/care-offerings/{id}", careRS.deleteCareOffering)
	careWriter := httptest.NewRecorder()
	router.ServeHTTP(careWriter, httptest.NewRequestWithContext(ctx, http.MethodDelete, "/enrollment/care-offerings/7", nil))
	return map[string]int{
		"decide":  answer(func(w http.ResponseWriter, r *http.Request) { rs.renderDecideError(w, r, err) }),
		"restore": answer(func(w http.ResponseWriter, r *http.Request) { rs.renderRestoreError(w, r, err) }),
		"manual":  answer(func(w http.ResponseWriter, r *http.Request) { rs.mapManualEnrollmentError(w, r, err) }),
		"care":    careWriter.Code,
	}
}

func all(status int) map[string]int {
	return map[string]int{"decide": status, "restore": status, "manual": status, "care": status}
}

func TestSavepointControlAnswers500AheadOfOtherRules(t *testing.T) {
	t.Parallel()
	rig := newStatusRig(t)
	type cell struct {
		site testpkg.FaultSite
		kind string
		main string
		want map[string]int
	}
	var cells []cell
	// A1/A2: the savepoint control fails → 500 everywhere.
	for _, site := range []testpkg.FaultSite{testpkg.FaultSavepointRestore, testpkg.FaultSavepointRelease} {
		for _, main := range []string{"", "23503", "23514", "40P01", "40001"} {
			for _, kind := range []string{"", "23514", "23503", "57014", "40P01", "40001", "bad_conn"} {
				cells = append(cells, cell{site: site, kind: kind, main: main, want: all(http.StatusInternalServerError)})
			}
		}
	}
	// A3: the outer ROLLBACK fails → the operation error decides.
	for _, main := range []string{"", "23503", "23514", "40P01", "40001"} {
		for _, kind := range []string{"", "23514", "23503", "57014", "40P01", "40001", "bad_conn"} {
			want := all(http.StatusInternalServerError)
			if main == "23503" {
				want["care"] = http.StatusBadRequest // the offering is in use: a real foreign key
			}
			cells = append(cells, cell{site: testpkg.FaultRollback, kind: kind, main: main, want: want})
		}
	}
	for _, c := range cells {
		name := fmt.Sprintf("main_%s/%s_%s", nameOf(c.main), strings.ReplaceAll(string(c.site), " ", "_"), nameOf(c.kind))
		t.Run(name, func(t *testing.T) {
			rig.probe.Faults.Arm(c.site, testpkg.InjectedFailure(c.kind))
			ctx, cancel := context.WithTimeout(rig.ctx, 20*time.Second)
			defer cancel()
			script := raiseMain(c.main)
			if c.site != testpkg.FaultRollback {
				script = inSavepoint(script)
			}
			err := rig.unitError(t, ctx, script)
			require.Error(t, err)
			require.True(t, rig.probe.Faults.Fired(), "the cleanup fault fired")
			require.Zero(t, rig.probe.Rows(t), "a failed unit commits nothing")
			got := statusMappings(t, ctx, err)
			t.Logf("CELL %s %v", name, got)
			require.Equal(t, c.want, got, "error: %v", err)
		})
	}
}

func nameOf(kind string) string {
	if kind == "" {
		return "no_sqlstate"
	}
	return kind
}

// Tables B, C and D: connection loss, an ended request context, COMMIT and
// BEGIN failures, each through every mapping.
func TestSavepointControlStatusAtTheEdges(t *testing.T) {
	t.Parallel()
	rig := newStatusRig(t)
	terminate := func(ctx context.Context, tx bun.Tx) error { return rig.probe.TerminateBackend(ctx, tx) }
	unavailable := map[string]int{"decide": http.StatusServiceUnavailable, "restore": http.StatusServiceUnavailable, "manual": http.StatusServiceUnavailable, "care": http.StatusInternalServerError}
	cases := []struct {
		name   string
		site   testpkg.FaultSite
		kind   string
		script func(cancel context.CancelFunc) txStep
		ctx    func() (context.Context, context.CancelFunc)
		want   map[string]int
		target error
		rows   int
	}{
		{name: "B1_connection_lost_in_savepoint", script: func(context.CancelFunc) txStep { return inSavepoint(terminate) }, want: all(http.StatusInternalServerError), target: tenant.ErrSavepointRollback},
		{name: "B2b_backend_terminated_outside_savepoint", script: func(context.CancelFunc) txStep { return terminate }, want: all(http.StatusInternalServerError)},
		{name: "B3_request_cancelled_in_savepoint", script: func(cancel context.CancelFunc) txStep {
			return inSavepoint(func(ctx context.Context, tx bun.Tx) error {
				cancel()
				_, err := tx.ExecContext(ctx, `SELECT 1`)
				return err
			})
		}, want: all(499), target: context.Canceled},
		{name: "B4_request_deadline_in_savepoint", script: func(context.CancelFunc) txStep {
			return inSavepoint(func(ctx context.Context, tx bun.Tx) error {
				_, err := tx.ExecContext(ctx, `SELECT pg_sleep(5)`)
				return err
			})
		}, ctx: func() (context.Context, context.CancelFunc) {
			return context.WithTimeout(rig.ctx, 300*time.Millisecond)
		},
			want: all(http.StatusRequestTimeout)},
		{name: "C3_commit_applied_bad_conn", site: testpkg.FaultCommitApplied, kind: "bad_conn", want: unavailable, rows: 1},
		{name: "C5_request_cancelled_before_commit", script: func(cancel context.CancelFunc) txStep {
			return func(context.Context, bun.Tx) error { cancel(); return nil }
		}, want: all(499), target: context.Canceled},
		{name: "D1_begin_40P01", site: testpkg.FaultBegin, kind: "40P01", want: all(http.StatusInternalServerError)},
		{name: "D2_begin_bad_conn", site: testpkg.FaultBegin, kind: "bad_conn", want: unavailable},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rig.probe.Faults.Arm(c.site, testpkg.InjectedFailure(c.kind))
			ctx, cancel := context.WithTimeout(rig.ctx, 20*time.Second)
			if c.ctx != nil {
				ctx, cancel = c.ctx()
			}
			defer cancel()
			var script txStep
			if c.script != nil {
				script = c.script(cancel)
			}
			err := rig.unitError(t, ctx, script)
			require.Error(t, err)
			if c.site != testpkg.FaultNone {
				require.True(t, rig.probe.Faults.Fired())
			}
			if c.target != nil {
				require.ErrorIs(t, err, c.target)
			}
			require.Equal(t, c.rows, rig.probe.Rows(t), "committed rows")
			got := statusMappings(t, ctx, err)
			t.Logf("CELL %s %v err=%v", c.name, got, err)
			require.Equal(t, c.want, got, "error: %v", err)
		})
	}
}
