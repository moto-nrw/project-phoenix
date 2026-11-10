package integration

import (
	"context"
	"testing"
	"time"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/internal/adapters/postgres"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestHolidayCriterionScopeWaitsForTheFirstOfficeAnswer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	w := newPlacesWeek(t, db, "Träger Kriterien-Sperre")
	var criterion int64
	require.NoError(t, db.NewRaw(`INSERT INTO enrollment.holiday_allocation_criteria (organization_id, name, question, scope, position)
  VALUES (?, 'Berufstätig', 'Sind Sie berufstätig?', 'family', 1) RETURNING id`, w.organization).Scan(t.Context(), &criterion))
	runtimeCtx, cancel := context.WithTimeout(testpkg.ContextWithTenantRuntime(t.Context(), testpkg.TenantRuntime(t, db)), 10*time.Second)
	defer cancel()
	store := postgres.NewHolidayRegistrationStore()
	done := make(chan error, 1)
	pidReady := make(chan int, 1)
	var finished bool
	var saveErr error
	require.NoError(t, testpkg.WithinTraeger(runtimeCtx, w.organization, func(ctx context.Context) error {
		yes := true
		found, err := store.CheckHolidayCriterion(ctx, w.organization, w.period, w.waiting, criterion, 0, &yes, 0)
		require.NoError(t, err)
		require.True(t, found)
		go func() {
			done <- testpkg.WithinTraeger(runtimeCtx, w.organization, func(ctx context.Context) error {
				tx, err := postgres.NewDatabase()(ctx)
				if err != nil {
					return err
				}
				var pid int
				if err := tx.NewRaw(`SELECT pg_backend_pid()`).Scan(ctx, &pid); err != nil {
					return err
				}
				pidReady <- pid
				return store.WriteHolidayCriteria(ctx, w.organization, []enrollment.HolidayCriterionInput{
					{ID: criterion, Name: "Berufstätig", Question: "Sind Sie berufstätig?", Scope: enrollment.HolidayCriterionChild, Active: true},
				}, 0)
			})
		}()
		var pid int
		select {
		case pid = <-pidReady:
		case <-ctx.Done():
			return ctx.Err()
		}
		assert.Eventually(t, func() bool {
			select {
			case saveErr = <-done:
				finished = true
				return true
			default:
			}
			var blocked bool
			err := db.NewRaw(`SELECT cardinality(pg_blocking_pids(?)) > 0`, pid).Scan(ctx, &blocked)
			return err == nil && blocked
		}, 5*time.Second, 10*time.Millisecond)
		assert.False(t, finished, "the catalog save must wait for the uncommitted first answer")
		return nil
	}))
	if !finished {
		select {
		case saveErr = <-done:
		case <-runtimeCtx.Done():
			t.Fatal(runtimeCtx.Err())
		}
	}
	var fields validation.Errors
	require.ErrorAs(t, saveErr, &fields)
	assert.Contains(t, fields, "criteria.0.scope")
}
