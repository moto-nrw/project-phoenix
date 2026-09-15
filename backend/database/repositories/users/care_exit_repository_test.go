package users_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/gofrs/uuid"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The archive column "Beendet am" is a calendar day derived from a TIMESTAMPTZ.
// Casting that instant straight to DATE resolves it in the database session's
// timezone, so an exit recorded at 00:30 Berlin would be filed under the
// previous day whenever the session is not Europe/Berlin — which is exactly
// what the server's own connections are. The date has to be the Berlin one the
// person saw when they ended the care (#2487).
func TestCareExitRepository_RecordedAtUsesTheBerlinDay(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareExit
	ctx := testpkg.Ctx(t)

	class := fmt.Sprintf("ce-%s", uuid.Must(uuid.NewV4()).String()[:8])
	student := testpkg.CreateTestStudent(t, db, "CareExitRecorded", "Kid", class)

	yesterday := timezone.TodayDate().AddDays(-1)
	setLifecycle(t, db, student.ID, users.StudentStatusActive, nil, &yesterday)

	require.NoError(t, repo.Upsert(ctx, &users.CareExit{
		StudentID: student.ID,
		Reason:    users.CareExitReasonMovedAway,
	}))

	// 23:30 UTC is 00:30 of the FOLLOWING day in Berlin (CET, winter). The two
	// readings differ by a day, so the assertion below can only hold when the
	// query converts before it truncates.
	const recordedInstant = "2026-01-15 23:30:00+00"
	const wantBerlinDay = "2026-01-16"
	_, err := db.NewUpdate().
		TableExpr("users.student_care_exits").
		Set("created_at = ?::timestamptz", recordedInstant).
		Where("student_id = ?", student.ID).
		Exec(ctx)
	require.NoError(t, err)

	err = testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t),
		func(txCtx context.Context, tx bun.Tx) error {
			// Pin a non-Berlin session timezone: without it the assertion would
			// pass on a machine that happens to run its database in Berlin.
			var configured string
			if err := tx.NewSelect().
				ColumnExpr(`set_config('TimeZone', ?, true)`, "UTC").
				Scan(txCtx, &configured); err != nil {
				return err
			}

			rows, total, err := repo.ListEnded(txCtx, timezone.TodayDate(),
				users.CareExitListFilter{Search: class})
			if err != nil {
				return err
			}
			require.Equal(t, 1, total, "the ended child must show up exactly once")
			require.Len(t, rows, 1)
			require.NotNil(t, rows[0].RecordedAt)
			assert.Equal(t, wantBerlinDay, rows[0].RecordedAt.String(),
				"the archive must show the Berlin day, not the session-timezone day")
			return nil
		})
	require.NoError(t, err)
}

func TestCareExitCleanupRepository_LocksPersonImpactRows(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	student := testpkg.CreateTestStudent(t, db, "CareExitLock", "Kid", "2a")
	repo := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db)).CareExitCleanup
	result := make(chan error, 1)

	err := testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(txCtx context.Context, _ bun.Tx) error {
		if err := repo.LockImpactRowsForCareExit(txCtx, []int64{student.ID}); err != nil {
			return err
		}
		go func() {
			result <- testpkg.WithTenantTx(t, context.Background(), db, testpkg.Tenant(t), func(otherCtx context.Context, tx bun.Tx) error {
				if _, err := tx.ExecContext(otherCtx, `SET LOCAL lock_timeout = '100ms'`); err != nil {
					return err
				}
				_, err := tx.NewUpdate().TableExpr("users.persons").
					Set("tag_id = ?", "care-exit-lock-probe").Where("id = ?", student.PersonID).Exec(otherCtx)
				return err
			})
		}()
		assert.Error(t, <-result, "RFID assignment must not change behind a binding preview")
		return nil
	})
	require.NoError(t, err)
}
